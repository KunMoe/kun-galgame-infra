package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type sqlRecorder struct{ stmts *[]string }

func (r sqlRecorder) LogMode(gormlogger.LogLevel) gormlogger.Interface { return r }
func (r sqlRecorder) Info(context.Context, string, ...any)             {}
func (r sqlRecorder) Warn(context.Context, string, ...any)             {}
func (r sqlRecorder) Error(context.Context, string, ...any)            {}
func (r sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	*r.stmts = append(*r.stmts, sql)
}

// relationQueryMarker is unique to loadWorkRelations: no other statement on the
// work-detail path projects the relation type's phrase columns.
const relationQueryMarker = "forward_phrase"

func recordingPublicSvc(stmts *[]string) *PublicService {
	db := testDB.Session(&gorm.Session{Logger: sqlRecorder{stmts: stmts}, NewDB: true})
	return NewPublicService(db, NewReadService(db), testResolve, "")
}

func relatesTo(rels []WorkRelationRow, want ...int64) bool {
	got := make(map[int64]bool, len(rels))
	for _, r := range rels {
		got[r.OtherID] = true
	}
	if len(got) != len(want) {
		return false
	}
	for _, id := range want {
		if !got[id] {
			return false
		}
	}
	return true
}

func countMatching(stmts []string, marker string) int {
	n := 0
	for _, s := range stmts {
		if strings.Contains(s, marker) {
			n++
		}
	}
	return n
}

func TestWorkDetailIncludeGatesTheRelationQuery(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	w := createWork(t, "本編")
	sib := createWork(t, "続編")
	other := createWork(t, "関連作")
	createSameSeriesEdge(t, w.ID, sib.ID)
	createWorkRelation(t, w.ID, other.ID)

	var stmts []string
	svc := recordingPublicSvc(&stmts)

	stmts = stmts[:0]
	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("detail without include: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, relationQueryMarker); n != 0 {
		t.Fatalf("no ?include=relations still ran the relation query %d time(s)", n)
	}
	if len(rec.Relations) != 0 {
		t.Fatalf("relations = %v (want empty without the include token)", rec.Relations)
	}
	// The block that must survive the gating: series_siblings is unconditional.
	if len(rec.SeriesSiblings) != 1 || rec.SeriesSiblings[0].ID != sib.ID {
		t.Fatalf("series_siblings = %v (want the one sibling %d)", rec.SeriesSiblings, sib.ID)
	}

	stmts = stmts[:0]
	rec, found, err = svc.WorkDetail(ctx, w.ID, PublicInclude{Relations: true}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("detail with include: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, relationQueryMarker); n != 1 {
		t.Fatalf("?include=relations ran the relation query %d time(s) (want 1)", n)
	}
	// Two edges: the adaptation edge plus the same_series edge, which the
	// relations block carries in its own right on top of series_siblings.
	relIDs := map[int64]bool{}
	for _, r := range rec.Relations {
		relIDs[r.Work.ID] = true
	}
	if len(relIDs) != 2 || !relIDs[other.ID] || !relIDs[sib.ID] {
		t.Fatalf("relations = %v (want works %d and %d)", rec.Relations, other.ID, sib.ID)
	}
	if len(rec.SeriesSiblings) != 1 || rec.SeriesSiblings[0].ID != sib.ID {
		t.Fatalf("series_siblings with include = %v (want the one sibling %d)", rec.SeriesSiblings, sib.ID)
	}
}

// The S2S / user faces have no include tokens; buildWorkResponse renders both
// blocks unconditionally, so their loader must keep fetching both.
func TestWorkByIDKeepsRelationsForTheNonPublicFaces(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	w := createWork(t, "本編S2S")
	sib := createWork(t, "続編S2S")
	other := createWork(t, "関連作S2S")
	createSameSeriesEdge(t, w.ID, sib.ID)
	createWorkRelation(t, w.ID, other.ID)

	read := NewReadService(testDB)
	for _, tc := range []struct {
		name string
		load func() (*WorkDetail, error)
	}{
		{"WorkByID", func() (*WorkDetail, error) { return read.WorkByID(ctx, w.ID, 0) }},
		{"WorkByIDIncludeHidden", func() (*WorkDetail, error) { return read.WorkByIDIncludeHidden(ctx, w.ID, 0) }},
		{"WorkByIDPublic(withRelations)", func() (*WorkDetail, error) {
			return read.WorkByIDPublic(ctx, w.ID, 0, true, PublicFields{})
		}},
	} {
		detail, err := tc.load()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !relatesTo(detail.Relations, other.ID, sib.ID) {
			t.Fatalf("%s relations = %v (want works %d and %d)", tc.name, detail.Relations, other.ID, sib.ID)
		}
		if len(detail.SeriesSiblings) != 1 || detail.SeriesSiblings[0].WorkID != sib.ID {
			t.Fatalf("%s series_siblings = %v (want the one sibling %d)", tc.name, detail.SeriesSiblings, sib.ID)
		}
	}
}

func TestWorkByAnchorKeepsRelations(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	w := createWork(t, "アンカー本編")
	sib := createWork(t, "アンカー続編")
	other := createWork(t, "アンカー関連作")
	createSameSeriesEdge(t, w.ID, sib.ID)
	createWorkRelation(t, w.ID, other.ID)
	addExternalRef(t, model.EntityTypeWork, w.ID, srcVNDB, "v999001", model.LinkKindExact)

	detail, err := NewReadService(testDB).WorkByAnchor(ctx, "vndb", "v999001")
	if err != nil {
		t.Fatalf("WorkByAnchor: %v", err)
	}
	if !relatesTo(detail.Relations, other.ID, sib.ID) {
		t.Fatalf("relations = %v (want works %d and %d)", detail.Relations, other.ID, sib.ID)
	}
	if len(detail.SeriesSiblings) != 1 || detail.SeriesSiblings[0].WorkID != sib.ID {
		t.Fatalf("series_siblings = %v (want the one sibling %d)", detail.SeriesSiblings, sib.ID)
	}
}
