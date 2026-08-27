package service

import (
	"context"
	"encoding/json"
	"testing"

	"api/internal/platform/catalog/model"
)

func addReleaseLabel(t *testing.T, releaseID, labelID int64, kind int16) {
	t.Helper()
	if err := testDB.Create(&model.CatalogReleaseLabel{
		ReleaseID: releaseID, LabelID: labelID, Kind: kind,
	}).Error; err != nil {
		t.Fatalf("create release label edge: %v", err)
	}
}

func createLabel(t *testing.T, displayName string, kind int16) int64 {
	t.Helper()
	l := &model.CatalogLabel{DisplayName: displayName, Kind: kind}
	if err := testDB.Create(l).Error; err != nil {
		t.Fatalf("create label %s: %v", displayName, err)
	}
	return l.ID
}

func TestReleaseLabelsAreScopedToTheirEdition(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "あまいろショコラータ")
	original := createRelease(t, w.ID, 2019, 4, 26)
	port := createRelease(t, w.ID, 2021, 3, 25)
	undated := createRelease(t, w.ID, 2020, 1, 1)

	cabbage := createLabel(t, "きゃべつそふと", model.LabelKindGameBrand)
	hunex := createLabel(t, "HuneX", model.LabelKindGameBrand)

	addReleaseLabel(t, original.ID, cabbage, model.WorkLabelKindDeveloper)
	addReleaseLabel(t, original.ID, cabbage, model.WorkLabelKindPublisher)
	addReleaseLabel(t, port.ID, cabbage, model.WorkLabelKindDeveloper)
	addReleaseLabel(t, port.ID, hunex, model.WorkLabelKindPublisher)

	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail = %v, %v", found, err)
	}

	byID := map[int64]int{}
	for i, r := range rec.Releases {
		byID[r.ID] = i
	}
	if len(byID) != 3 {
		t.Fatalf("releases[] = %d rows, want 3", len(rec.Releases))
	}

	got := rec.Releases[byID[original.ID]].Labels
	if len(got) != 1 {
		t.Fatalf("original edition labels[] = %+v, want one company", got)
	}
	if got[0].ID != cabbage || got[0].DisplayName != "きゃべつそふと" {
		t.Fatalf("original edition company = %+v, want きゃべつそふと", got[0])
	}
	if len(got[0].Kinds) != 2 || got[0].Kinds[0] != "developer" || got[0].Kinds[1] != "publisher" {
		t.Fatalf("kinds = %v, want [developer publisher] — the collapse is not shared with the work grain", got[0].Kinds)
	}
	if got[0].Kind != "developer" {
		t.Fatalf("primary kind = %q, want developer (who made it identifies it better)", got[0].Kind)
	}

	got = rec.Releases[byID[port.ID]].Labels
	if len(got) != 2 {
		t.Fatalf("port labels[] = %+v, want two companies", got)
	}
	var sawHuneX bool
	for _, l := range got {
		if l.ID == hunex {
			sawHuneX = true
			if l.Kind != "publisher" {
				t.Fatalf("HuneX kind = %q, want publisher", l.Kind)
			}
		}
	}
	if !sawHuneX {
		t.Fatal("the port's publisher is missing from its own edition")
	}
	for _, l := range rec.Releases[byID[original.ID]].Labels {
		if l.ID == hunex {
			t.Fatal("the port's publisher leaked onto the original edition — the grain collapsed")
		}
	}

	if labels := rec.Releases[byID[undated.ID]].Labels; labels == nil || len(labels) != 0 {
		t.Fatalf("unattributed edition labels[] = %+v, want an empty slice", labels)
	}
	blob, err := json.Marshal(rec.Releases[byID[undated.ID]])
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(blob, &wire); err != nil {
		t.Fatalf("unmarshal release: %v", err)
	}
	if string(wire["labels"]) != "[]" {
		t.Fatalf("labels on the wire = %s, want [] (a missing key reads as a parse failure)", wire["labels"])
	}
}

