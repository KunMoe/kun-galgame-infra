package workproducers

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

var (
	testDB  *gorm.DB
	testDSN string
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/workproducers")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/workproducers", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workproducers", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workproducers", "catalog seed failed: %v", err)
	}
	if err := srcv.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/workproducers", "src_vndb schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_label", "catalog_label", "catalog_external_ref", "catalog_release", "catalog_work",
		"src_vndb.releases_producers", "src_vndb.releases_titles",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkWork(t *testing.T, olang string) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: 1, OLang: olang, DisplayName: "作品-" + olang}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkRelease(t *testing.T, workID int64, rid string) int64 {
	t.Helper()
	rel := model.CatalogRelease{WorkID: workID, Kind: model.ReleaseKindDigital}
	require.NoError(t, testDB.Create(&rel).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: 2,
		ExternalID: rid, LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
	}).Error)
	return rel.ID
}

func mkTitle(t *testing.T, rid, lang string, mtl bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO src_vndb.releases_titles (id, lang, mtl, title, latin)
		VALUES (?, ?, ?, 'タイトル', '')`, rid, lang, mtl).Error)
}

func mkRP(t *testing.T, rid, pid string, dev, pub bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO src_vndb.releases_producers (id, pid, developer, publisher)
		VALUES (?, ?, ?, ?)`, rid, pid, dev, pub).Error)
}

func mkLabel(t *testing.T, name, pid string) int64 {
	t.Helper()
	l := model.CatalogLabel{DisplayName: name, Kind: model.LabelKindGameBrand}
	require.NoError(t, testDB.Create(&l).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeLabel, EntityID: l.ID, SourceID: 2,
		ExternalID: pid, LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
	}).Error)
	return l.ID
}

func TestImportWorkProducers(t *testing.T) {
	clean(t)

	wj := mkWork(t, "ja")
	mkRelease(t, wj, "r1")
	mkTitle(t, "r1", "ja", false)
	mkRelease(t, wj, "r2")
	mkTitle(t, "r2", "en", false)
	mkRelease(t, wj, "r3")
	mkTitle(t, "r3", "ja", true)

	mkRP(t, "r1", "p1", true, true)
	mkRP(t, "r1", "p2", false, true)
	mkRP(t, "r1", "p9", true, false)
	mkRP(t, "r2", "p3", false, true)
	mkRP(t, "r3", "p4", false, true)

	l1 := mkLabel(t, "ブランド1", "p1")
	l2 := mkLabel(t, "ブランド2", "p2")
	src2 := int16(2)
	require.NoError(t, testDB.Create(&model.CatalogWorkLabel{
		WorkID: wj, LabelID: l2, Kind: model.WorkLabelKindPublisher, SourceID: &src2,
	}).Error)

	ctx := context.Background()
	opts := Opts{DSN: testDSN}

	st, err := Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, st.DevPlanned, "p1 dev (p9 unresolved, r2/r3 gated)")
	assert.Equal(t, 2, st.PubPlanned, "p1 + p2 pub")
	assert.Equal(t, 1, st.Unresolved, "p9 has no exact label anchor")
	assert.Zero(t, st.Written)
	var n int64
	require.NoError(t, testDB.Table("catalog_work_label").Count(&n).Error)
	assert.Equal(t, int64(1), n, "dry run must not write (only the pre-existing edge)")

	opts.Apply = true
	st, err = Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 2, st.Written, "L1 dev + L1 pub (L2 pub pre-existing)")
	assert.Equal(t, 1, st.SkippedDup)
	assert.Zero(t, st.Errors)

	var edges []model.CatalogWorkLabel
	require.NoError(t, testDB.Where("work_id = ? AND label_id = ?", wj, l1).Order("kind").Find(&edges).Error)
	require.Len(t, edges, 2)
	assert.Equal(t, model.WorkLabelKindPublisher, edges[0].Kind)
	assert.Equal(t, model.WorkLabelKindDeveloper, edges[1].Kind)
	require.NotNil(t, edges[0].SourceID)
	assert.Equal(t, int16(2), *edges[0].SourceID)

	st, err = Run(ctx, opts)
	require.NoError(t, err)
	assert.Zero(t, st.Written, "idempotent re-run")
	assert.Equal(t, 3, st.SkippedDup)
}
