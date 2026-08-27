package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

// The flat titles[] on the work detail face keeps every row rather than
// electing one per locale, so `machine` has to ride each row.
func TestPublicWorkDetailTitlesCarryMachine(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "ひかり")
	addWorkTitle(t, w.ID, "ja", "ひかり", model.WorkTitleKindOfficial)
	addWorkTitleP(t, w.ID, "zh-Hans", "光", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)

	rec, found, err := svc.WorkDetail(t.Context(), w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail: found=%v err=%v", found, err)
	}
	if len(rec.Titles) != 2 {
		t.Fatalf("titles = %+v, want both rows", rec.Titles)
	}
	// provenance leads the ORDER BY, so the source row comes first.
	if rec.Titles[0].Title != "ひかり" || rec.Titles[0].Machine {
		t.Fatalf("titles[0] = %+v, want the source row first and unflagged", rec.Titles[0])
	}
	if rec.Titles[1].Title != "光" || !rec.Titles[1].Machine {
		t.Fatalf("titles[1] = %+v, want the machine row flagged", rec.Titles[1])
	}
}