func TestReleaseLabelsDropSoftDeletedLabels(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Merged Twin")
	rel := createRelease(t, w.ID, 2022, 8, 1)
	survivor := createLabel(t, "Survivor", model.LabelKindGameBrand)
	merged := createLabel(t, "Survivor", model.LabelKindGameBrand)
	addReleaseLabel(t, rel.ID, survivor, model.WorkLabelKindPublisher)
	addReleaseLabel(t, rel.ID, merged, model.WorkLabelKindPublisher)

	if err := testDB.Delete(&model.CatalogLabel{}, merged).Error; err != nil {
		t.Fatalf("soft-delete label: %v", err)
	}

	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail = %v, %v", found, err)
	}
	got := rec.Releases[0].Labels
	if len(got) != 1 || got[0].ID != survivor {
		t.Fatalf("labels[] = %+v, want only the surviving label %d", got, survivor)
	}
}

func TestReleaseLabelWorkCountMatchesTheChipTarget(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Counted")
	rel := createRelease(t, w.ID, 2023, 5, 5)
	other := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Counted Sibling")
	claimLive(t, w.ID, 9401)
	claimLive(t, other.ID, 9402)
	brand := addWorkLabel(t, w.ID, "Counted Brand", model.LabelKindGameBrand, model.WorkLabelKindBrand)
	if err := testDB.Create(&model.CatalogWorkLabel{
		WorkID: other.ID, LabelID: brand, Kind: model.WorkLabelKindBrand,
	}).Error; err != nil {
		t.Fatalf("attach label to sibling: %v", err)
	}
	addReleaseLabel(t, rel.ID, brand, model.WorkLabelKindPublisher)

	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail = %v, %v", found, err)
	}
	got := rec.Releases[0].Labels
	if len(got) != 1 {
		t.Fatalf("release labels[] = %+v, want one chip", got)
	}
	if got[0].WorkCount != 2 {
		t.Fatalf("release chip work_count = %d, want 2", got[0].WorkCount)
	}
	assertCountMatchesWorksList(t, svc, WorksListFilter{Sort: "id", LabelID: brand}, got[0].WorkCount)
	if len(rec.Labels) != 1 || rec.Labels[0].WorkCount != got[0].WorkCount {
		t.Fatalf("work chip %+v disagrees with the release chip %+v", rec.Labels, got[0])
	}
}

func TestReleaseFeedCarriesEditionLabels(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Feed Work")
	claimLive(t, w.ID, 9403)
	rel := createRelease(t, w.ID, 2024, 2, 2)
	pub := createLabel(t, "Feed Publisher", model.LabelKindGameBrand)
	addReleaseLabel(t, rel.ID, pub, model.WorkLabelKindPublisher)
	if err := testDB.Create(&model.CatalogWorkLabel{
		WorkID: w.ID, LabelID: pub, Kind: model.WorkLabelKindPublisher,
	}).Error; err != nil {
		t.Fatalf("attach feed publisher to the work: %v", err)
	}
	bare := createRelease(t, w.ID, 2024, 3, 3)

	page, err := svc.ReleaseFeed(ctx, ReleaseFeedFilter{}, "", 20)
	if err != nil {
		t.Fatalf("ReleaseFeed: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("feed = %d items, want 2", len(page.Items))
	}
	for _, it := range page.Items {
		switch it.ID {
		case rel.ID:
			if len(it.Labels) != 1 || it.Labels[0].ID != pub {
				t.Fatalf("feed item labels[] = %+v, want the edition's publisher", it.Labels)
			}
			if it.Labels[0].WorkCount != 1 {
				t.Fatalf("feed chip work_count = %d, want 1", it.Labels[0].WorkCount)
			}
		case bare.ID:
			if it.Labels == nil || len(it.Labels) != 0 {
				t.Fatalf("unattributed feed item labels[] = %+v, want an empty slice", it.Labels)
			}
		}
	}
}
