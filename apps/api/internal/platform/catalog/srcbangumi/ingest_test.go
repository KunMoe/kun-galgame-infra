package srcbangumi

import (
	"os"
	"testing"
	"time"

	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDecodeSubjectTransform(t *testing.T) {
	now := time.Now()

	ok := []byte(`{"id":4,"type":4,"name":"CLANNAD","name_cn":"CLANNAD","infobox":"{{Infobox Game\r\n|中文名= CLANNAD\r\n}}","platform":4001,"summary":"s","nsfw":false,"tags":[{"name":"KEY","count":1}],"meta_tags":[],"score":9,"score_details":{},"rank":1,"date":"2004-04-28","favorite":{},"series":false}`)
	row, isErr, err := decodeSubject(ok, now)
	require.NoError(t, err)
	assert.False(t, isErr)
	assert.Equal(t, int64(4), row.ID)
	assert.Equal(t, "", row.ParseError)
	require.NotNil(t, row.InfoboxParsed, "parsed infobox must land as jsonb")
	assert.Contains(t, string(row.InfoboxParsed), "CLANNAD")
	assert.Equal(t, ParserVersion, row.ParserVersion)

	bad := []byte(`{"id":15,"type":1,"name":"x","name_cn":"","infobox":"{{Infobox Book\r\n|别名={\r\n[x]\r\n","platform":0,"summary":"","nsfw":false,"score":0,"rank":0,"date":"","series":false}`)
	row, isErr, err = decodeSubject(bad, now)
	require.NoError(t, err)
	assert.True(t, isErr)
	assert.Nil(t, row.InfoboxParsed)
	assert.NotEmpty(t, row.ParseError)

	_, _, err = decodeSubject([]byte(`{not json`), now)
	require.Error(t, err)
}

func TestJSONOrNull(t *testing.T) {
	assert.Nil(t, jsonOrNull(nil))
	assert.Nil(t, jsonOrNull([]byte("null")))
	assert.Equal(t, `[{"a":1}]`, string(jsonOrNull([]byte(`[{"a":1}]`))))
}

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("catalog/srcbangumi")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		dbtest.SkipMainf("catalog/srcbangumi", "cannot connect to test database: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		dbtest.SkipMainf("catalog/srcbangumi", "ensure src_bangumi schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func TestIngestFixtureAndIdempotency(t *testing.T) {
	report, err := Run(testDB, "testdata", "")
	require.NoError(t, err)

	expect := map[string]int64{
		"subject": 3, "person": 2, "character": 1,
		"subject-relations": 3,
		"subject-persons":   2, "subject-characters": 1, "person-characters": 1,
	}
	for name, want := range expect {
		assert.Equalf(t, want, report.PerFile[name].Rows, "%s read-row count", name)
	}
	assert.Equal(t, int64(1), report.PerFile["subject"].ParseErrors)
	assert.Equal(t, int64(1), report.PerFile["person"].ParseErrors)

	tableCounts := func() map[string]int64 {
		out := map[string]int64{}
		for _, table := range []string{
			"src_bangumi.subject", "src_bangumi.person", "src_bangumi.character",
			"src_bangumi.subject_relation", "src_bangumi.subject_person",
			"src_bangumi.subject_character", "src_bangumi.person_character",
		} {
			var n int64
			require.NoError(t, testDB.Table(table).Count(&n).Error)
			out[table] = n
		}
		return out
	}
	first := tableCounts()
	assert.Equal(t, int64(2), first["src_bangumi.subject_relation"], "exact duplicate deduped")
	assert.Equal(t, int64(3), first["src_bangumi.subject"])

	var clannad Subject
	require.NoError(t, testDB.First(&clannad, 4).Error)
	assert.Contains(t, string(clannad.InfoboxParsed), "クラナド")
	assert.Equal(t, "", clannad.ParseError)

	var bad Subject
	require.NoError(t, testDB.First(&bad, 15).Error)
	assert.Nil(t, bad.InfoboxParsed)
	assert.NotEmpty(t, bad.ParseError)

	var norm string
	require.NoError(t, testDB.Raw(
		`SELECT name_norm FROM src_bangumi.subject WHERE id = 8`).Scan(&norm).Error)
	assert.Equal(t, "テスト ゲーム", norm, "NFKC generated column folds U+3000")

	_, err = Run(testDB, "testdata", "")
	require.NoError(t, err)
	assert.Equal(t, first, tableCounts(), "second run must reproduce identical row counts")

	var runs int64
	require.NoError(t, testDB.Table("src_bangumi.ingest_run").Count(&runs).Error)
	assert.GreaterOrEqual(t, runs, int64(2))

	_, err = Run(testDB, "testdata", "subject")
	require.NoError(t, err)
	assert.Equal(t, first, tableCounts())
}
