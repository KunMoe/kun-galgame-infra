package entitylinks

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcb "api/internal/platform/catalog/srcbangumi"
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
		fmt.Fprintln(os.Stderr, "SKIP: no test db")
		os.Exit(m.Run())
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/entitylinks", "no test db: %v", err)
	}
	for _, step := range []func(*gorm.DB) error{migrate.Run, seed.Run, srcb.EnsureSchema, srcv.EnsureSchema} {
		if err := step(db); err != nil {
			dbtest.SkipMainf("jobs/entitylinks", "setup: %v", err)
		}
	}
	testDB = db
	os.Exit(m.Run())
}

func cleanAll(t *testing.T) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_external_ref", "catalog_match_rejection", "catalog_release",
		"catalog_work", "catalog_label", "catalog_person",
		"src_vndb.extlinks", "src_vndb.releases", "src_vndb.releases_extlinks", "src_vndb.vn_extlinks",
		"src_vndb.producers_extlinks", "src_vndb.staff_extlinks",
		"src_bangumi.subject",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkWork(t *testing.T, id int64) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogWork{
		ID: id, MediumID: 1, OLang: "ja", DisplayName: fmt.Sprintf("W%d", id),
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
	}).Error)
}

func mkRelease(t *testing.T, id, workID int64) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogRelease{
		ID: id, WorkID: workID, Kind: model.ReleaseKindDefault,
	}).Error)
}

func mkLabel(t *testing.T, id int64, name string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogLabel{
		ID: id, DisplayName: name, Kind: model.LabelKindGameBrand,
	}).Error)
}

func mkPerson(t *testing.T, id int64, name string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogPerson{ID: id, DisplayName: name}).Error)
}

func mkRef(t *testing.T, entityType int16, entityID int64, source int16, ext string, kind int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: entityType, EntityID: entityID, SourceID: source,
		ExternalID: ext, LinkKind: kind, MatchedBy: "rule:test",
	}).Error)
}

func mkVndbRelease(t *testing.T, id string, official bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.releases
		   (id, gtin, olang, released, voiced, reso_x, reso_y, ani_story, ani_ero,
		    has_ero, patch, freeware, official, catalog, notes, engine)
		 VALUES (?, 0, 'ja', 20200101, 0, 0, 0, 0, 0,
		    false, false, false, ?, '', '', '')`, id, official).Error)
}

func mkExtlink(t *testing.T, id int, site, value, junction, ownerID string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.extlinks (id, site, value) VALUES (?, ?, ?)`, id, site, value).Error)
	require.NoError(t, testDB.Exec(
		fmt.Sprintf(`INSERT INTO %s (id, link) VALUES (?, ?)`, junction), ownerID, id).Error)
}

func mkSubject(t *testing.T, id int64, infobox string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id), InfoboxRaw: "",
		InfoboxParsed: []byte(infobox), ParseError: "", Summary: "", Date: "",
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func sourceID(t *testing.T, key string) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error)
	require.NotZero(t, id, key)
	return id
}

func refExists(t *testing.T, entityType int16, entityID int64, source int16, ext string) bool {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw(
		`SELECT count(*) FROM catalog_external_ref
		 WHERE entity_type=? AND entity_id=? AND source_id=? AND external_id=? AND link_kind=2`,
		entityType, entityID, source, ext).Scan(&n).Error)
	return n == 1
}

func countRelated(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw(`SELECT count(*) FROM catalog_external_ref WHERE link_kind=2`).Scan(&n).Error)
	return n
}

