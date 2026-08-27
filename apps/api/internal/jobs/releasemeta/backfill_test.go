package releasemeta

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcb "api/internal/platform/catalog/srcbangumi"
	srcv "api/internal/platform/catalog/srcvndb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	testDB    *gorm.DB
	testDSN   string
	dlTestDSN string
	egTestDSN string
)

func TestMain(m *testing.M) {
	testDSN = os.Getenv("TEST_DATABASE_DSN")
	if testDSN == "" {
		testDSN = "host=localhost port=5432 user=postgres password=postgres dbname=kun_catalog_test sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: cannot connect to test database: %v\n", err)
		os.Exit(0)
	}
	if err := migrate.Run(db); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: catalog migrate failed: %v\n", err)
		os.Exit(0)
	}
	if err := seed.Run(db); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: catalog seed failed: %v\n", err)
		os.Exit(0)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: src_bangumi schema failed: %v\n", err)
		os.Exit(0)
	}
	if err := srcv.EnsureSchema(db); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: src_vndb schema failed: %v\n", err)
		os.Exit(0)
	}
	for _, ddl := range []string{
		`CREATE SCHEMA IF NOT EXISTS releasemeta_dl`,
		`CREATE TABLE IF NOT EXISTS releasemeta_dl.works (workno text PRIMARY KEY, regist_date timestamptz, age_category text)`,
		`CREATE SCHEMA IF NOT EXISTS releasemeta_eg`,
		`CREATE TABLE IF NOT EXISTS releasemeta_eg.games (id int PRIMARY KEY, sellday text)`,
		`ALTER TABLE releasemeta_eg.games ADD COLUMN IF NOT EXISTS erogame boolean`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			fmt.Fprintf(os.Stderr, "SKIP: fixture schema failed: %v\n", err)
			os.Exit(0)
		}
	}
	dlTestDSN = testDSN + " options='-csearch_path=releasemeta_dl'"
	egTestDSN = testDSN + " options='-csearch_path=releasemeta_eg'"
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_external_ref", "catalog_release", "catalog_work", "src_bangumi.subject",
		"src_vndb.releases", "src_vndb.releases_vn",
		"releasemeta_dl.works", "releasemeta_eg.games",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func galgameMedium(t *testing.T) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&id).Error)
	return id
}

func mkWork(t *testing.T, medium int16, name string, site *string, productID *int64, rating int16) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: name,
		Site: site, ProductWorkID: productID, ContentRating: rating}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkRelease(t *testing.T, workID int64, y, m, d int16) int64 {
	t.Helper()
	rel := model.CatalogRelease{WorkID: workID, Kind: model.ReleaseKindDigital}
	if y != 0 {
		rel.ReleasedY, rel.ReleasedM, rel.ReleasedD = &y, &m, &d
	}
	require.NoError(t, testDB.Create(&rel).Error)
	return rel.ID
}

func mkWorkAnchor(t *testing.T, workID int64, externalID string, source, kind int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: source,
		ExternalID: externalID, LinkKind: kind, MatchedBy: "rule:test",
	}).Error)
}

func mkReleaseAnchor(t *testing.T, releaseID int64, workno string, source int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: releaseID, SourceID: source,
		ExternalID: workno, LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
	}).Error)
}

func mkSubject(t *testing.T, id int64, date string, nsfw bool) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id),
		Date: date, NSFW: nsfw,
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func mkSubjectMeta(t *testing.T, id int64, nsfw bool, meta ...string) {
	t.Helper()
	tags, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, testDB.Create(&srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id),
		NSFW: nsfw, MetaTags: datatypes.JSON(tags),
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func mkVndbRelease(t *testing.T, vid, rid string, minage *int16, hasEro, patch bool) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcv.Release{
		ID: rid, OLang: "ja", MinAge: minage, HasEro: hasEro, Patch: patch,
	}).Error)
	require.NoError(t, testDB.Create(&srcv.ReleaseVN{ID: rid, VID: vid, RType: "complete"}).Error)
}

