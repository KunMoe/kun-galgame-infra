package bgmsummaries

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
		dbtest.SkipMain("jobs/bgmsummaries")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/bgmsummaries", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/bgmsummaries", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/bgmsummaries", "catalog seed failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/bgmsummaries", "src_bangumi schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_intro", "catalog_external_ref", "catalog_work", "src_bangumi.subject",
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

func mkAnchor(t *testing.T, workID, subjectID int64, source int16, kind int16, matchedBy string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: source,
		ExternalID: strconv.FormatInt(subjectID, 10), LinkKind: kind, MatchedBy: matchedBy,
	}).Error)
}

func mkSubject(t *testing.T, id int64, summary string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Subject{
		ID: id, Type: 4, Name: fmt.Sprintf("subject-%d", id), NameCN: "",
		InfoboxRaw: "", ParseError: "", Summary: summary, Date: "",
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func mkIntro(t *testing.T, workID int64, lang, intro string, source int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogWorkIntro{
		WorkID: workID, Lang: lang, Intro: intro, SourceID: source,
	}).Error)
}

func introCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_work_intro "+where, args...).Scan(&n).Error)
	return n
}

func TestFillMissingLanguage(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)
	var dlsite int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'dlsite'`).Scan(&dlsite).Error)
	require.NotZero(t, dlsite)

	claimed := "kungal"
	wZh := mkWork(t, reg.galgameMedium, "has-ja-gets-zh", nil)
	wJaDup := mkWork(t, reg.galgameMedium, "has-ja-dup-ja", nil)
	wJaFill := mkWork(t, reg.galgameMedium, "no-intro-gets-ja", nil)
	wEmpty := mkWork(t, reg.galgameMedium, "blank-summary", nil)
	wClaimed := mkWork(t, reg.galgameMedium, "claimed", &claimed)
	wProbable := mkWork(t, reg.galgameMedium, "probable-anchor", nil)

	zhSummary := "两位新人冒险者发现自己被困孤岛。"
	jaDupSummary := "ふたりの冒険者が孤島に流れ着く。"
	jaCRLFSummary := "一行目です。\r\n二行目です。"
	mkSubject(t, 1001, zhSummary)
	mkSubject(t, 1002, jaDupSummary)
	mkSubject(t, 1003, jaCRLFSummary)
	mkSubject(t, 1004, " \r\n ")
	mkSubject(t, 1005, "claimed のあらすじ")
	mkSubject(t, 1006, "probable のあらすじ")

	mkAnchor(t, wZh, 1001, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wJaDup, 1002, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wJaFill, 1003, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wEmpty, 1004, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wClaimed, 1005, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wProbable, 1006, reg.bangumiSource, model.LinkKindProbable, "rule:bgm-title-only")

	mkIntro(t, wZh, "ja", "DLsite の紹介文。", dlsite)
	mkIntro(t, wJaDup, "ja", "既にある日本語紹介。", dlsite)

	st, err := Run(ctx, Opts{DSN: testDSN, Population: PopulationBodyless})
	require.NoError(t, err)
	assert.Equal(t, 4, st.Candidates, "claimed (other population) + probable are excluded in SQL")
	assert.Equal(t, 1, st.ZhNew, "wZh: zh summary, no zh intro yet")
	assert.Equal(t, 1, st.JaFill, "wJaFill: ja summary, no intro at all")
	assert.Equal(t, 1, st.SkipDupLang, "wJaDup: ja summary duplicates the DLsite ja intro")
	assert.Equal(t, 1, st.NoSummary)
	assert.Zero(t, st.ZhWritten+st.JaWritten+st.Conflict+st.Errors)
	assert.EqualValues(t, 2, introCount(t, ""), "dry run writes nothing (the two fixtures only)")
	require.Len(t, st.ZhSamples, 1)
	assert.Equal(t, wZh, st.ZhSamples[0].WorkID)
	assert.Equal(t, "zh-Hans", st.ZhSamples[0].Lang)

	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true, Population: PopulationBodyless})
	require.NoError(t, err)
	assert.Equal(t, 1, st.ZhWritten)
	assert.Equal(t, 1, st.JaWritten)
	assert.Equal(t, 0, st.Conflict)
	assert.Equal(t, 0, st.Errors)

	var rowZh, rowJa model.CatalogWorkIntro
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wZh, reg.bangumiSource).First(&rowZh).Error)
	assert.Equal(t, "zh-Hans", rowZh.Lang)
	assert.Equal(t, zhSummary, rowZh.Intro, "text verbatim")
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wJaFill, reg.bangumiSource).First(&rowJa).Error)
	assert.Equal(t, "ja", rowJa.Lang)
	assert.Equal(t, "一行目です。\n二行目です。", rowJa.Intro, "CRLF normalized to LF")
	assert.EqualValues(t, 0, introCount(t, "WHERE work_id = ? AND source_id = ?", wJaDup, reg.bangumiSource),
		"duplicate-language summary never lands")
	assert.EqualValues(t, 0, introCount(t, "WHERE work_id = ?", wClaimed), "the other population stays untouched")
	assert.EqualValues(t, 4, introCount(t, ""), "2 fixtures + 2 writes")

	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true, Population: PopulationBodyless})
	require.NoError(t, err)
	assert.Zero(t, st.ZhWritten+st.JaWritten+st.Errors, "second pass writes zero")
	assert.Equal(t, 3, st.SkipDupLang, "wZh + wJaFill now have their langs; wJaDup still dup")
	assert.EqualValues(t, 4, introCount(t, ""), "row count unchanged")
}

func TestNonTitleYearExactAnchorEntersCandidates(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	wGated := mkWork(t, reg.galgameMedium, "gated-anchor", nil)
	mkSubject(t, 3001, "ゲートされたあらすじ。")
	mkAnchor(t, wGated, 3001, reg.bangumiSource, model.LinkKindExact, "rule:bgm-type4-gated")

	wXmedia := mkWork(t, reg.galgameMedium, "xmedia-anchor", nil)
	mkSubject(t, 3002, "クロスメディアのあらすじ。")
	mkAnchor(t, wXmedia, 3002, reg.bangumiSource, model.LinkKindExact, "rule:bangumi-xmedia-import")

	wProbable := mkWork(t, reg.galgameMedium, "probable-anchor", nil)
	mkSubject(t, 3003, "確度の低いあらすじ。")
	mkAnchor(t, wProbable, 3003, reg.bangumiSource, model.LinkKindProbable, "rule:bgm-title-only")

	st, err := Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 2, st.Candidates, "both non-title-year exact anchors are candidates; probable stays out")
	assert.EqualValues(t, 1, introCount(t, "WHERE work_id = ?", wGated), "gated-rule work gets its intro")
	assert.EqualValues(t, 1, introCount(t, "WHERE work_id = ?", wXmedia), "xmedia-rule work gets its intro")
	assert.EqualValues(t, 0, introCount(t, "WHERE work_id = ?", wProbable), "probable work stays out")
}

func TestClaimedPopulation(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)
	var curated int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'curated'`).Scan(&curated).Error)
	require.NotZero(t, curated)

	site := "kungal"
	wBlank := mkWork(t, reg.galgameMedium, "published-en-only", &site)
	wZh := mkWork(t, reg.galgameMedium, "published-gets-zh", &site)
	wHasJa := mkWork(t, reg.galgameMedium, "published-has-ja", &site)
	wBody := mkWork(t, reg.galgameMedium, "bodyless-peer", nil)

	mkIntro(t, wBlank, "en", "An English blurb carried over from the wiki.", curated)
	mkIntro(t, wZh, "en", "Another English blurb.", curated)
	mkIntro(t, wHasJa, "ja", "既にある日本語紹介。", curated)

	mkSubject(t, 4001, "ふたりの少女が出会う物語。")
	mkSubject(t, 4002, "两位新人冒险者发现自己被困孤岛。")
	mkSubject(t, 4003, "重複する日本語のあらすじ。")
	mkSubject(t, 4004, "ボディレスのあらすじ。")
	mkAnchor(t, wBlank, 4001, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wZh, 4002, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wHasJa, 4003, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)
	mkAnchor(t, wBody, 4004, reg.bangumiSource, model.LinkKindExact, ruleTitleYear)

	st, err := Run(ctx, Opts{DSN: testDSN, Apply: true, Population: PopulationClaimed})
	require.NoError(t, err)
	assert.Equal(t, 3, st.Candidates, "the bodyless peer is out of this population")
	assert.Equal(t, 1, st.JaWritten)
	assert.Equal(t, 1, st.ZhWritten)
	assert.Equal(t, 2, st.ClaimedWritten, "both writes landed on published works")
	assert.Equal(t, 1, st.SkipDupLang, "wHasJa already has ja")
	assert.Zero(t, st.Errors)

	assert.EqualValues(t, 1, introCount(t, "WHERE work_id = ? AND lang = 'ja'", wBlank))
	assert.EqualValues(t, 1, introCount(t, "WHERE work_id = ? AND lang = 'zh-Hans'", wZh))
	assert.EqualValues(t, 1, introCount(t, "WHERE work_id = ? AND source_id = ? AND lang = 'en'", wBlank, curated),
		"the work's own curated English text is untouched")
	assert.EqualValues(t, 0, introCount(t, "WHERE work_id = ?", wBody), "the other population stays untouched")

	var row model.CatalogWorkIntro
	require.NoError(t, testDB.Where("work_id = ? AND lang = 'ja'", wBlank).First(&row).Error)
	assert.EqualValues(t, 0, row.Provenance, "an ingested upstream blurb is a source row, never machine")

	st, err = Run(ctx, Opts{DSN: testDSN, Apply: true, Population: PopulationClaimed})
	require.NoError(t, err)
	assert.Zero(t, st.ZhWritten+st.JaWritten+st.Errors, "second pass writes zero")
}