func TestWorkLane(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)

	mkWork(t, 901)
	mkRelease(t, 9011, 901)
	mkRelease(t, 9012, 901)
	vndb, bangumi := sourceID(t, "vndb"), sourceID(t, "bangumi")
	site, web := sourceID(t, "official_site"), sourceID(t, "web")
	mkVndbRelease(t, "r1", true)
	mkVndbRelease(t, "r2", false)
	mkRef(t, model.EntityTypeRelease, 9011, vndb, "r1", model.LinkKindExact)
	mkRef(t, model.EntityTypeRelease, 9012, vndb, "r2", model.LinkKindExact)
	mkRef(t, model.EntityTypeWork, 901, vndb, "v901", model.LinkKindExact)
	mkRef(t, model.EntityTypeWork, 901, bangumi, "9001", model.LinkKindExact)

	mkExtlink(t, 1, "website", "http://alpha.example.com/", "src_vndb.releases_extlinks", "r1")
	mkExtlink(t, 2, "website", "https://alpha.example.com", "src_vndb.releases_extlinks", "r1")
	mkExtlink(t, 3, "website", "https://www.dlsite.com/maniax/work/=/product_id/RJ01.html",
		"src_vndb.releases_extlinks", "r1")
	mkExtlink(t, 4, "dlsite", "RJ012345", "src_vndb.releases_extlinks", "r1")
	mkExtlink(t, 5, "wikidata", "4242", "src_vndb.vn_extlinks", "v901")
	mkExtlink(t, 6, "website", "https://fanpatch.example.org/sg", "src_vndb.releases_extlinks", "r2")
	mkSubject(t, 9001, `{"Fields":[{"Key":"官网","Value":"http://windmill.suki.jp/"}]}`)

	ctx := context.Background()

	dry, err := run(ctx, testDB, Opts{Only: LaneWork})
	require.NoError(t, err)
	assert.Equal(t, 3, dry.Work.Planned, "site + wikidata + bangumi site")
	assert.Equal(t, 1, dry.Work.SkippedDedup, "the http/https twin collapses")
	assert.Equal(t, 1, dry.Work.SkippedStore, "the dlsite URL is not a work official site")
	assert.Equal(t, 0, dry.Work.Written)
	assert.Zero(t, countRelated(t))

	capped, err := run(ctx, testDB, Opts{Only: LaneWork, Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, capped.Work.Planned)

	st, err := run(ctx, testDB, Opts{Only: LaneWork, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 3, st.Work.Written)
	assert.True(t, refExists(t, model.EntityTypeWork, 901, site, "alpha.example.com"))
	assert.True(t, refExists(t, model.EntityTypeWork, 901, web, "https://www.wikidata.org/wiki/Q4242"))
	assert.True(t, refExists(t, model.EntityTypeWork, 901, site, "windmill.suki.jp"))
	assert.False(t, refExists(t, model.EntityTypeWork, 901, site, "fanpatch.example.org/sg"),
		"the unofficial release's website must not reach the work")

	var rule string
	require.NoError(t, testDB.Raw(
		`SELECT matched_by FROM catalog_external_ref WHERE entity_type=5 AND entity_id=901
		 AND source_id=? AND external_id='windmill.suki.jp'`, site).Scan(&rule).Error)
	assert.Equal(t, ruleBGMWorkSite, rule)

	before := countRelated(t)
	again, err := run(ctx, testDB, Opts{Only: LaneWork, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 0, again.Work.Written)
	assert.Equal(t, before, countRelated(t))
}

func TestLabelAndPersonLanes(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)

	mkLabel(t, 800, "alicesoft")
	mkPerson(t, 700, "someone")
	vndb := sourceID(t, "vndb")
	mkRef(t, model.EntityTypeLabel, 800, vndb, "p1", model.LinkKindExact)
	mkRef(t, model.EntityTypePerson, 700, vndb, "s1", model.LinkKindExact)

	mkExtlink(t, 10, "twitter", "Alice_Soft", "src_vndb.producers_extlinks", "p1")
	mkExtlink(t, 11, "pixiv", "12345", "src_vndb.staff_extlinks", "s1")
	mkExtlink(t, 12, "bgmtv", "3300", "src_vndb.staff_extlinks", "s1")
	mkExtlink(t, 13, "mobygames_comp", "acme", "src_vndb.staff_extlinks", "s1")

	ctx := context.Background()
	st, err := run(ctx, testDB, Opts{Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 1, st.Label.Written)
	assert.Equal(t, 1, st.Person.Written)
	assert.True(t, refExists(t, model.EntityTypeLabel, 800, sourceID(t, "twitter"), "alice_soft"))
	assert.True(t, refExists(t, model.EntityTypePerson, 700, sourceID(t, "pixiv"), "12345"))
	assert.Equal(t, int64(2), countRelated(t), "bgmtv and mobygames_comp contribute nothing")
}

func TestSkipIdentityAndRejection(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	cleanAll(t)

	mkLabel(t, 810, "brandx")
	vndb := sourceID(t, "vndb")
	twitter, site := sourceID(t, "twitter"), sourceID(t, "official_site")
	mkRef(t, model.EntityTypeLabel, 810, vndb, "p2", model.LinkKindExact)
	mkRef(t, model.EntityTypeLabel, 810, twitter, "someone_else", model.LinkKindProbable)
	require.NoError(t, testDB.Create(&model.CatalogMatchRejection{
		EntityType: model.EntityTypeLabel, EntityID: 810, SourceID: site,
		ExternalID: "brandx.example.com", Reason: "test",
	}).Error)

	mkExtlink(t, 20, "twitter", "brandx", "src_vndb.producers_extlinks", "p2")
	mkExtlink(t, 21, "website", "https://brandx.example.com/", "src_vndb.producers_extlinks", "p2")

	st, err := run(context.Background(), testDB, Opts{Only: LaneLabel, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 0, st.Label.Planned)
	assert.Equal(t, 1, st.Label.SkippedIdentity)
	assert.Equal(t, 1, st.Label.SkippedRejection)
	assert.Equal(t, 0, st.Label.Written)
	assert.Zero(t, countRelated(t))
}