func mkDlWork(t *testing.T, workno, regist, age string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO releasemeta_dl.works (workno, regist_date, age_category)
		 VALUES (?, NULLIF(?, '')::timestamptz, NULLIF(?, ''))`, workno, regist, age).Error)
}

func mkEgGame(t *testing.T, id int64, sellday string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO releasemeta_eg.games (id, sellday) VALUES (?, ?)`, id, sellday).Error)
}

func mkEgErogame(t *testing.T, id int64, erogame bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO releasemeta_eg.games (id, sellday, erogame) VALUES (?, '', ?)`, id, erogame).Error)
}

func relDate(t *testing.T, id int64) (y, m, d *int16) {
	t.Helper()
	var rel model.CatalogRelease
	require.NoError(t, testDB.First(&rel, id).Error)
	return rel.ReleasedY, rel.ReleasedM, rel.ReleasedD
}

func workRating(t *testing.T, id int64) int16 {
	t.Helper()
	var w model.CatalogWork
	require.NoError(t, testDB.First(&w, id).Error)
	return w.ContentRating
}

func assertDate(t *testing.T, relID int64, y, m, d int16) {
	t.Helper()
	gy, gm, gd := relDate(t, relID)
	require.NotNil(t, gy)
	assert.Equal(t, y, *gy)
	if m == 0 {
		assert.Nil(t, gm, "month should stay NULL")
	} else {
		require.NotNil(t, gm)
		assert.Equal(t, m, *gm)
	}
	if d == 0 {
		assert.Nil(t, gd, "day should stay NULL")
	} else {
		require.NotNil(t, gd)
		assert.Equal(t, d, *gd)
	}
}

func runOpts(apply bool) Opts {
	return Opts{Apply: apply, DSN: testDSN, DlsiteDSN: dlTestDSN, EGDSN: egTestDSN}
}

func str(s string) *string { return &s }
func i64(v int64) *int64   { return &v }

func TestBackfillReleaseMeta(t *testing.T) {
	clean(t)
	ctx := context.Background()
	medium := galgameMedium(t)
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)
	wiki := "galgame_wiki"

	wDl := mkWork(t, medium, "dl-date", nil, nil, 0)
	relDl := mkRelease(t, wDl, 0, 0, 0)
	mkReleaseAnchor(t, relDl, "RJ000001", reg.dlsiteSource)
	mkDlWork(t, "RJ000001", "2020-05-01 00:00:00+08", "")

	wDlNull := mkWork(t, medium, "dl-null-regist", nil, nil, 0)
	relDlNull := mkRelease(t, wDlNull, 0, 0, 0)
	mkReleaseAnchor(t, relDlNull, "RJ000002", reg.dlsiteSource)
	mkDlWork(t, "RJ000002", "", "")

	wDlMissing := mkWork(t, medium, "dl-missing", nil, nil, 0)
	relDlMissing := mkRelease(t, wDlMissing, 0, 0, 0)
	mkReleaseAnchor(t, relDlMissing, "RJ000003", reg.dlsiteSource)

	wDlFuture := mkWork(t, medium, "dl-placeholder", nil, nil, 0)
	relDlFuture := mkRelease(t, wDlFuture, 0, 0, 0)
	mkReleaseAnchor(t, relDlFuture, "RJ000004", reg.dlsiteSource)
	mkDlWork(t, "RJ000004", "2099-12-31 00:00:00+08", "")

	wDlNonEmpty := mkWork(t, medium, "dl-non-empty", nil, nil, 0)
	relDlNonEmpty := mkRelease(t, wDlNonEmpty, 1999, 12, 24)
	mkReleaseAnchor(t, relDlNonEmpty, "RJ000005", reg.dlsiteSource)
	mkDlWork(t, "RJ000005", "2020-01-01 00:00:00+08", "")

	wBoth := mkWork(t, medium, "dl-beats-eg", nil, nil, 0)
	relBoth := mkRelease(t, wBoth, 0, 0, 0)
	mkReleaseAnchor(t, relBoth, "RJ000006", reg.dlsiteSource)
	mkDlWork(t, "RJ000006", "2021-07-15 00:00:00+08", "")
	mkWorkAnchor(t, wBoth, "601", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 601, "2022-01-01")

	wEg := mkWork(t, medium, "eg-date", nil, nil, 0)
	relEg := mkRelease(t, wEg, 0, 0, 0)
	mkWorkAnchor(t, wEg, "602", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 602, "2004-05-28")

	wEgDlNull := mkWork(t, medium, "eg-fills-dl-null", nil, nil, 0)
	relEgDlNull := mkRelease(t, wEgDlNull, 0, 0, 0)
	mkReleaseAnchor(t, relEgDlNull, "RJ000007", reg.dlsiteSource)
	mkDlWork(t, "RJ000007", "", "")
	mkWorkAnchor(t, wEgDlNull, "603", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 603, "2010-10-10")

	wEgTwoRel := mkWork(t, medium, "eg-two-releases", nil, nil, 0)
	mkRelease(t, wEgTwoRel, 0, 0, 0)
	mkRelease(t, wEgTwoRel, 0, 0, 0)
	mkWorkAnchor(t, wEgTwoRel, "604", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 604, "2003-03-03")

	wEgClaimed := mkWork(t, medium, "eg-claimed", str(wiki), i64(9101), 0)
	mkRelease(t, wEgClaimed, 0, 0, 0)
	mkWorkAnchor(t, wEgClaimed, "605", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 605, "2005-05-05")

	wEgBad := mkWork(t, medium, "eg-placeholder", nil, nil, 0)
	relEgBad := mkRelease(t, wEgBad, 0, 0, 0)
	mkWorkAnchor(t, wEgBad, "606", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 606, "2050-01-01")

	wEgMissing := mkWork(t, medium, "eg-missing", nil, nil, 0)
	mkRelease(t, wEgMissing, 0, 0, 0)
	mkWorkAnchor(t, wEgMissing, "607", reg.egSource, model.LinkKindExact)

	wEgMulti := mkWork(t, medium, "eg-multi-anchor", nil, nil, 0)
	relEgMulti := mkRelease(t, wEgMulti, 0, 0, 0)
	mkWorkAnchor(t, wEgMulti, "608", reg.egSource, model.LinkKindExact)
	mkWorkAnchor(t, wEgMulti, "609", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 609, "2015-03-03")

	wBgmClaimed := mkWork(t, medium, "bgm-claimed", str(wiki), i64(9102), 0)
	relBgmClaimed := mkRelease(t, wBgmClaimed, 0, 0, 0)
	mkWorkAnchor(t, wBgmClaimed, "701", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 701, "2010-04-30", false)

	wBgmPartial := mkWork(t, medium, "bgm-partial", nil, nil, 0)
	relBgmPartial := mkRelease(t, wBgmPartial, 0, 0, 0)
	mkWorkAnchor(t, wBgmPartial, "702", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 702, "2015", false)

	wBgmEmpty := mkWork(t, medium, "bgm-empty-date", nil, nil, 0)
	mkRelease(t, wBgmEmpty, 0, 0, 0)
	mkWorkAnchor(t, wBgmEmpty, "703", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 703, "", false)

	wBgmGarbage := mkWork(t, medium, "bgm-garbage", nil, nil, 0)
	mkRelease(t, wBgmGarbage, 0, 0, 0)
	mkWorkAnchor(t, wBgmGarbage, "704", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 704, "TBA?", false)

	wBgmCovered := mkWork(t, medium, "eg-beats-bgm", nil, nil, 0)
	relBgmCovered := mkRelease(t, wBgmCovered, 0, 0, 0)
	mkWorkAnchor(t, wBgmCovered, "610", reg.egSource, model.LinkKindExact)
	mkEgGame(t, 610, "2001-02-03")
	mkWorkAnchor(t, wBgmCovered, "705", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 705, "2002-03-04", false)

	wBgmProbable := mkWork(t, medium, "bgm-probable", nil, nil, 0)
	mkRelease(t, wBgmProbable, 0, 0, 0)
	mkWorkAnchor(t, wBgmProbable, "706", reg.bangumiSource, model.LinkKindProbable)
	mkSubject(t, 706, "2011-11-11", true)

	rDlAdult := mkWork(t, medium, "rating-dl-adult", nil, nil, 0)
	relRDlAdult := mkRelease(t, rDlAdult, 2000, 1, 1)
	mkReleaseAnchor(t, relRDlAdult, "RJ000101", reg.dlsiteSource)
	mkDlWork(t, "RJ000101", "", "3")

	rDlR15 := mkWork(t, medium, "rating-dl-r15", nil, nil, 0)
	relRDlR15 := mkRelease(t, rDlR15, 2000, 1, 1)
	mkReleaseAnchor(t, relRDlR15, "RJ000102", reg.dlsiteSource)
	mkDlWork(t, "RJ000102", "", "2")

	rDlAll := mkWork(t, medium, "rating-dl-all", str(wiki), i64(9103), 0)
	relRDlAll := mkRelease(t, rDlAll, 2000, 1, 1)
	mkReleaseAnchor(t, relRDlAll, "RJ000103", reg.dlsiteSource)
	mkDlWork(t, "RJ000103", "", "1")

	rBgm := mkWork(t, medium, "rating-bgm-nsfw", nil, nil, 0)
	mkWorkAnchor(t, rBgm, "708", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 708, "", true)

	rBgmFalse := mkWork(t, medium, "rating-bgm-sfw", nil, nil, 0)
	mkWorkAnchor(t, rBgmFalse, "709", reg.bangumiSource, model.LinkKindExact)
	mkSubject(t, 709, "", false)

	rRated := mkWork(t, medium, "rating-already-rated", nil, nil, model.ContentRatingR18)
	relRRated := mkRelease(t, rRated, 2000, 1, 1)
	mkReleaseAnchor(t, relRRated, "RJ000104", reg.dlsiteSource)
	mkDlWork(t, "RJ000104", "", "2")

	age18 := int16(18)
	age0 := int16(0)

	rVndb18 := mkWork(t, medium, "rating-vndb-minage", nil, nil, 0)
	mkWorkAnchor(t, rVndb18, "v901", reg.vndbSource, model.LinkKindExact)
	mkVndbRelease(t, "v901", "r901", &age18, false, false)

	rVndbEro := mkWork(t, medium, "rating-vndb-ero", nil, nil, 0)
	mkWorkAnchor(t, rVndbEro, "v902", reg.vndbSource, model.LinkKindExact)
	mkVndbRelease(t, "v902", "r902", nil, true, false)

	rVndbPatch := mkWork(t, medium, "rating-vndb-patch-only", nil, nil, 0)
	mkWorkAnchor(t, rVndbPatch, "v903", reg.vndbSource, model.LinkKindExact)
	mkVndbRelease(t, "v903", "r903", &age18, true, true)

	rVndbSafe := mkWork(t, medium, "rating-vndb-all-ages", nil, nil, 0)
	mkWorkAnchor(t, rVndbSafe, "v904", reg.vndbSource, model.LinkKindExact)
	mkVndbRelease(t, "v904", "r904", &age0, false, false)

	rVndbBeatsDl := mkWork(t, medium, "rating-vndb-beats-dl-allages", nil, nil, 0)
	relVndbBeatsDl := mkRelease(t, rVndbBeatsDl, 2000, 1, 1)
	mkReleaseAnchor(t, relVndbBeatsDl, "RJ000105", reg.dlsiteSource)
	mkDlWork(t, "RJ000105", "", "1")
	mkWorkAnchor(t, rVndbBeatsDl, "v905", reg.vndbSource, model.LinkKindExact)
	mkVndbRelease(t, "v905", "r905", &age18, true, false)

	rEgTrue := mkWork(t, medium, "rating-eg-erogame", nil, nil, 0)
	mkWorkAnchor(t, rEgTrue, "611", reg.egSource, model.LinkKindExact)
	mkEgErogame(t, 611, true)

	rEgFalse := mkWork(t, medium, "rating-eg-not-erogame", nil, nil, 0)
	mkWorkAnchor(t, rEgFalse, "612", reg.egSource, model.LinkKindExact)
	mkEgErogame(t, 612, false)

	rEgBeatsDl := mkWork(t, medium, "rating-eg-beats-dl-allages", nil, nil, 0)
	relEgBeatsDl := mkRelease(t, rEgBeatsDl, 2000, 1, 1)
	mkReleaseAnchor(t, relEgBeatsDl, "RJ000106", reg.dlsiteSource)
	mkDlWork(t, "RJ000106", "", "1")
	mkWorkAnchor(t, rEgBeatsDl, "613", reg.egSource, model.LinkKindExact)
	mkEgErogame(t, 613, true)

	rBgmMeta := mkWork(t, medium, "rating-bgm-meta-tag", nil, nil, 0)
	mkWorkAnchor(t, rBgmMeta, "710", reg.bangumiSource, model.LinkKindExact)
	mkSubjectMeta(t, 710, false, "游戏", "R18")

	st, err := Run(ctx, runOpts(false))
	require.NoError(t, err)
	assert.Equal(t, 6, st.DlDateCandidates)
	assert.Equal(t, 2, st.DlDateNoRegist, "RJ000002 + RJ000007")
	assert.Equal(t, 1, st.DlDateMissingMirror)
	assert.Equal(t, 1, st.DlDateOutOfRange, "2099 placeholder gated")
	assert.Equal(t, 2, st.DlDatePlanned, "relDl + relBoth")
	assert.Equal(t, 7, st.EgDateCandidates, "two-release + claimed excluded in SQL")
	assert.Equal(t, 1, st.EgDateCovered, "relBoth belongs to the dlsite lane")
	assert.Equal(t, 1, st.EgDateMultiAnchor)
	assert.Equal(t, 1, st.EgDateMissingMirror)
	assert.Equal(t, 1, st.EgDateBadDate, "2050 placeholder gated")
	assert.Equal(t, 4, st.EgDatePlanned, "wEg + wEgDlNull + wEgMulti + wBgmCovered")
	assert.Equal(t, 5, st.BgmDateCandidates, "probable + release-less excluded in SQL")
	assert.Equal(t, 1, st.BgmDateCovered, "relBgmCovered belongs to the eg lane")
	assert.Equal(t, 1, st.BgmDateNoDate)
	assert.Equal(t, 1, st.BgmDateBadDate)
	assert.Equal(t, 1, st.BgmDatePartial)
	assert.Equal(t, 2, st.BgmDatePlanned, "wBgmClaimed + wBgmPartial")
	assert.Equal(t, 33, st.RatingCandidates, "every rating-0 work; rRated excluded")
	assert.Equal(t, 3, st.RatingVndbR18, "minage + has_ero + the dl-allages preemption")
	assert.Equal(t, 1, st.RatingDlR18)
	assert.Equal(t, 1, st.RatingDlSensitive)
	assert.Equal(t, 1, st.RatingDlAllAges, "explicit all-ages verdict keeps the row at 0")
	assert.Equal(t, 2, st.RatingEgR18, "erogame=true + the dl-allages preemption")
	assert.Equal(t, 2, st.RatingBgmR18, "nsfw flag + R18 meta_tag")
	assert.Equal(t, 23, st.RatingNoVerdict)
	assert.Equal(t, 9, st.RatingPlanned)
	assert.Zero(t, st.DlDateFilled+st.EgDateFilled+st.BgmDateFilled+st.RatingFilled+
		st.DlDateSkippedNonEmpty+st.EgDateSkippedNonEmpty+st.BgmDateSkippedNonEmpty+
		st.RatingSkippedNonEmpty+st.Errors)
	y, _, _ := relDate(t, relDl)
	assert.Nil(t, y, "dry run writes nothing")
	assert.Equal(t, int16(0), workRating(t, rDlAdult), "dry run writes nothing")

	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 2, st.DlDateFilled)
	assert.Equal(t, 4, st.EgDateFilled)
	assert.Equal(t, 2, st.BgmDateFilled)
	assert.Equal(t, 9, st.RatingFilled)
	assert.Zero(t, st.DlDateSkippedNonEmpty+st.EgDateSkippedNonEmpty+st.BgmDateSkippedNonEmpty+
		st.RatingSkippedNonEmpty+st.Errors)

	assertDate(t, relDl, 2020, 5, 1)
	assertDate(t, relBoth, 2021, 7, 15)
	assertDate(t, relEg, 2004, 5, 28)
	assertDate(t, relEgDlNull, 2010, 10, 10)
	assertDate(t, relEgMulti, 2015, 3, 3)
	assertDate(t, relBgmCovered, 2001, 2, 3)
	assertDate(t, relBgmClaimed, 2010, 4, 30)
	assertDate(t, relBgmPartial, 2015, 0, 0)
	assertDate(t, relDlNonEmpty, 1999, 12, 24)
	yBad, _, _ := relDate(t, relEgBad)
	assert.Nil(t, yBad, "gated placeholder never lands")
	yFut, _, _ := relDate(t, relDlFuture)
	assert.Nil(t, yFut)

	assert.Equal(t, model.ContentRatingR18, workRating(t, rDlAdult))
	assert.Equal(t, model.ContentRatingSensitive, workRating(t, rDlR15))
	assert.Equal(t, int16(0), workRating(t, rDlAll), "an explicit all-ages verdict leaves the row at 0")
	assert.Equal(t, model.ContentRatingR18, workRating(t, rBgm))
	assert.Equal(t, int16(0), workRating(t, rBgmFalse), "nsfw=false never infers a rating")
	assert.Equal(t, model.ContentRatingR18, workRating(t, rRated), "non-zero rating untouched")
	assert.Equal(t, model.ContentRatingR18, workRating(t, rVndb18))
	assert.Equal(t, model.ContentRatingR18, workRating(t, rVndbEro))
	assert.Equal(t, int16(0), workRating(t, rVndbPatch), "an 18+ patch is not the work's rating")
	assert.Equal(t, int16(0), workRating(t, rVndbSafe))
	assert.Equal(t, model.ContentRatingR18, workRating(t, rVndbBeatsDl),
		"work-level vndb verdict outranks the 全年齢版 SKU's dlsite age")
	assert.Equal(t, model.ContentRatingR18, workRating(t, rEgTrue))
	assert.Equal(t, model.ContentRatingR18, workRating(t, rEgBeatsDl),
		"work-level EG erogame outranks the 全年齢版 SKU's dlsite age")
	assert.Equal(t, int16(0), workRating(t, rEgFalse), "erogame=false never infers a rating")
	assert.Equal(t, model.ContentRatingR18, workRating(t, rBgmMeta))

	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 3, st.DlDateCandidates, "only the unfillable three remain")
	assert.Equal(t, 2, st.EgDateCandidates, "bad-date + missing-mirror remain")
	assert.Equal(t, 2, st.BgmDateCandidates, "no-date + garbage remain")
	assert.Equal(t, 24, st.RatingCandidates, "the nine filled works left the set")
	assert.Zero(t, st.DlDatePlanned+st.EgDatePlanned+st.BgmDatePlanned+st.RatingPlanned,
		"second pass plans zero")
	assert.Zero(t, st.DlDateFilled+st.EgDateFilled+st.BgmDateFilled+st.RatingFilled+st.Errors,
		"second pass writes zero")
	assert.Equal(t, 1, st.RatingDlAllAges, "all-ages verdicts persist as counted no-ops")
}

func TestFillEmptyGuardAndDSNRequired(t *testing.T) {
	clean(t)
	ctx := context.Background()
	medium := galgameMedium(t)

	wDated := mkWork(t, medium, "guard-dated", nil, nil, 0)
	relDated := mkRelease(t, wDated, 2001, 2, 3)
	wRated := mkWork(t, medium, "guard-rated", nil, nil, model.ContentRatingSensitive)

	w := &writer{db: testDB, stats: &Stats{}}
	var filled, skipped int
	m, d := int16(6), int16(7)
	w.fillDate(ctx, relDated, 2020, &m, &d, true, &filled, &skipped)
	assert.Zero(t, filled)
	assert.Equal(t, 1, skipped, "non-empty date refused at write time")
	assertDate(t, relDated, 2001, 2, 3)

	w.fillRating(ctx, wRated, model.ContentRatingR18, true)
	assert.Zero(t, w.stats.RatingFilled)
	assert.Equal(t, 1, w.stats.RatingSkippedNonEmpty, "non-zero rating refused at write time")
	assert.Equal(t, model.ContentRatingSensitive, workRating(t, wRated))

	for _, opts := range []Opts{
		{DlsiteDSN: testDSN, EGDSN: testDSN},
		{DSN: testDSN, EGDSN: testDSN},
		{DSN: testDSN, DlsiteDSN: testDSN},
	} {
		_, err := Run(context.Background(), opts)
		require.Error(t, err)
	}
}