func TestNoLangAbstentionAndConflictBackstop(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	wLatin := mkWork(t, reg.galgameMedium, "latin-summary", nil)
	wBody := mkWork(t, reg.galgameMedium, "bodyless-direct", nil)

	r := &runner{db: testDB, sourceID: reg.bangumiSource, exist: map[int64]map[string]bool{}, stats: &Stats{}}
	r.enrich(ctx, candidate{WorkID: wLatin, SubjectID: 2001, Summary: "A short doujin RPG about friendship."}, true)
	assert.Equal(t, 1, r.stats.NoLang)
	assert.Zero(t, r.stats.JaFill+r.stats.JaWritten+r.stats.ZhNew+r.stats.ZhWritten)
	assert.EqualValues(t, 0, introCount(t, ""), "CJK-free text is never filed under a language tag")

	r.enrich(ctx, candidate{WorkID: wBody, SubjectID: 2002, Site: nil, Summary: "あらすじ"}, true)
	assert.Equal(t, 1, r.stats.JaWritten)
	rStale := &runner{db: testDB, sourceID: reg.bangumiSource, exist: map[int64]map[string]bool{}, stats: &Stats{}}
	rStale.enrich(ctx, candidate{WorkID: wBody, SubjectID: 2002, Site: nil, Summary: "あらすじ"}, true)
	assert.Equal(t, 1, rStale.stats.Conflict, "ON CONFLICT refuses the duplicate")
	assert.Equal(t, 0, rStale.stats.JaWritten)
	assert.EqualValues(t, 1, introCount(t, ""), "still exactly one row")
}
