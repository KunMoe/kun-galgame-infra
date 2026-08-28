package entityintros

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

const egSchema = "entityintros_eg"

var (
	testDB    *gorm.DB
	testDSN   string
	egTestDSN string
	egDDL     = []string{
		`CREATE SCHEMA IF NOT EXISTS ` + egSchema,
		`CREATE TABLE IF NOT EXISTS ` + egSchema + `.appearances (
			raw jsonb NOT NULL,
			pk text GENERATED ALWAYS AS ((raw->>'game') || '/' || (raw->>'character') || '/' || (raw->>'stage')) STORED,
			game integer GENERATED ALWAYS AS ((raw->>'game')::integer) STORED,
			character_id integer GENERATED ALWAYS AS ((raw->>'character')::integer) STORED,
			synced_at timestamptz NOT NULL DEFAULT now())`,
	}
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/entityintros")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/entityintros", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/entityintros", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/entityintros", "catalog seed failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/entityintros", "src_bangumi schema failed: %v", err)
	}
	if err := srcv.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/entityintros", "src_vndb schema failed: %v", err)
	}
	for _, stmt := range egDDL {
		if err := db.Exec(stmt).Error; err != nil {
			dbtest.SkipMainf("jobs/entityintros", "erogamescape stand-in failed: %v", err)
		}
	}
	testDB = db
	egTestDSN = testDSN + " search_path=" + egSchema
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_character_intro", "catalog_person_intro", "catalog_external_ref",
		"catalog_work_character", "catalog_work",
		"catalog_character", "catalog_person",
		"src_bangumi.character", "src_bangumi.person", "src_vndb.chars",
		egSchema + ".appearances",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkCharacter(t *testing.T, name string) int64 {
	t.Helper()
	c := model.CatalogCharacter{DisplayName: name}
	require.NoError(t, testDB.Create(&c).Error)
	return c.ID
}

func mkPerson(t *testing.T, name string) int64 {
	t.Helper()
	p := model.CatalogPerson{DisplayName: name}
	require.NoError(t, testDB.Create(&p).Error)
	return p.ID
}

func mkAnchor(t *testing.T, entityType int16, entityID int64, source int16, externalID string, kind int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: entityType, EntityID: entityID, SourceID: source,
		ExternalID: externalID, LinkKind: kind, MatchedBy: "rule:test",
	}).Error)
}

func mkSrcCharacter(t *testing.T, id int64, summary string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Character{
		ID: id, Role: 1, Name: fmt.Sprintf("char-%d", id), InfoboxRaw: "", ParseError: "",
		Summary: summary, ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func mkSrcPerson(t *testing.T, id int64, summary string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Person{
		ID: id, Name: fmt.Sprintf("person-%d", id), Type: 1, InfoboxRaw: "", ParseError: "",
		Summary: summary, ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func mkSrcVNDBChar(t *testing.T, id, description string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcv.Char{
		ID: id, Image: "", Sex: "", Gender: "", Main: "", MainSpoil: 0,
		Description: description, IngestedAt: time.Now(),
	}).Error)
}

func charIntroCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw("SELECT count(*) FROM catalog_character_intro "+where, args...).Scan(&n).Error)
	return n
}

