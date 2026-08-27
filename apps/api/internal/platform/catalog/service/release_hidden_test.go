package service

import (
	"testing"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
)

func hideRelease(t *testing.T, id int64) {
	t.Helper()
	if err := testDB.Exec(`UPDATE catalog_release SET deleted_at = now() WHERE id = ?`, id).Error; err != nil {
		t.Fatalf("hide release %d: %v", id, err)
	}
}

func TestHiddenReleaseLeavesPublicSurfaces(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()
	svc := newPublicSvc()
	read := NewReadService(testDB)

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "隠蔽公開")
	live := createRelease(t, w.ID, 2024, 6, 14)
	hidden := createRelease(t, w.ID, 1999, 1, 1)
	hideRelease(t, hidden.ID)

	detail, err := read.WorkByID(ctx, w.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Releases) != 1 || detail.Releases[0].Release.ID != live.ID {
		t.Fatalf("default WorkByID releases = %+v, want only live %d", detail.Releases, live.ID)
	}

	withHidden, err := read.WorkByIDIncludeHidden(ctx, w.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(withHidden.Releases) != 2 {
		t.Fatalf("include-hidden releases = %d, want 2", len(withHidden.Releases))
	}
	var sawHidden bool
	for _, rd := range withHidden.Releases {
		if rd.Release.ID == hidden.ID {
			sawHidden = rd.Release.DeletedAt.Valid
		}
	}
	if !sawHidden {
		t.Fatal("include-hidden must return the hidden row with DeletedAt set")
	}

	pub, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, true, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("public WorkDetail: found=%v err=%v", found, err)
	}
	if len(pub.Releases) != 1 || pub.Releases[0].ID != live.ID {
		t.Fatalf("public work-detail releases[] = %+v", pub.Releases)
	}
	if pub.ReleaseDate == nil || *pub.ReleaseDate != "2024-06-14" {
		t.Fatalf("public ReleaseDate = %v, hidden 1999 must not win the fold", pub.ReleaseDate)
	}

	feed, err := svc.ReleaseFeed(ctx, ReleaseFeedFilter{NSFW: true}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range feed.Items {
		if item.ID == hidden.ID {
			t.Fatal("hidden release leaked into the public release feed items")
		}
	}
	count, _, _, err := svc.ReleaseFeedMeta(ctx, ReleaseFeedFilter{NSFW: true})
	if err != nil {
		t.Fatal(err)
	}
	if int(count) != len(feed.Items) {
		t.Fatalf("release feed meta count %d != items %d (same predicate)", count, len(feed.Items))
	}
	if count != 1 {
		t.Fatalf("release feed count = %d, want 1 live dated release", count)
	}

	june := calIDs(t, svc, june2024(), CalendarFilter{NSFW: true})
	if len(june) != 1 || june[0] != w.ID {
		t.Fatalf("calendar june = %v, want work %d from the live June date (not the hidden 1999)", june, w.ID)
	}

	list, err := svc.WorksList(ctx, WorksListFilter{NSFW: true}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	var listed *string
	for i := range list.Items {
		if list.Items[i].ID == w.ID {
			listed = list.Items[i].ReleaseDate
		}
	}
	if listed == nil || *listed != "2024-06-14" {
		t.Fatalf("works-list earliest date = %v, want 2024-06-14", listed)
	}

	var ord *int64
	if err := testDB.Raw(`
		SELECT min(released_y::int * 10000 + coalesce(released_m,0)::int * 100 + coalesce(released_d,0)::int)
		FROM catalog_release
		WHERE work_id = ? AND released_y IS NOT NULL AND deleted_at IS NULL`, w.ID).Scan(&ord).Error; err != nil {
		t.Fatal(err)
	}
	if ord == nil || *ord != 20240614 {
		t.Fatalf("loadEarliestReleaseOrd predicate = %v, want 20240614 (hidden 19990101 excluded)", ord)
	}
}

func TestHiddenReleaseStillResolvesByAnchor(t *testing.T) {
	cleanTables(t)
	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "錨隠蔽")
	rel := createRelease(t, w.ID, 2020, 1, 1)
	addExternalRef(t, model.EntityTypeRelease, rel.ID, srcDlsite, "RJHIDDEN", model.LinkKindExact)
	hideRelease(t, rel.ID)

	id, ok, err := repository.FindWorkIDByAnchor(testDB, srcDlsite, "RJHIDDEN")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || id != w.ID {
		t.Fatalf("FindWorkIDByAnchor after hide = (%d, %v), want work %d", id, ok, w.ID)
	}
}
