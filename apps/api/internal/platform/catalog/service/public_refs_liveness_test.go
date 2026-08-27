package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

func markRefDead(t *testing.T, entityType int16, entityID int64, sourceID int16, externalID string) {
	t.Helper()
	res := testDB.Exec(`UPDATE catalog_external_ref SET dead_at = now()
		WHERE entity_type = ? AND entity_id = ? AND source_id = ? AND external_id = ?`,
		entityType, entityID, sourceID, externalID)
	if res.Error != nil {
		t.Fatalf("mark ref dead: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("mark ref dead: affected %d rows, want 1", res.RowsAffected)
	}
}

func TestPublicRefsDropDeadAnchors(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()
	ctx := t.Context()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "half-dead anchors")
	addExternalRef(t, model.EntityTypeWork, w.ID, srcVNDB, "v42", model.LinkKindExact)
	addExternalRef(t, model.EntityTypeWork, w.ID, srcDlsite, "RJ42", model.LinkKindExact)
	markRefDead(t, model.EntityTypeWork, w.ID, srcVNDB, "v42")

	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("detail: found=%v err=%v", found, err)
	}
	if len(rec.Refs) != 1 || rec.Refs[0].Source != "dlsite" || rec.Refs[0].ExternalID != "RJ42" {
		t.Fatalf("detail refs must drop the dead vndb anchor: %+v", rec.Refs)
	}

	listRefs, err := svc.workListRefs(ctx, []int64{w.ID})
	if err != nil {
		t.Fatalf("workListRefs: %v", err)
	}
	if got := listRefs[w.ID]; len(got) != 1 || got[0].ExternalID != "RJ42" {
		t.Fatalf("list refs must drop the dead vndb anchor: %+v", got)
	}

	l := &model.CatalogLabel{DisplayName: "liveness label", Kind: model.LabelKindGameBrand}
	if err := testDB.Create(l).Error; err != nil {
		t.Fatalf("create label: %v", err)
	}
	addExternalRef(t, model.EntityTypeLabel, l.ID, srcVNDB, "p9", model.LinkKindExact)
	addExternalRef(t, model.EntityTypeLabel, l.ID, srcDlsite, "RG9", model.LinkKindExact)
	markRefDead(t, model.EntityTypeLabel, l.ID, srcVNDB, "p9")

	byEntity, err := svc.entityRefsFor(ctx, model.EntityTypeLabel, []int64{l.ID})
	if err != nil {
		t.Fatalf("entityRefsFor: %v", err)
	}
	if got := byEntity[l.ID]; len(got) != 1 || got[0].ExternalID != "RG9" {
		t.Fatalf("entity refs must drop the dead vndb anchor: %+v", got)
	}
}

func TestDeadRefStillBlocksTheExactSlot(t *testing.T) {
	cleanTables(t)

	a := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "holder")
	b := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "squatter")
	addExternalRef(t, model.EntityTypeWork, a.ID, srcVNDB, "v67491", model.LinkKindExact)
	markRefDead(t, model.EntityTypeWork, a.ID, srcVNDB, "v67491")

	err := testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: b.ID, SourceID: srcVNDB,
		ExternalID: "v67491", LinkKind: model.LinkKindExact, MatchedBy: "import:test",
	}).Error
	if err == nil {
		t.Fatal("a dead anchor must still occupy its exact slot, but a second work claimed it")
	}
}
