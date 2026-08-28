package bgmworkmeta

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
		dbtest.SkipMain("jobs/bgmworkmeta")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/bgmworkmeta", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/bgmworkmeta", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/bgmworkmeta", "catalog seed failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/bgmworkmeta", "src_bangumi schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_tag", "catalog_work_popularity", "catalog_external_ref",
		"catalog_work", "src_bangumi.subject",
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

func mkSubject(t *testing.T, id int64, metaTags, favorite string) {
	t.Helper()
	sub := srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id), NameCN: "",
		InfoboxRaw: "", ParseError: "", Summary: "", Date: "",
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}
	if metaTags != "" {
		sub.MetaTags = []byte(metaTags)
	}
	if favorite != "" {
		sub.Favorite = []byte(favorite)
	}
	require.NoError(t, testDB.Create(&sub).Error)
}

func tagCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_work_tag "+where, args...).Scan(&n).Error)
	return n
}

func popCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_work_popularity "+where, args...).Scan(&n).Error)
	return n
}

func TestBackfillBgmWorkMeta(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	claimed := "galgame_wiki"

	wFull := mkWork(t, reg.galgameMedium, "meta-full", nil)
	mkSubject(t, 301, `["游戏", "  PC  ", "   ", "游戏"]`,
		`{"wish": 5, "done": 12, "doing": 0, "on_hold": 2, "dropped": 1}`)
	mkAnchor(t, wFull, "301", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")

	wOther := mkWork(t, reg.galgameMedium, "meta-other-rule", nil)
	mkSubject(t, 302, `["Galgame"]`, `{"wish": 3, "done": -1, "doing": 4, "dropped": 0, "mystery": 9}`)
	mkAnchor(t, wOther, "302", reg.bangumiSource, model.LinkKindExact, "rule:wiki-bid-typed")

	wEmpty := mkWork(t, reg.galgameMedium, "meta-empty", nil)
	mkSubject(t, 303, `[]`, `{}`)
	mkAnchor(t, wEmpty, "303", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")

	wNull := mkWork(t, reg.galgameMedium, "meta-null", nil)
	mkSubject(t, 304, "", "")
	mkAnchor(t, wNull, "304", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")

	wBad := mkWork(t, reg.galgameMedium, "meta-bad", nil)
	mkSubject(t, 305, `{"name":"游戏"}`, `[1,2,3]`)
	mkAnchor(t, wBad, "305", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")

	wClaimed := mkWork(t, reg.galgameMedium, "meta-claimed", &claimed)
	mkSubject(t, 306, `["游戏"]`, `{"wish": 1}`)
	mkAnchor(t, wClaimed, "306", reg.bangumiSource, model.LinkKindExact, "rule:bgm-title-year")
	wProbable := mkWork(t, reg.galgameMedium, "meta-probable", nil)
	mkSubject(t, 307, `["游戏"]`, `{"wish": 1}`)
	mkAnchor(t, wProbable, "307", reg.bangumiSource, model.LinkKindProbable, "rule:bgm-title-only")

	require.NoError(t, testDB.Create(&model.CatalogWorkTag{
		WorkID: wFull, Name: "游戏", Count: 24, SourceID: reg.bangumiSource,
	}).Error)

	st, err := Run(ctx, Opts{DSN: testDSN})
	require.NoError(t, err)
	assert.Equal(t, 6, st.Candidates, "probable excluded; claimed admitted (T2)")
	assert.Equal(t, 2, st.MetaNoTags, "empty array + NULL (wBad's object is not_array)")
	assert.Equal(t, 1, st.MetaNotArray)
	assert.Equal(t, 1, st.MetaNameBlank)
	assert.Equal(t, 1, st.MetaDup, "the second 游戏 on wFull collapsed")
	assert.Equal(t, 4, st.MetaPlanned, "wFull 2 (游戏, PC) + wOther 1 (Galgame) + wClaimed 1 (游戏)")
	assert.Equal(t, 3, st.MetaDistinct)
	assert.Equal(t, 3, st.FavNoObject, "empty object + NULL + malformed array")
	assert.Equal(t, 1, st.FavUnknownKey, "mystery counted, never guessed")
	assert.Equal(t, 1, st.FavBadValue, "negative done skipped")
	assert.Equal(t, 9, st.FavPlanned, "wFull 5 + wOther 3 + wClaimed 1 (wish)")
	assert.Zero(t, st.MetaWritten+st.MetaConflict+st.FavWritten+st.FavUnchanged+st.Errors)
	assert.EqualValues(t, 1, tagCount(t, ""), "dry run writes nothing (only the pre-seeded folksonomy row)")
	assert.EqualValues(t, 0, popCount(t, ""))
	require.GreaterOrEqual(t, len(st.FavSamples), 5)
	assert.Equal(t, FavSample{WorkID: wFull, SubjectID: 301, Bucket: "wish", Metric: model.PopularityMetricBgmWish, Value: 5}, st.FavSamples[0])
	assert.Equal(t, FavSample{WorkID: wFull, SubjectID: 301, Bucket: "done", Metric: model.PopularityMetricBgmCollect, Value: 12}, st.FavSamples[1],
		"the dump's done key maps to the collect metric")
	assert.Equal(t, FavSample{WorkID: wFull, SubjectID: 301, Bucket: "doing", Metric: model.PopularityMetricBgmDoing, Value: 0}, st.FavSamples[2],
		"a present 0 is a real planned row")

	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 3, st.MetaWritten, "PC + Galgame + wClaimed 游戏 — wFull 游戏 collided with the folksonomy row")
	assert.Equal(t, 1, st.MetaConflict)
	assert.Equal(t, 9, st.FavWritten, "wClaimed's favorite writes too (T2b)")
	assert.Zero(t, st.FavUnchanged+st.Errors)
	assert.EqualValues(t, 4, tagCount(t, ""))
	assert.EqualValues(t, 9, popCount(t, ""))

	var rows []model.CatalogWorkTag
	require.NoError(t, testDB.Where("work_id = ?", wFull).Order("name").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, "PC", rows[0].Name)
	assert.Equal(t, 0, rows[0].Count, "meta-only name lands with count=0")
	assert.Equal(t, "游戏", rows[1].Name)
	assert.Equal(t, 24, rows[1].Count, "folksonomy votes survive the meta-tag collision")

	var pops []model.CatalogWorkPopularity
	require.NoError(t, testDB.Where("work_id = ?", wOther).Order("metric").Find(&pops).Error)
	require.Len(t, pops, 3)
	assert.Equal(t, model.PopularityMetricBgmWish, pops[0].Metric)
	assert.EqualValues(t, 3, pops[0].Value)
	assert.Equal(t, model.PopularityMetricBgmDoing, pops[1].Metric)
	assert.EqualValues(t, 4, pops[1].Value)
	assert.Equal(t, model.PopularityMetricBgmDropped, pops[2].Metric)
	assert.EqualValues(t, 0, pops[2].Value, "present 0 is a real row")
	assert.EqualValues(t, 1, tagCount(t, "WHERE work_id = ?", wClaimed), "T2: claimed meta tag materialises")
	assert.EqualValues(t, 1, popCount(t, "WHERE work_id = ?", wClaimed), "T2b: claimed favorite materialises")
	assert.EqualValues(t, 0, tagCount(t, "WHERE work_id = ?", wProbable))
	assert.EqualValues(t, 0, popCount(t, "WHERE work_id = ?", wProbable))

	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.MetaWritten+st.FavWritten+st.Errors, "second pass writes zero")
	assert.Equal(t, 4, st.MetaConflict, "PC + wFull 游戏 + Galgame + wClaimed 游戏 all exist")
	assert.Equal(t, 9, st.FavUnchanged)
	assert.EqualValues(t, 4, tagCount(t, ""))
	assert.EqualValues(t, 9, popCount(t, ""))

	require.NoError(t, testDB.Exec(
		`UPDATE catalog_work_popularity SET value = 999 WHERE work_id = ? AND metric = ?`,
		wFull, model.PopularityMetricBgmCollect).Error)
	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 1, st.FavWritten, "only the mutated row updates")
	assert.Equal(t, 8, st.FavUnchanged)
	var healed int64
	require.NoError(t, testDB.Raw(
		`SELECT value FROM catalog_work_popularity WHERE work_id = ? AND metric = ?`,
		wFull, model.PopularityMetricBgmCollect).Scan(&healed).Error)
	assert.EqualValues(t, 12, healed, "value healed back to the subject's shelf count")
	assert.Zero(t, st.MetaWritten)
	assert.Equal(t, 4, st.MetaConflict)
}

func TestWriterClaimAdmissionAndDSNRequired(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	claimed := "galgame_wiki"
	wClaimed := mkWork(t, reg.galgameMedium, "claimed-direct", &claimed)
	wBody := mkWork(t, reg.galgameMedium, "bodyless-direct", nil)

	w := &writer{db: testDB, stats: &Stats{}}
	w.writeTag(ctx, tagRow{WorkID: wClaimed, SourceID: reg.bangumiSource, Name: "游戏"}, true)
	assert.Equal(t, 1, w.stats.MetaWritten, "T2: Field A materialises the claimed meta tag")
	w.writeFavorite(ctx, favRow{WorkID: wClaimed, SourceID: reg.bangumiSource, Metric: model.PopularityMetricBgmWish, Value: 3}, true)
	assert.Equal(t, 1, w.stats.FavWritten, "T2b: Field B materialises the claimed favorite")
	assert.EqualValues(t, 1, tagCount(t, ""), "the Field A meta tag landed")
	assert.EqualValues(t, 1, popCount(t, ""), "the claimed favorite row landed")

	w.writeTag(ctx, tagRow{WorkID: wBody, SourceID: reg.bangumiSource, Name: "游戏"}, true)
	assert.Equal(t, 2, w.stats.MetaWritten)
	w.writeTag(ctx, tagRow{WorkID: wBody, SourceID: reg.bangumiSource, Name: "游戏"}, true)
	assert.Equal(t, 1, w.stats.MetaConflict, "ON CONFLICT refuses the duplicate")
	w.writeFavorite(ctx, favRow{WorkID: wBody, SourceID: reg.bangumiSource, Metric: model.PopularityMetricBgmWish, Value: 3}, true)
	assert.Equal(t, 2, w.stats.FavWritten)
	w.writeFavorite(ctx, favRow{WorkID: wBody, SourceID: reg.bangumiSource, Metric: model.PopularityMetricBgmWish, Value: 3}, true)
	assert.Equal(t, 1, w.stats.FavUnchanged, "same value is a change-detected no-op")
	w.writeFavorite(ctx, favRow{WorkID: wBody, SourceID: reg.bangumiSource, Metric: model.PopularityMetricBgmWish, Value: 4}, true)
	assert.Equal(t, 3, w.stats.FavWritten, "changed value updates in place")
	assert.EqualValues(t, 2, popCount(t, ""))

	_, err = Run(ctx, Opts{})
	require.Error(t, err)
}
