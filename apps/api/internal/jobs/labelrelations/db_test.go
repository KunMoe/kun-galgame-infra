package labelrelations

import (
	"context"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcv "api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/labelrelations")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/labelrelations", "no test db: %v", err)
	}
	for _, step := range []func(*gorm.DB) error{migrate.Run, seed.Run, srcv.EnsureSchema} {
		if err := step(db); err != nil {
			dbtest.SkipMainf("jobs/labelrelations", "setup: %v", err)
		}
	}
	testDB = db
	os.Exit(m.Run())
}

const sourceVNDB int16 = 2

func cleanAll(t *testing.T) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_label_relation", "catalog_external_ref", "catalog_label",
		"src_vndb.producers_relations",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkLabel(t *testing.T, id int64, name string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogLabel{
		ID: id, DisplayName: name, Kind: model.LabelKindGameBrand,
	}).Error)
}

func mkAnchor(t *testing.T, ext string, label int64) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO catalog_external_ref (entity_type, entity_id, source_id, external_id, link_kind, matched_by, created_at)
		 VALUES (?, ?, ?, ?, ?, 'rule:test-label', now())`,
		model.EntityTypeLabel, label, sourceVNDB, ext, model.LinkKindExact).Error)
}

func mkUpstream(t *testing.T, id, pid, relation string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.producers_relations (id, pid, relation) VALUES (?, ?, ?)`,
		id, pid, relation).Error)
}

func readGraph(t *testing.T) []model.CatalogLabelRelation {
	t.Helper()
	var rows []model.CatalogLabelRelation
	require.NoError(t, testDB.Raw(
		`SELECT label_id, other_label_id, relation, source_id, matched_by, created_at
		 FROM catalog_label_relation ORDER BY label_id, other_label_id, relation`).Scan(&rows).Error)
	return rows
}

func TestMirroredPairBecomesTwoRows(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)
	mkLabel(t, 800, "Key")
	mkLabel(t, 801, "VisualArt's")
	mkAnchor(t, "p1", 800)
	mkAnchor(t, "p2", 801)
	mkUpstream(t, "p1", "p2", "par")
	mkUpstream(t, "p2", "p1", "sub")

	st, err := build(context.Background(), testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 2, st.EdgesTotal)
	assert.Equal(t, 2, st.BothAnchored)
	assert.Equal(t, 2, st.Written)
	assert.Zero(t, st.SkippedUnanchored)

	rows := readGraph(t)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(800), rows[0].LabelID)
	assert.Equal(t, int64(801), rows[0].OtherLabelID)
	assert.Equal(t, model.LabelRelationParent, rows[0].Relation)
	assert.Equal(t, sourceVNDB, rows[0].SourceID)
	assert.Equal(t, matchedBy, rows[0].MatchedBy)
	assert.Equal(t, int64(801), rows[1].LabelID)
	assert.Equal(t, int64(800), rows[1].OtherLabelID)
	assert.Equal(t, model.LabelRelationSubsidiary, rows[1].Relation)
}

func TestUnanchoredEndpointIsSkipped(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)
	mkLabel(t, 800, "Key")
	mkAnchor(t, "p1", 800)
	mkUpstream(t, "p1", "p2", "par")
	mkUpstream(t, "p2", "p1", "sub")

	st, err := build(context.Background(), testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 2, st.EdgesTotal)
	assert.Zero(t, st.BothAnchored)
	assert.Equal(t, 2, st.SkippedUnanchored)
	assert.Zero(t, st.Written)
	assert.Empty(t, readGraph(t))

	mkLabel(t, 801, "VisualArt's")
	require.NoError(t, testDB.Exec(
		`INSERT INTO catalog_external_ref (entity_type, entity_id, source_id, external_id, link_kind, matched_by, created_at)
		 VALUES (?, ?, ?, 'p2', ?, 'rule:test-label', now())`,
		model.EntityTypeLabel, int64(801), sourceVNDB, model.LinkKindProbable).Error)
	st, err = build(context.Background(), testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 2, st.SkippedUnanchored)
	assert.Empty(t, readGraph(t))
}

func TestRebuildIsIdempotentAndReapsStaleRows(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)
	mkLabel(t, 800, "Key")
	mkLabel(t, 801, "VisualArt's")
	mkLabel(t, 802, "Kinetic Novel")
	mkAnchor(t, "p1", 800)
	mkAnchor(t, "p2", 801)
	mkAnchor(t, "p3", 802)
	mkUpstream(t, "p1", "p2", "par")
	mkUpstream(t, "p2", "p1", "sub")
	mkUpstream(t, "p1", "p3", "imp")
	mkUpstream(t, "p3", "p1", "ipa")

	ctx := context.Background()
	st, err := build(ctx, testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 4, st.Written)
	first := readGraph(t)
	require.Len(t, first, 4)

	st2, err := build(ctx, testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 4, st2.Written)
	assert.Equal(t, int64(4), st2.Deleted, "the rebuild replaced its own previous rows")
	second := readGraph(t)
	require.Len(t, second, 4)
	for i := range first {
		assert.Equal(t, first[i].LabelID, second[i].LabelID)
		assert.Equal(t, first[i].OtherLabelID, second[i].OtherLabelID)
		assert.Equal(t, first[i].Relation, second[i].Relation)
	}

	require.NoError(t, testDB.Exec(
		`DELETE FROM src_vndb.producers_relations WHERE relation IN ('imp','ipa')`).Error)
	st3, err := build(ctx, testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 2, st3.Written)
	rows := readGraph(t)
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.NotEqual(t, model.LabelRelationImprint, r.Relation)
		assert.NotEqual(t, model.LabelRelationImprintOf, r.Relation)
	}

	require.NoError(t, testDB.Exec(`DELETE FROM src_vndb.producers_relations`).Error)
	st4, err := build(ctx, testDB, false)
	require.NoError(t, err)
	assert.Zero(t, st4.Written)
	assert.Len(t, readGraph(t), 2, "dry run left the previous graph untouched")
}

func TestSelfEdgesAndUnknownCodesAreDropped(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)
	mkLabel(t, 800, "Key")
	mkAnchor(t, "p1", 800)
	mkAnchor(t, "p2", 800)
	mkUpstream(t, "p1", "p2", "par")
	mkUpstream(t, "p1", "p2", "zzz")

	st, err := build(context.Background(), testDB, true)
	require.NoError(t, err)
	assert.Equal(t, 1, st.SkippedSelf)
	assert.Equal(t, 1, st.SkippedUnknownRelation)
	assert.Zero(t, st.Written)
	assert.Empty(t, readGraph(t))
}