func TestFillMissingAllLanes(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)
	var userSrc int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'user'`).Scan(&userSrc).Error)
	require.NotZero(t, userSrc)

	chZh := mkCharacter(t, "bgm-zh")
	chJaDup := mkCharacter(t, "bgm-ja-dup")
	chJaCRLF := mkCharacter(t, "bgm-ja-crlf")
	chBlank := mkCharacter(t, "bgm-blank")
	chSpoiler := mkCharacter(t, "vndb-spoiler")
	chAllSpoil := mkCharacter(t, "vndb-all-spoiler")
	chBoth := mkCharacter(t, "both-sources")
	chProbable := mkCharacter(t, "vndb-probable")
	chDeleted := mkCharacter(t, "deleted")

	mkSrcCharacter(t, 101, "少女是学园的学生会长。")
	mkSrcCharacter(t, 102, "ヒロインのひとり。")
	mkSrcCharacter(t, 103, "一行目です。\r\n二行目です。")
	mkSrcCharacter(t, 104, " \r\n ")
	mkSrcCharacter(t, 105, "主人公の幼馴染。")
	mkSrcVNDBChar(t, "c201", "A cheerful student.\n\n[spoiler]She is the final boss.[/spoiler]\n\nLoves cats.")
	mkSrcVNDBChar(t, "c202", "[spoiler]entirely spoiler[/spoiler]")
	mkSrcVNDBChar(t, "c203", "The protagonist's childhood friend.")
	mkSrcVNDBChar(t, "c204", "Probable-tier description.")
	mkSrcVNDBChar(t, "c205", "Deleted character description.")

	mkAnchor(t, model.EntityTypeCharacter, chZh, reg.bangumiSource, "101", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chJaDup, reg.bangumiSource, "102", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chJaCRLF, reg.bangumiSource, "103", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chBlank, reg.bangumiSource, "104", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chSpoiler, reg.vndbSource, "c201", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chAllSpoil, reg.vndbSource, "c202", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chBoth, reg.bangumiSource, "105", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chBoth, reg.vndbSource, "c203", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, chProbable, reg.vndbSource, "c204", model.LinkKindProbable)
	mkAnchor(t, model.EntityTypeCharacter, chDeleted, reg.vndbSource, "c205", model.LinkKindExact)
	require.NoError(t, testDB.Delete(&model.CatalogCharacter{ID: chDeleted}).Error)

	require.NoError(t, testDB.Create(&model.CatalogCharacterIntro{
		CharacterID: chJaDup, Lang: "ja", Intro: "既にある日本語紹介。", SourceID: userSrc,
	}).Error)

	pZh := mkPerson(t, "person-zh")
	pJaDup := mkPerson(t, "person-ja-dup")
	mkSrcPerson(t, 301, "中国出身的插画家。")
	mkSrcPerson(t, 302, "日本のイラストレーター。")
	mkAnchor(t, model.EntityTypePerson, pZh, reg.bangumiSource, "301", model.LinkKindExact)
	mkAnchor(t, model.EntityTypePerson, pJaDup, reg.bangumiSource, "302", model.LinkKindExact)
	require.NoError(t, testDB.Create(&model.CatalogPersonIntro{
		PersonID: pJaDup, Lang: "ja", Intro: "既存の人物紹介。", SourceID: userSrc,
	}).Error)

	wBgm := mkWork(t, "host-bgm")
	wVndb := mkWork(t, "host-vndb")
	wBoth := mkWork(t, "host-both")
	wQuiet := mkWork(t, "host-quiet")
	wGone := mkWork(t, "host-deleted")
	mkRosterEdge(t, wBgm, chZh)
	mkRosterEdge(t, wVndb, chSpoiler)
	mkRosterEdge(t, wBoth, chBoth)
	mkRosterEdge(t, wQuiet, chJaDup)
	mkRosterEdge(t, wQuiet, chBlank)
	mkRosterEdge(t, wGone, chJaCRLF)
	require.NoError(t, testDB.Delete(&model.CatalogWork{ID: wGone}).Error)

	st, err := Run(ctx, Opts{DSN: testDSN, EGDSN: egTestDSN})
	require.NoError(t, err)
	assert.Equal(t, 5, st.CharBangumi.Candidates, "chZh chJaDup chJaCRLF chBlank chBoth")
	assert.Equal(t, 1, st.CharBangumi.ZhNew, "chZh")
	assert.Equal(t, 2, st.CharBangumi.JaNew, "chJaCRLF + chBoth")
	assert.Equal(t, 1, st.CharBangumi.SkipDupLang, "chJaDup")
	assert.Equal(t, 1, st.CharBangumi.NoText, "chBlank")

	assert.Equal(t, 3, st.CharVNDB.Candidates, "chSpoiler chAllSpoil chBoth; probable + deleted excluded in SQL")
	assert.Equal(t, 2, st.CharVNDB.EnNew, "chSpoiler + chBoth")
	assert.Equal(t, 1, st.CharVNDB.NoText, "chAllSpoil emptied by the spoiler strip")
	assert.Equal(t, 2, st.CharVNDB.SpoilerStripped, "chSpoiler + chAllSpoil had spans removed")

	assert.Equal(t, 2, st.PersonBangumi.Candidates)
	assert.Equal(t, 1, st.PersonBangumi.ZhNew, "pZh")
	assert.Equal(t, 1, st.PersonBangumi.SkipDupLang, "pJaDup")

	assert.Zero(t, st.CharBangumi.JaWritten+st.CharBangumi.ZhWritten+st.CharVNDB.EnWritten+
		st.PersonBangumi.JaWritten+st.PersonBangumi.ZhWritten, "dry run writes nothing")
	assert.EqualValues(t, 1, charIntroCount(t, ""), "only the fixture row exists")
	assert.Zero(t, st.CharBangumi.Touched+st.CharVNDB.Touched+st.PersonBangumi.Touched,
		"dry run touches nothing")
	for _, w := range []int64{wBgm, wVndb, wBoth, wQuiet} {
		assert.Equal(t, backdated.UTC(), workUpdatedAt(t, w).UTC(), "dry run moves no watermark")
	}

	st, err = Run(ctx, Opts{DSN: testDSN, EGDSN: egTestDSN, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 1, st.CharBangumi.ZhWritten)
	assert.Equal(t, 2, st.CharBangumi.JaWritten)
	assert.Equal(t, 2, st.CharVNDB.EnWritten)
	assert.Equal(t, 1, st.PersonBangumi.ZhWritten)
	assert.Zero(t, st.CharBangumi.Errors+st.CharVNDB.Errors+st.PersonBangumi.Errors)
	assert.Zero(t, st.CharBangumi.Conflict+st.CharVNDB.Conflict+st.PersonBangumi.Conflict)

	assert.Equal(t, 2, st.CharBangumi.Touched, "wBgm (chZh) + wBoth (chBoth ja); wGone is soft-deleted")
	assert.Equal(t, 2, st.CharVNDB.Touched, "wVndb (chSpoiler) + wBoth (chBoth en)")
	assert.Zero(t, st.PersonBangumi.Touched, "persons are not on the work read face — no touch path")
	bumped := map[int64]time.Time{}
	for _, w := range []int64{wBgm, wVndb, wBoth} {
		bumped[w] = workUpdatedAt(t, w)
		assert.True(t, bumped[w].After(backdated), "host work of a written character is bumped")
	}
	assert.Equal(t, backdated.UTC(), workUpdatedAt(t, wQuiet).UTC(), "dup-lang skip and no_text bump nothing")
	assert.Equal(t, backdated.UTC(), workUpdatedAt(t, wGone).UTC(), "a soft-deleted host work is never bumped")

	var spoilRow model.CatalogCharacterIntro
	require.NoError(t, testDB.Where("character_id = ? AND source_id = ?", chSpoiler, reg.vndbSource).First(&spoilRow).Error)
	assert.Equal(t, "en", spoilRow.Lang)
	assert.Equal(t, "A cheerful student.\n\nLoves cats.", spoilRow.Intro)
	assert.NotContains(t, spoilRow.Intro, "final boss")

	var crlfRow model.CatalogCharacterIntro
	require.NoError(t, testDB.Where("character_id = ?", chJaCRLF).First(&crlfRow).Error)
	assert.Equal(t, "ja", crlfRow.Lang)
	assert.Equal(t, "一行目です。\n二行目です。", crlfRow.Intro)

	assert.EqualValues(t, 2, charIntroCount(t, "WHERE character_id = ?", chBoth), "ja(bangumi) + en(vndb)")
	assert.EqualValues(t, 0, charIntroCount(t, "WHERE character_id = ?", chDeleted), "soft-deleted never materialises")
	assert.EqualValues(t, 0, charIntroCount(t, "WHERE character_id = ?", chProbable), "probable tier never materialises")

	var pRow model.CatalogPersonIntro
	require.NoError(t, testDB.Where("person_id = ?", pZh).First(&pRow).Error)
	assert.Equal(t, "zh-Hans", pRow.Lang)

	st, err = Run(ctx, Opts{DSN: testDSN, EGDSN: egTestDSN, Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st.CharBangumi.JaWritten+st.CharBangumi.ZhWritten+st.CharVNDB.EnWritten+
		st.PersonBangumi.JaWritten+st.PersonBangumi.ZhWritten+
		st.CharBangumi.Errors+st.CharVNDB.Errors+st.PersonBangumi.Errors, "second pass writes zero")
	assert.Equal(t, 4, st.CharBangumi.SkipDupLang, "the three written langs now skip; chJaDup still dup")
	assert.EqualValues(t, 6, charIntroCount(t, ""), "row count unchanged: 1 fixture + 5 writes")
	assert.Zero(t, st.CharBangumi.Touched+st.CharVNDB.Touched, "no writes, so no watermark moves")
	for w, ts := range bumped {
		assert.Equal(t, ts.UTC(), workUpdatedAt(t, w).UTC(), "watermark does not drift on a re-run")
	}
}

func TestOnlyLaneAndConflictBackstop(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	_, err = Run(ctx, Opts{DSN: testDSN, Only: "bogus"})
	require.Error(t, err, "unknown lane is refused")

	ch := mkCharacter(t, "only-vndb")
	mkSrcVNDBChar(t, "c900", "Only-lane description.")
	mkSrcCharacter(t, 901, "bgm 側の紹介。")
	mkAnchor(t, model.EntityTypeCharacter, ch, reg.vndbSource, "c900", model.LinkKindExact)
	mkAnchor(t, model.EntityTypeCharacter, ch, reg.bangumiSource, "901", model.LinkKindExact)

	st, err := Run(ctx, Opts{DSN: testDSN, Apply: true, Only: LaneCharVNDB})
	require.NoError(t, err)
	assert.Equal(t, 1, st.CharVNDB.EnWritten)
	assert.Zero(t, st.CharBangumi.Candidates, "bgm lane not run under --only char-vndb")
	assert.Zero(t, st.PersonBangumi.Candidates)
	assert.EqualValues(t, 1, charIntroCount(t, ""))

	r := &laneRunner{db: testDB, sourceID: reg.vndbSource, exist: map[int64]map[string]bool{},
		stats: &LaneStats{}, vndb: true, lang: langEn, insert: insertCharacterIntro}
	r.enrich(ctx, candidate{EntityID: ch, ExternalID: "c900", Text: "Only-lane description."}, true)
	assert.Equal(t, 1, r.stats.Conflict, "ON CONFLICT refuses the duplicate")
	assert.Equal(t, 0, r.stats.EnWritten)
	assert.EqualValues(t, 1, charIntroCount(t, ""), "still exactly one row")
}
