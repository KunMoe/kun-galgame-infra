package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

type rollupFixture struct {
	parent, imprint, spinoff     int64
	own, viaImprint, shared, r18 int64
	spinoffWork                  int64
}

func seedRollup(t *testing.T) rollupFixture {
	t.Helper()
	cleanTables(t)
	cleanLabelGraphTables(t)

	f := rollupFixture{
		parent:  mkLabel(t, "VISUAL ARTS", ""),
		imprint: mkLabel(t, "Key", ""),
		spinoff: mkLabel(t, "fairys", ""),
	}
	relateMirrored(t, f.parent, f.imprint, model.LabelRelationImprint)
	relateMirrored(t, f.parent, f.spinoff, model.LabelRelationSpawned)

	mk := func(name string, rating int16, claimID int64, labels ...int64) int64 {
		w := createWorkX(t, galgameMediumID, rating, model.WorkStatusLive, name)
		claimLive(t, w.ID, claimID)
		for _, l := range labels {
			if err := testDB.Create(&model.CatalogWorkLabel{
				WorkID: w.ID, LabelID: l, Kind: model.WorkLabelKindBrand,
			}).Error; err != nil {
				t.Fatalf("attribute %s: %v", name, err)
			}
		}
		return w.ID
	}
	f.own = mk("Kanon Memorial", model.ContentRatingAllAges, 9901, f.parent)
	f.viaImprint = mk("CLANNAD", model.ContentRatingAllAges, 9902, f.imprint)
	f.shared = mk("Rewrite", model.ContentRatingAllAges, 9903, f.parent, f.imprint)
	f.r18 = mk("Little Busters EX", model.ContentRatingR18, 9904, f.imprint)
	f.spinoffWork = mk("Canvas", model.ContentRatingAllAges, 9905, f.spinoff)
	return f
}

func TestLabelRollupCountsAreDisjoint(t *testing.T) {
	f := seedRollup(t)
	svc := newPublicSvc()
	ctx := t.Context()

	l, ok, err := svc.Label(ctx, f.parent, false, false, 50, 0)
	if err != nil || !ok {
		t.Fatalf("label: %v ok=%v", err, ok)
	}
	if l.WorkCount != 2 {
		t.Fatalf("work_count = %d, want 2 (own + shared)", l.WorkCount)
	}
	if l.ImprintWorkCount != 1 {
		t.Fatalf("imprint_work_count = %d, want 1 (CLANNAD only)", l.ImprintWorkCount)
	}

	l, _, err = svc.Label(ctx, f.parent, false, true, 50, 0)
	if err != nil {
		t.Fatalf("label nsfw: %v", err)
	}
	if l.ImprintWorkCount != 2 {
		t.Fatalf("nsfw imprint_work_count = %d, want 2", l.ImprintWorkCount)
	}

	if l.WorkCount+l.ImprintWorkCount != 4 {
		t.Fatalf("rolled-up total = %d, want 4 (the spin-off's work is not ours)", l.WorkCount+l.ImprintWorkCount)
	}
}

func TestLabelRollupPageMatchesTheCountsAndIsAttributed(t *testing.T) {
	f := seedRollup(t)
	svc := newPublicSvc()
	ctx := t.Context()

	base := WorksListFilter{LabelID: f.parent, ClaimStates: []string{model.ClaimStateKeyLive}}

	plain, err := svc.WorksList(ctx, base, "", 50)
	if err != nil {
		t.Fatalf("plain list: %v", err)
	}
	if len(plain.Items) != 2 {
		t.Fatalf("plain page = %d items, want 2", len(plain.Items))
	}
	for _, it := range plain.Items {
		if it.ViaLabel != nil {
			t.Fatalf("work %d carries via_label outside the roll-up", it.ID)
		}
	}

	rolled := base
	rolled.LabelRollup = true
	page, err := svc.WorksList(ctx, rolled, "", 50)
	if err != nil {
		t.Fatalf("rolled list: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("rolled page = %d items, want 3", len(page.Items))
	}
	via := map[int64]string{}
	for _, it := range page.Items {
		if it.ID == f.spinoffWork {
			t.Fatalf("the spin-off's work reached a rolled-up company page")
		}
		if it.ViaLabel != nil {
			via[it.ID] = it.ViaLabel.DisplayName
		}
	}
	if len(via) != 1 || via[f.viaImprint] != "Key" {
		t.Fatalf("via_label = %v, want only work %d via Key", via, f.viaImprint)
	}
}

func TestLabelRollupIgnoresAMergedImprint(t *testing.T) {
	f := seedRollup(t)
	svc := newPublicSvc()
	ctx := t.Context()

	if err := testDB.Exec(`UPDATE catalog_label SET deleted_at = now() WHERE id = ?`, f.imprint).Error; err != nil {
		t.Fatalf("soft-delete imprint: %v", err)
	}
	l, ok, err := svc.Label(ctx, f.parent, false, true, 50, 0)
	if err != nil || !ok {
		t.Fatalf("label: %v ok=%v", err, ok)
	}
	if l.ImprintWorkCount != 0 {
		t.Fatalf("imprint_work_count = %d, want 0 after the imprint was merged away", l.ImprintWorkCount)
	}
	rolled := WorksListFilter{LabelID: f.parent, LabelRollup: true, ClaimStates: []string{model.ClaimStateKeyLive}}
	page, err := svc.WorksList(ctx, rolled, "", 50)
	if err != nil {
		t.Fatalf("rolled list: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("rolled page = %d items, want 2 (the company's own works only)", len(page.Items))
	}
}
