package workratings

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

var (
	testDB      *gorm.DB
	testDSN     string
	egTestDSN   string
	dlTestDSN   string
	hltbTestDSN string
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/workratings")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/workratings", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workratings", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workratings", "catalog seed failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/workratings", "src_bangumi schema failed: %v", err)
	}
	if err := srcv.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/workratings", "src_vndb schema failed: %v", err)
	}
	for _, ddl := range []string{
		`CREATE SCHEMA IF NOT EXISTS workratings_eg`,
		// `raw` mirrors the EG mirror's own shape: it generates typed columns for
		// a handful of keys and leaves the rest — the spread four included — in
		// the jsonb, which is where the EG lane reads them from.
		`CREATE TABLE IF NOT EXISTS workratings_eg.games (id int PRIMARY KEY, median int, count2 int, raw jsonb)`,
		// CREATE TABLE IF NOT EXISTS adds no columns to a table that already
		// exists, so a test database created before wave 205 kept a `raw`-less
		// games fixture and every EG assertion failed with `column "raw" does not
		// exist`. Widen the fixture explicitly whenever a column is added.
		`ALTER TABLE workratings_eg.games ADD COLUMN IF NOT EXISTS raw jsonb`,
		`CREATE TABLE IF NOT EXISTS workratings_eg.reviews (game int, tokuten int)`,
		`CREATE SCHEMA IF NOT EXISTS workratings_dl`,
		`CREATE TABLE IF NOT EXISTS workratings_dl.works (workno text PRIMARY KEY, info_json jsonb)`,
		`CREATE SCHEMA IF NOT EXISTS workratings_hltb`,
		`CREATE TABLE IF NOT EXISTS workratings_hltb.games (hltb_id bigint PRIMARY KEY, raw jsonb)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			dbtest.SkipMainf("jobs/workratings", "mirror fixture failed: %v", err)
		}
	}
	egTestDSN = testDSN + " options='-csearch_path=workratings_eg'"
	dlTestDSN = testDSN + " options='-csearch_path=workratings_dl'"
	hltbTestDSN = testDSN + " options='-csearch_path=workratings_hltb'"
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_rating", "catalog_work_popularity", "catalog_external_ref", "catalog_release",
		"catalog_work", "src_bangumi.subject", "workratings_eg.games", "workratings_eg.reviews",
		"workratings_dl.works", "workratings_hltb.games", "src_vndb.vn", "src_vndb.vn_vote_stats",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkWork(t *testing.T, medium int16, name string, site *string) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: name, Site: site}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkAnchor(t *testing.T, workID int64, externalID string, source, kind int16, matchedBy string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: source,
		ExternalID: externalID, LinkKind: kind, MatchedBy: matchedBy,
	}).Error)
}

func mkReleaseAnchor(t *testing.T, workID int64, externalID string, source, kind int16) {
	t.Helper()
	rel := model.CatalogRelease{WorkID: workID, Kind: model.ReleaseKindDigital}
	require.NoError(t, testDB.Create(&rel).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: source,
		ExternalID: externalID, LinkKind: kind, MatchedBy: "rule:test",
	}).Error)
}

func mkSubject(t *testing.T, id int64, score float64, rank int, details string) {
	t.Helper()
	sub := srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id), NameCN: "",
		InfoboxRaw: "", ParseError: "", Summary: "", Date: "",
		Score: score, Rank: rank,
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}
	if details != "" {
		sub.ScoreDetails = []byte(details)
	}
	require.NoError(t, testDB.Create(&sub).Error)
}

func mkEGGame(t *testing.T, id int, median *int, count2 int) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO workratings_eg.games (id, median, count2) VALUES (?, ?, ?)`, id, median, count2).Error)
}

func setEGSpread(t *testing.T, id int, raw string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`UPDATE workratings_eg.games SET raw = ?::jsonb WHERE id = ?`, raw, id).Error)
}

func setDlsiteRateDetail(t *testing.T, workno, detail string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`UPDATE workratings_dl.works SET info_json = info_json || jsonb_build_object('rate_count_detail', ?::jsonb)
		 WHERE workno = ?`, detail, workno).Error)
}

