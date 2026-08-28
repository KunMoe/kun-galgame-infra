package getchurefs

import (
	"context"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/platform/catalog/srcvndb"
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
		dbtest.SkipMain("jobs/getchurefs")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/getchurefs", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/getchurefs", "catalog migrate failed: %v", err)
	}
	if err := srcvndb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/getchurefs", "src_vndb migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/getchurefs", "catalog seed failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_external_ref", "catalog_release", "catalog_work",
		"src_vndb.releases_extlinks", "src_vndb.extlinks",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" CASCADE").Error)
	}
}

func fixture(t *testing.T, name, vndbReleaseID string, links map[string]string) int64 {
	t.Helper()
	var medium int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key='galgame'`).Scan(&medium).Error)
	var vndbSource int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='vndb'`).Scan(&vndbSource).Error)

	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: name}
	require.NoError(t, testDB.Create(&w).Error)
	rel := model.CatalogRelease{WorkID: w.ID, Kind: 0}
	require.NoError(t, testDB.Create(&rel).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: vndbSource,
		ExternalID: vndbReleaseID, LinkKind: model.LinkKindExact, MatchedBy: "import:test"}).Error)

	for site, value := range links {
		var id int
		require.NoError(t, testDB.Raw(
			`INSERT INTO src_vndb.extlinks (id, site, value)
			 VALUES ((SELECT coalesce(max(id),0)+1 FROM src_vndb.extlinks), ?, ?) RETURNING id`,
			site, value).Scan(&id).Error)
		require.NoError(t, testDB.Exec(
			`INSERT INTO src_vndb.releases_extlinks (id, link) VALUES (?, ?)`, vndbReleaseID, id).Error)
	}
	return rel.ID
}

func TestImportFromVNDBExtlinks(t *testing.T) {
	clean(t)
	ctx := context.Background()
	var getchuSource int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='getchu'`).Scan(&getchuSource).Error)
	require.NotZero(t, getchuSource, "the seed must carry the getchu source row")

	relA := fixture(t, "has-getchu", "r100", map[string]string{"getchu": "1117747"})
	relB := fixture(t, "getchudl-only", "r200", map[string]string{"getchudl": "999001"})
	relC := fixture(t, "other-store-only", "r300", map[string]string{"dlsite": "RJ01"})

	st, err := Run(ctx, testDB, Opts{})
	require.NoError(t, err)
	assert.Equal(t, 1, st.Candidates, "only the getchu link is a candidate")
	assert.Equal(t, 1, st.Planned)
	assert.Zero(t, st.Written)
	var n int64
	require.NoError(t, testDB.Raw(`SELECT count(*) FROM catalog_external_ref WHERE source_id = ?`, getchuSource).Scan(&n).Error)
	assert.EqualValues(t, 0, n, "dry run writes nothing")

	st, err = Run(ctx, testDB, Opts{Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 1, st.Written)
	assert.Zero(t, st.Errors)

	var ref model.CatalogExternalRef
	require.NoError(t, testDB.Where("source_id = ?", getchuSource).First(&ref).Error)
	assert.Equal(t, model.EntityTypeRelease, ref.EntityType, "anchors are release-level, like dlsite worknos")
	assert.Equal(t, relA, ref.EntityID)
	assert.Equal(t, "1117747", ref.ExternalID)
	assert.Equal(t, model.LinkKindExact, ref.LinkKind,
		"the ref is exactly as strong as the vndb ref it rides on")
	assert.Equal(t, matchedBy, ref.MatchedBy)

	for _, rel := range []int64{relB, relC} {
		require.NoError(t, testDB.Raw(
			`SELECT count(*) FROM catalog_external_ref WHERE source_id = ? AND entity_id = ?`,
			getchuSource, rel).Scan(&n).Error)
		assert.EqualValues(t, 0, n, "release %d must not be anchored", rel)
	}

	st, err = Run(ctx, testDB, Opts{Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.Candidates, "an anchored release leaves the candidate set")
	assert.Zero(t, st.Written)
}

func TestNeverRegradesAnExistingAnchor(t *testing.T) {
	clean(t)
	ctx := context.Background()
	var getchuSource int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='getchu'`).Scan(&getchuSource).Error)

	rel := fixture(t, "already-anchored", "r400", map[string]string{"getchu": "222222"})
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: rel, SourceID: getchuSource,
		ExternalID: "111111", LinkKind: model.LinkKindProbable, MatchedBy: "human:review"}).Error)

	st, err := Run(ctx, testDB, Opts{Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.Candidates, "an already-anchored release is not a candidate")

	var ref model.CatalogExternalRef
	require.NoError(t, testDB.Where("source_id = ? AND entity_id = ?", getchuSource, rel).First(&ref).Error)
	assert.Equal(t, "111111", ref.ExternalID, "the existing id survives")
	assert.Equal(t, model.LinkKindProbable, ref.LinkKind, "the existing tier survives")
	assert.Equal(t, "human:review", ref.MatchedBy)
}

func TestProbableVndbAnchorIsNotAChain(t *testing.T) {
	clean(t)
	ctx := context.Background()
	var medium, vndbSource int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key='galgame'`).Scan(&medium).Error)
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='vndb'`).Scan(&vndbSource).Error)

	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: "probable-vndb"}
	require.NoError(t, testDB.Create(&w).Error)
	rel := model.CatalogRelease{WorkID: w.ID, Kind: 0}
	require.NoError(t, testDB.Create(&rel).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: vndbSource,
		ExternalID: "r500", LinkKind: model.LinkKindProbable, MatchedBy: "rule:guess"}).Error)
	var id int
	require.NoError(t, testDB.Raw(
		`INSERT INTO src_vndb.extlinks (id, site, value) VALUES (1, 'getchu', '333333') RETURNING id`).Scan(&id).Error)
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.releases_extlinks (id, link) VALUES ('r500', ?)`, id).Error)

	st, err := Run(ctx, testDB, Opts{Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.Candidates, "a probable vndb anchor cannot mint an exact getchu one")
	assert.Zero(t, st.Written)
}
