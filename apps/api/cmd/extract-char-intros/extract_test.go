package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/seed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "SKIP: TEST_DATABASE_DSN is unset")
		os.Exit(0)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
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
	testDB = db
	os.Exit(m.Run())
}

func TestVerbatimGate(t *testing.T) {
	intro := "序章的舞台是学园都市。\n\n「沙耶」\n主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。\n喜欢做点心。\n\n「玲」\n转校生,沉默寡言。"
	cases := []struct {
		name    string
		passage string
		want    bool
	}{
		{"exact line", "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。", true},
		{"merged adjacent lines", "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。\n喜欢做点心。", true},
		{"stripped bullet and punctuation", "主人公的青梅竹马、性格开朗", true},
		{"paraphrase", "从小一起长大的女孩,活泼爱笑。", false},
		{"invented content", "主人公的青梅竹马,性格开朗,其实是魔法使。", false},
		{"whole-intro copy", intro, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, verbatim(intro, c.passage), c.name)
	}
}

func TestNameAppears(t *testing.T) {
	intro := "与身为恶魔般的学生会长的月森鈴一起。\n\n「沙耶」\n主人公的青梅竹马。\n转校生香坂 玲奈也来了。"
	cases := []struct {
		name   string
		target rosterChar
		want   bool
	}{
		{"heading style full name", rosterChar{Name: "沙耶"}, true},
		{"spaced name matches unspaced text", rosterChar{Name: "香坂 玲奈"}, true},
		{"given-name segment alone", rosterChar{Name: "如月 玲奈"}, true},
		{"surname alone is not a match", rosterChar{Name: "月森 玲子"}, false},
		{"zh alias rescues a missing name", rosterChar{Name: "Saya", ZhName: "沙耶"}, true},
		{"absent everywhere", rosterChar{Name: "空蝉 譲二"}, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, nameAppears(intro, c.target), c.name)
	}
}

func TestParseExtraction(t *testing.T) {
	got, err := parseExtraction("```json\n{\"沙耶\": \"青梅竹马。\"}\n```")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"沙耶": "青梅竹马。"}, got)

	_, err = parseExtraction("[1,2]")
	assert.Error(t, err, "a non-object payload must be refused")

	got, err = parseExtraction("{}")
	require.NoError(t, err)
	assert.Empty(t, got)
}

type fakeExtractor struct {
	out map[string]string
}

func (f fakeExtractor) ExtractBatch(_ context.Context, batch []candidateWork) []extraction {
	out := make([]extraction, len(batch))
	for i := range batch {
		out[i] = extraction{Found: f.out, Model: "glm-5.2"}
	}
	return out
}

func seedWorkWithIntro(t *testing.T, intro string) int64 {
	t.Helper()
	var workID int64
	require.NoError(t, testDB.Raw(`INSERT INTO catalog_work
		(medium_id, olang, display_name, content_rating, status, display_nsfw,
		 extra, field_provenance, created_at, updated_at)
		VALUES (1, 'ja', 'テスト', 0, 0, false, '{}', '{}', now(), now()) RETURNING id`).Scan(&workID).Error)
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_work_intro
		(work_id, lang, intro, source_id, provenance, created_at, updated_at)
		VALUES (?, 'zh-Hans', ?, 12, 1, now(), now())`, workID, intro).Error)
	return workID
}

func seedRosterChar(t *testing.T, workID int64, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, testDB.Raw(`INSERT INTO catalog_character
		(display_name, lang, description, extra, field_provenance, created_at, updated_at)
		VALUES (?, 'ja', '', '{}', '{}', now(), now()) RETURNING id`, name).Scan(&id).Error)
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_work_character
		(work_id, character_id, kind, spoiler, matched_by, created_at, updated_at)
		VALUES (?, ?, 0, 0, 'test', now(), now())`, workID, id).Error)
	return id
}

var longIntro = "序章的舞台是学园都市。" + strings.Repeat("这个故事讲述了少年与少女们在学园中相遇、离别、再会的经过。", 8) +
	"\n\n「沙耶」\n主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。\n\n「玲」\n转校生,沉默寡言。"