func mkDlsiteWork(t *testing.T, workno string, star *float64, rc *int, dl, wl *int64, rv *int) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO workratings_dl.works (workno, info_json)
		VALUES (?, jsonb_strip_nulls(jsonb_build_object(
			'rate_average_2dp', ?::float8, 'rate_count', ?::int,
			'dl_count', ?::bigint, 'wishlist_count', ?::bigint, 'review_count', ?::int)))`,
		workno, star, rc, dl, wl, rv).Error)
}

func ratingCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_work_rating "+where, args...).Scan(&n).Error)
	return n
}

func popCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_work_popularity "+where, args...).Scan(&n).Error)
	return n
}

func p(v int) *int          { return &v }
func pf(v float64) *float64 { return &v }
func pl(v int64) *int64     { return &v }

func runOpts(apply bool) Opts {
	return Opts{DSN: testDSN, EGDSN: egTestDSN, DlsiteDSN: dlTestDSN, HltbDSN: hltbTestDSN, Apply: apply}
}

func TestBackfillWorkRatings(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	claimed := "galgame_wiki"

	wBgm := mkWork(t, reg.galgameMedium, "bgm-scored", nil)
	wBgmZero := mkWork(t, reg.galgameMedium, "bgm-unrated", nil)
	wBgmNoRank := mkWork(t, reg.galgameMedium, "bgm-norank", nil)
	wBgmClaimed := mkWork(t, reg.galgameMedium, "bgm-claimed", &claimed)
	wBgmProbable := mkWork(t, reg.galgameMedium, "bgm-probable", nil)
	mkSubject(t, 101, 7.4, 321, `{"1":0,"5":10,"7":20,"10":12}`)
	mkSubject(t, 102, 0, 0, `{"1":0}`)
	mkSubject(t, 103, 5.5, 0, `{"5":3}`)
	mkSubject(t, 104, 8.0, 1, `{"10":5}`)
	mkSubject(t, 105, 8.0, 1, `{"10":5}`)
	mkAnchor(t, wBgm, "101", reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wBgmZero, "102", reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wBgmNoRank, "103", reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wBgmClaimed, "104", reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wBgmProbable, "105", reg.bangumiSource, model.LinkKindProbable, "rule:bgm-title-only")

	wEg := mkWork(t, reg.galgameMedium, "eg-scored", nil)
	wEgNoMedian := mkWork(t, reg.galgameMedium, "eg-nomedian", nil)
	wEgMissing := mkWork(t, reg.galgameMedium, "eg-missing", nil)
	wEgMulti := mkWork(t, reg.galgameMedium, "eg-multianchor", nil)
	wEgClaimed := mkWork(t, reg.galgameMedium, "eg-claimed", &claimed)
	mkEGGame(t, 1001, p(78), 40)
	setEGSpread(t, 1001, `{"average2":"63","stdev":"14","min2":"0","max2":"90"}`)
	for _, tk := range []*int{p(0), p(78), p(100), nil} {
		mkEGReview(t, 1001, tk)
	}
	mkEGGame(t, 1002, nil, 5)
	mkEGGame(t, 1004, p(50), 10)
	mkEGGame(t, 1014, p(90), 99)
	mkEGGame(t, 1005, p(60), 20)
	mkAnchor(t, wEg, "1001", reg.egSource, model.LinkKindExact, "rule:test")
	mkAnchor(t, wEgNoMedian, "1002", reg.egSource, model.LinkKindExact, "rule:test")
	mkAnchor(t, wEgMissing, "1003", reg.egSource, model.LinkKindExact, "rule:test")
	mkAnchor(t, wEgMulti, "1004", reg.egSource, model.LinkKindExact, "rule:test")
	mkAnchor(t, wEgMulti, "1014", reg.egSource, model.LinkKindExact, "rule:test")
	mkAnchor(t, wEgClaimed, "1005", reg.egSource, model.LinkKindExact, "rule:test")

	mkEGGame(t, 1006, p(70), 7)
	mkAnchor(t, wBgm, "1006", reg.egSource, model.LinkKindExact, "rule:test")

	wDl := mkWork(t, reg.galgameMedium, "dl-full", nil)
	mkReleaseAnchor(t, wDl, "RJ100001", reg.dlsiteSource, model.LinkKindExact)
	mkDlsiteWork(t, "RJ100001", pf(4.36), p(120), pl(2000), pl(300), p(12))
	setDlsiteRateDetail(t, "RJ100001", `[{"count":2,"ratio":2,"review_point":1},{"count":0,"ratio":0,"review_point":2},
		{"count":8,"ratio":7,"review_point":3},{"count":30,"ratio":25,"review_point":4},
		{"count":80,"ratio":66,"review_point":5}]`)
	wDlNoRating := mkWork(t, reg.galgameMedium, "dl-norating", nil)
	mkReleaseAnchor(t, wDlNoRating, "RJ100002", reg.dlsiteSource, model.LinkKindExact)
	mkDlsiteWork(t, "RJ100002", nil, nil, nil, pl(7), p(0))
	wDlMissing := mkWork(t, reg.galgameMedium, "dl-missing", nil)
	mkReleaseAnchor(t, wDlMissing, "RJ100003", reg.dlsiteSource, model.LinkKindExact)
	wDlClaimed := mkWork(t, reg.galgameMedium, "dl-claimed", &claimed)
	mkReleaseAnchor(t, wDlClaimed, "RJ100004", reg.dlsiteSource, model.LinkKindExact)
	mkDlsiteWork(t, "RJ100004", pf(4.9), p(999), pl(1), pl(1), p(1))
	wDlAsmr := mkWork(t, 5, "dl-asmr", nil)
	mkReleaseAnchor(t, wDlAsmr, "RJ100005", reg.dlsiteSource, model.LinkKindExact)
	mkDlsiteWork(t, "RJ100005", pf(4.9), p(999), pl(1), pl(1), p(1))
	wDlProbable := mkWork(t, reg.galgameMedium, "dl-probable", nil)
	mkReleaseAnchor(t, wDlProbable, "RJ100006", reg.dlsiteSource, model.LinkKindProbable)
	mkDlsiteWork(t, "RJ100006", pf(4.0), p(10), pl(1), pl(1), p(1))

	st, err := Run(ctx, runOpts(false))
	require.NoError(t, err)
	assert.Equal(t, 4, st.BgmCandidates, "claimed included now; probable still excluded in SQL")
	assert.Equal(t, 1, st.BgmNoScore)
	assert.Equal(t, 3, st.BgmPlanned, "wBgm + wBgmNoRank + wBgmClaimed")
	assert.Equal(t, 6, st.EgCandidates, "wBgm's EG anchor joins the lane; claimed included now")
	assert.Equal(t, 1, st.EgMultiAnchor, "wEgMulti's second anchor collapsed")
	assert.Equal(t, 1, st.EgMissingMirror)
	assert.Equal(t, 1, st.EgNoMedian)
	assert.Equal(t, 4, st.EgPlanned, "wEg + wEgMulti + wBgm + wEgClaimed")
	assert.Equal(t, 4, st.DlCandidates, "claimed included now; probable / asmr excluded in SQL")
	assert.Equal(t, 1, st.DlMissingMirror)
	assert.Equal(t, 1, st.DlNoRating, "wDlNoRating publishes no rating")
	assert.Equal(t, 2, st.DlRatingPlanned, "wDl + wDlClaimed")
	assert.Equal(t, 3, st.BgmDistribution, "every scored bangumi subject carries score_details")
	assert.Equal(t, 1, st.DlDistribution, "only RJ100001 was given a rate_count_detail")
	assert.Equal(t, 1, st.EgStats, "only game 1001 was given the spread keys")
	assert.Equal(t, 1, st.EgDistribution, "only game 1001 was given scored reviews")
	assert.Equal(t, 3, st.EgNoReviews, "the other three planned EG works have no scored review")
	assert.Equal(t, 8, st.PopPlanned, "wDl 3 + wDlNoRating 2 + wDlClaimed 3")
	assert.Zero(t, st.BgmWritten+st.EgWritten+st.DlRatingWritten+st.PopWritten+
		st.BgmUnchanged+st.EgUnchanged+st.DlRatingUnchanged+st.PopUnchanged+st.Errors)
	assert.EqualValues(t, 0, ratingCount(t, ""), "dry run writes nothing")
	assert.EqualValues(t, 0, popCount(t, ""), "dry run writes nothing")
	require.NotEmpty(t, st.BgmSamples)
	assert.Equal(t, wBgm, st.BgmSamples[0].WorkID)
	assert.Equal(t, 42, st.BgmSamples[0].VoteCount, "vote_count = summed score_details buckets")
	require.NotEmpty(t, st.DlSamples)
	assert.Equal(t, "RJ100001", st.DlSamples[0].Workno)

	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 3, st.BgmWritten)
	assert.Equal(t, 4, st.EgWritten)
	assert.Equal(t, 2, st.DlRatingWritten)
	assert.Equal(t, 8, st.PopWritten)
	assert.Zero(t, st.BgmUnchanged+st.EgUnchanged+st.DlRatingUnchanged+st.PopUnchanged+st.Errors)
	assert.EqualValues(t, 9, ratingCount(t, ""))
	assert.EqualValues(t, 8, popCount(t, ""))

	var rBgm model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wBgm, reg.bangumiSource).First(&rBgm).Error)
	assert.InDelta(t, 7.4, rBgm.Score, 1e-9)
	assert.Equal(t, 42, rBgm.VoteCount)
	require.NotNil(t, rBgm.Rank)
	assert.Equal(t, 321, *rBgm.Rank)
	assert.JSONEq(t, `{"5":10,"7":20,"10":12}`, string(rBgm.Distribution),
		"the histogram vote_count was summed from, minus its empty buckets")
	assert.Nil(t, rBgm.Stats, "bangumi publishes a histogram, not a spread")

	var rNoRank model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ?", wBgmNoRank).First(&rNoRank).Error)
	assert.Nil(t, rNoRank.Rank, "unranked subject stores NULL, never a fake 0")

	var rEg model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wEg, reg.egSource).First(&rEg).Error)
	assert.InDelta(t, 78, rEg.Score, 1e-9)
	assert.Equal(t, 40, rEg.VoteCount)
	assert.Nil(t, rEg.Rank)
	assert.JSONEq(t, `{"0":1,"70":1,"100":1}`, string(rEg.Distribution),
		"the histogram is folded from `reviews`; the NULL-tokuten row is comment-only and never counted")
	assert.JSONEq(t, `{"average":63,"stdev":14,"min":0,"max":90}`, string(rEg.Stats),
		"the spread comes from the same games row as the median")
	assert.Equal(t, 40, rEg.VoteCount,
		"vote_count stays the games row's count2 — the two mirrors sync independently and the bars need not sum to it")

	var rEgNoSpread model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wEgMulti, reg.egSource).First(&rEgNoSpread).Error)
	assert.Nil(t, rEgNoSpread.Stats, "a game with no spread keys stores NULL, not an empty object")

	var rMulti model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ?", wEgMulti).First(&rMulti).Error)
	assert.InDelta(t, 90, rMulti.Score, 1e-9)
	assert.Equal(t, 99, rMulti.VoteCount)

	var rDl model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wDl, reg.dlsiteSource).First(&rDl).Error)
	assert.InDelta(t, 4.36, rDl.Score, 1e-9)
	assert.Equal(t, 120, rDl.VoteCount)
	assert.Nil(t, rDl.Rank)
	assert.JSONEq(t, `{"1":2,"3":8,"4":30,"5":80}`, string(rDl.Distribution),
		"rate_count_detail folded onto the 1-5 star scale, empty buckets dropped")

	var rDlNoDetail model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wDlClaimed, reg.dlsiteSource).First(&rDlNoDetail).Error)
	assert.Nil(t, rDlNoDetail.Distribution, "a work whose payload omits rate_count_detail stores NULL")

	var pops []model.CatalogWorkPopularity
	require.NoError(t, testDB.Where("work_id = ?", wDl).Order("metric").Find(&pops).Error)
	require.Len(t, pops, 3)
	assert.Equal(t, model.PopularityMetricDownloads, pops[0].Metric)
	assert.EqualValues(t, 2000, pops[0].Value)
	assert.Equal(t, model.PopularityMetricWishlist, pops[1].Metric)
	assert.EqualValues(t, 300, pops[1].Value)
	assert.Equal(t, model.PopularityMetricReviews, pops[2].Metric)
	assert.EqualValues(t, 12, pops[2].Value)
	assert.Equal(t, reg.dlsiteSource, pops[0].SourceID)
	require.NoError(t, testDB.Where("work_id = ?", wDlNoRating).Order("metric").Find(&pops).Error)
	require.Len(t, pops, 2)
	assert.Equal(t, model.PopularityMetricWishlist, pops[0].Metric)
	assert.EqualValues(t, 7, pops[0].Value)
	assert.Equal(t, model.PopularityMetricReviews, pops[1].Metric)
	assert.EqualValues(t, 0, pops[1].Value)

	assert.EqualValues(t, 2, ratingCount(t, "WHERE work_id = ?", wBgm))
	assert.EqualValues(t, 3, ratingCount(t, "WHERE work_id IN (?, ?, ?)", wBgmClaimed, wEgClaimed, wDlClaimed),
		"CLAIMED works materialise now — the claim guard is gone")
	assert.EqualValues(t, 3, popCount(t, "WHERE work_id = ?", wDlClaimed), "and so do their counters")
	assert.EqualValues(t, 0, popCount(t, "WHERE work_id = ?", wDlAsmr),
		"off-domain works never materialise")

	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Zero(t, st.BgmWritten+st.EgWritten+st.DlRatingWritten+st.PopWritten+st.Errors, "second pass writes zero")
	assert.Equal(t, 3, st.BgmUnchanged)
	assert.Equal(t, 4, st.EgUnchanged)
	assert.Equal(t, 2, st.DlRatingUnchanged)
	assert.Equal(t, 8, st.PopUnchanged)
	assert.EqualValues(t, 9, ratingCount(t, ""), "row count unchanged")
	assert.EqualValues(t, 8, popCount(t, ""), "row count unchanged")

	require.NoError(t, testDB.Exec(
		`UPDATE workratings_dl.works SET info_json = info_json || '{"wishlist_count": 301, "rate_count": 121}' WHERE workno = 'RJ100001'`).Error)
	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 1, st.DlRatingWritten, "rate_count change updates the rating row")
	assert.Equal(t, 1, st.PopWritten, "exactly the mutated wishlist row updates")
	assert.Equal(t, 7, st.PopUnchanged)
	assert.Zero(t, st.BgmWritten+st.EgWritten, "untouched lanes stay no-op")
	assert.EqualValues(t, 9, ratingCount(t, ""), "update in place — no row growth")
	assert.EqualValues(t, 8, popCount(t, ""))
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wDl, reg.dlsiteSource).First(&rDl).Error)
	assert.Equal(t, 121, rDl.VoteCount, "refreshed vote_count lands")
	var wl model.CatalogWorkPopularity
	require.NoError(t, testDB.Where("work_id = ? AND metric = ?", wDl, model.PopularityMetricWishlist).First(&wl).Error)
	assert.EqualValues(t, 301, wl.Value, "refreshed wishlist lands")
}

func TestClaimPeerWritesAndDSNRequired(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	claimed := "galgame_wiki"
	wClaimed := mkWork(t, reg.galgameMedium, "claimed-direct", &claimed)
	wBody := mkWork(t, reg.galgameMedium, "bodyless-direct", nil)

	w := &writer{db: testDB, stats: &Stats{}}
	var written, unchanged int
	w.write(ctx, plannedRow{WorkID: wClaimed, SourceID: reg.bangumiSource, Score: 7.0}, true, &written, &unchanged)
	w.writePopularity(ctx, popPlannedRow{WorkID: wClaimed, SourceID: reg.dlsiteSource,
		Metric: model.PopularityMetricDownloads, Value: 5}, true)
	assert.Equal(t, 1, written)
	assert.Equal(t, 1, w.stats.PopWritten)
	assert.EqualValues(t, 1, ratingCount(t, ""))
	assert.EqualValues(t, 1, popCount(t, ""))

	w.write(ctx, plannedRow{WorkID: wBody, SourceID: reg.bangumiSource, Score: 7.0, VoteCount: 3}, true, &written, &unchanged)
	assert.Equal(t, 2, written)
	w.write(ctx, plannedRow{WorkID: wBody, SourceID: reg.bangumiSource, Score: 7.0, VoteCount: 3}, true, &written, &unchanged)
	assert.Equal(t, 1, unchanged, "unchanged values → no-op")
	w.write(ctx, plannedRow{WorkID: wBody, SourceID: reg.bangumiSource, Score: 7.2, VoteCount: 4}, true, &written, &unchanged)
	assert.Equal(t, 3, written, "changed values → in-place update")

	// The conflict guard has to see the detail columns too. If it compared only
	// (score, vote_count, rank), the first run after wave 205 would have reported
	// `unchanged` for every work whose score had not moved and left the new
	// columns NULL — a backfill that silently backfills nothing.
	same := plannedRow{WorkID: wBody, SourceID: reg.bangumiSource, Score: 7.2, VoteCount: 4}
	withDist := same
	withDist.Distribution = []byte(`{"7":4}`)
	w.write(ctx, withDist, true, &written, &unchanged)
	assert.Equal(t, 4, written, "a new histogram alone is a change")
	w.write(ctx, withDist, true, &written, &unchanged)
	assert.Equal(t, 2, unchanged, "and re-writing the identical histogram is not")
	withStats := withDist
	withStats.Stats = []byte(`{"average":63}`)
	w.write(ctx, withStats, true, &written, &unchanged)
	assert.Equal(t, 5, written, "so is a new stats blob alone")
	assert.EqualValues(t, 2, ratingCount(t, ""), "the claimed row plus this one")
	var r model.CatalogWorkRating
	require.NoError(t, testDB.Where("work_id = ?", wBody).First(&r).Error)
	assert.InDelta(t, 7.2, r.Score, 1e-9)

	w.writePopularity(ctx, popPlannedRow{WorkID: wBody, SourceID: reg.dlsiteSource,
		Metric: model.PopularityMetricWishlist, Value: 10}, true)
	w.writePopularity(ctx, popPlannedRow{WorkID: wBody, SourceID: reg.dlsiteSource,
		Metric: model.PopularityMetricWishlist, Value: 10}, true)
	w.writePopularity(ctx, popPlannedRow{WorkID: wBody, SourceID: reg.dlsiteSource,
		Metric: model.PopularityMetricWishlist, Value: 11}, true)
	assert.Equal(t, 3, w.stats.PopWritten)
	assert.Equal(t, 1, w.stats.PopUnchanged)
	assert.EqualValues(t, 2, popCount(t, ""), "the claimed row plus this one")
	var pop model.CatalogWorkPopularity
	require.NoError(t, testDB.Where("work_id = ?", wBody).First(&pop).Error)
	assert.EqualValues(t, 11, pop.Value)

	_, err = Run(ctx, Opts{EGDSN: testDSN, DlsiteDSN: testDSN})
	require.Error(t, err)
	_, err = Run(ctx, Opts{DSN: testDSN, DlsiteDSN: testDSN})
	require.Error(t, err)
	_, err = Run(ctx, Opts{DSN: testDSN, EGDSN: testDSN})
	require.Error(t, err)
}

func TestNonTitleYearExactAnchorEntersBgmCandidates(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	wGated := mkWork(t, reg.galgameMedium, "bgm-gated", nil)
	mkSubject(t, 601, 7.4, 321, `{"7":20,"10":12}`)
	mkAnchor(t, wGated, "601", reg.bangumiSource, model.LinkKindExact, "rule:bgm-type4-gated")

	wProbable := mkWork(t, reg.galgameMedium, "bgm-probable", nil)
	mkSubject(t, 602, 8.0, 1, `{"10":5}`)
	mkAnchor(t, wProbable, "602", reg.bangumiSource, model.LinkKindProbable, "rule:bgm-title-only")

	st, err := Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 1, st.BgmCandidates, "the gated-rule exact anchor is now a bgm candidate; probable stays out")
	assert.EqualValues(t, 1, ratingCount(t, "WHERE work_id = ? AND source_id = ?", wGated, reg.bangumiSource))
	assert.EqualValues(t, 0, ratingCount(t, "WHERE work_id = ?", wProbable))
}

func workUpdatedAt(t *testing.T, id int64) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, testDB.Raw(`SELECT updated_at FROM catalog_work WHERE id = ?`, id).Scan(&ts).Error)
	return ts
}

func TestRatingWriteTouchesHostWork(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	wRated := mkWork(t, reg.galgameMedium, "touch-rated", nil)
	mkAnchor(t, wRated, "801", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")
	mkSubject(t, 801, 7.5, 120, `{"1":1,"8":9}`)

	wBystander := mkWork(t, reg.galgameMedium, "touch-bystander", nil)
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, testDB.Exec(`UPDATE catalog_work SET updated_at = ?`, stamp).Error)

	_, err = Run(ctx, runOpts(false))
	require.NoError(t, err)
	assert.True(t, workUpdatedAt(t, wRated).Equal(stamp), "dry run must not touch")

	st, err := Run(ctx, runOpts(true))
	require.NoError(t, err)
	require.Equal(t, 1, st.BgmWritten)
	touched := workUpdatedAt(t, wRated)
	assert.True(t, touched.After(stamp), "the rating write bumped its host work")
	assert.True(t, workUpdatedAt(t, wBystander).Equal(stamp), "an unrated work stays put")

	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	require.Zero(t, st.BgmWritten)
	require.Equal(t, 1, st.BgmUnchanged)
	assert.True(t, workUpdatedAt(t, wRated).Equal(touched), "a refresh no-op must not drift the watermark")

	require.NoError(t, testDB.Exec(`UPDATE src_bangumi.subject SET score = 8.5 WHERE id = 801`).Error)
	st, err = Run(ctx, runOpts(true))
	require.NoError(t, err)
	require.Equal(t, 1, st.BgmWritten)
	assert.True(t, workUpdatedAt(t, wRated).After(touched), "a changed score re-publishes the work")
}