func TestRunExtractsOnlyMissingAndVerbatim(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	saya := seedRosterChar(t, workID, "沙耶")
	rei := seedRosterChar(t, workID, "玲")
	// 玲 already has a zh intro — must not even be offered, let alone written.
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_character_intro
		(character_id, lang, intro, source_id, provenance, created_at, updated_at)
		VALUES (?, 'zh-Hans', '既有介绍', 3, 1, now(), now())`, rei).Error)

	cands, err := loadCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	require.Len(t, cands, 1)
	require.Len(t, cands[0].Roster, 1, "only the intro-less character is a target")
	assert.Equal(t, saya, cands[0].Roster[0].CharacterID)

	ex := fakeExtractor{out: map[string]string{
		"沙耶": "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。",
		"玲":  "转校生,沉默寡言。", // not in roster → unmatched, dropped
	}}
	require.NoError(t, run(context.Background(), testDB, ex, nil, opts{Apply: true}))

	var rows []struct {
		Intro      string `gorm:"column:intro"`
		SourceID   int16  `gorm:"column:source_id"`
		Provenance int16  `gorm:"column:provenance"`
		MTModel    string `gorm:"column:mt_model"`
	}
	require.NoError(t, testDB.Raw(`SELECT intro, source_id, provenance, mt_model
		FROM catalog_character_intro WHERE character_id = ?`, saya).Scan(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。", rows[0].Intro)
	assert.EqualValues(t, sourceDerived, rows[0].SourceID)
	assert.EqualValues(t, 1, rows[0].Provenance)
	assert.Equal(t, "glm-5.2", rows[0].MTModel)

	var reiIntro string
	require.NoError(t, testDB.Raw(`SELECT intro FROM catalog_character_intro
		WHERE character_id = ? AND source_id = 3`, rei).Scan(&reiIntro).Error)
	assert.Equal(t, "既有介绍", reiIntro, "the existing row is untouched")
}

func TestPanelVerdictFoldsOnDirection(t *testing.T) {
	cases := []struct {
		name  string
		votes []panelVote
		want  bool
	}{
		{"unanimous challenger", []panelVote{voteChallenger, voteChallenger, voteChallenger}, true},
		{"two challenger one equal", []panelVote{voteChallenger, voteEqual, voteChallenger}, true},
		{"a single challenger vote is not enough", []panelVote{voteChallenger, voteEqual, voteEqual}, false},
		{"any incumbent vote keeps the incumbent", []panelVote{voteChallenger, voteChallenger, voteIncumbent}, false},
		{"all equal keeps the incumbent", []panelVote{voteEqual, voteEqual, voteEqual}, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, panelVerdict(c.votes), c.name)
	}
}

type fakeJudge struct {
	votes []panelVote
	calls int
}

func (f *fakeJudge) CompareBatch(_ context.Context, batch []comparison) []comparisonResult {
	out := make([]comparisonResult, len(batch))
	for i := range batch {
		out[i] = comparisonResult{Vote: f.votes[f.calls%len(f.votes)]}
		f.calls++
	}
	return out
}

func seedPanelFixture(t *testing.T) (workID, saya, rei int64) {
	t.Helper()
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID = seedWorkWithIntro(t, longIntro)
	saya = seedRosterChar(t, workID, "沙耶")
	rei = seedRosterChar(t, workID, "玲")
	// 沙耶 holds a translated machine row → panel target.
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_character_intro
		(character_id, lang, intro, source_id, provenance, created_at, updated_at)
		VALUES (?, 'zh-Hans', '既有机翻介绍。', 3, 1, now(), now())`, saya).Error)
	// 玲 holds a SOURCE row → machine output never competes, excluded.
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_character_intro
		(character_id, lang, intro, source_id, provenance, created_at, updated_at)
		VALUES (?, 'zh-Hans', '源文介绍。', 3, 0, now(), now())`, rei).Error)
	return workID, saya, rei
}

func TestRunPanelAdoptsUnanimousChallenger(t *testing.T) {
	_, saya, _ := seedPanelFixture(t)

	cands, err := loadPanelCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	require.Len(t, cands, 1)
	require.Len(t, cands[0].Roster, 1, "only the translated-machine-row holder is a target")
	assert.Equal(t, saya, cands[0].Roster[0].CharacterID)
	assert.Equal(t, "既有机翻介绍。", cands[0].Roster[0].Incumbent)

	ex := fakeExtractor{out: map[string]string{"沙耶": "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。"}}
	judge := &fakeJudge{votes: []panelVote{voteChallenger}}
	require.NoError(t, run(context.Background(), testDB, ex, judge, opts{Apply: true, Panel: true}))
	assert.Equal(t, panelVotes, judge.calls)

	var rows []struct {
		Intro    string `gorm:"column:intro"`
		SourceID int16  `gorm:"column:source_id"`
	}
	require.NoError(t, testDB.Raw(`SELECT intro, source_id FROM catalog_character_intro
		WHERE character_id = ? ORDER BY source_id`, saya).Scan(&rows).Error)
	require.Len(t, rows, 2, "the incumbent row survives next to the derived row")
	assert.Equal(t, "既有机翻介绍。", rows[0].Intro)
	assert.EqualValues(t, sourceDerived, rows[1].SourceID)
	assert.Equal(t, "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。", rows[1].Intro)

	cands, err = loadPanelCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	assert.Empty(t, cands, "a written derived row retires the character from the panel")
}

func TestRunPanelKeepsIncumbentOnSplit(t *testing.T) {
	_, saya, _ := seedPanelFixture(t)

	ex := fakeExtractor{out: map[string]string{"沙耶": "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。"}}
	judge := &fakeJudge{votes: []panelVote{voteChallenger, voteIncumbent, voteChallenger}}
	require.NoError(t, run(context.Background(), testDB, ex, judge, opts{Apply: true, Panel: true}))

	var n int64
	require.NoError(t, testDB.Raw(`SELECT count(*) FROM catalog_character_intro
		WHERE character_id = ? AND source_id = ?`, saya, sourceDerived).Scan(&n).Error)
	assert.Zero(t, n, "a split panel must not write the derived row")
}

func TestRunRefusesInventedPassages(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	saya := seedRosterChar(t, workID, "沙耶")

	ex := fakeExtractor{out: map[string]string{"沙耶": "其实是来自未来的魔法使,拯救了世界。"}}
	require.NoError(t, run(context.Background(), testDB, ex, nil, opts{Apply: true}))

	var n int64
	require.NoError(t, testDB.Raw(`SELECT count(*) FROM catalog_character_intro
		WHERE character_id = ?`, saya).Scan(&n).Error)
	assert.Zero(t, n, "an invented passage must never be written")
}
