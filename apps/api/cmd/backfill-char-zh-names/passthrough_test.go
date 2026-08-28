package main

import (
	"context"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/seed"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("cmd/backfill-char-zh-names")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/backfill-char-zh-names", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("cmd/backfill-char-zh-names", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("cmd/backfill-char-zh-names", "catalog seed failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func TestPureHan(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"雪村時音", true},
		{"結城 美柑", true},
		{"佐々木", true},      // iteration mark 々 is script Han (U+3005)
		{"涼宮ハルヒ", false},   // katakana
		{"ひなた", false},     // hiragana
		{"アリス", false},     // katakana only
		{"Alice", false},   // latin
		{"雪村Alice", false}, // mixed latin
		{"エレン＝ローズ", false}, // katakana + double hyphen
		{"レナ=リヒテ", false},  // separator
		{"雪村・時音", false},   // katakana middle dot — conservative exclusion
		{"雪風ー", false},     // long-vowel mark
		{"", false},        // empty
		{" ", false},       // whitespace only
		{"時雨　沢", true},     // ideographic space between Han
		{"雪村23", false},    // digits
		{"猫神さま", false},    // trailing hiragana
	}
	for _, c := range cases {
		assert.Equal(t, c.want, pureHan(c.name), "pureHan(%q)", c.name)
	}
}

func seedCharacter(t *testing.T, name string, latin *string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, testDB.Raw(`INSERT INTO catalog_character
		(display_name, latin, lang, description, extra, field_provenance, created_at, updated_at)
		VALUES (?, ?, 'ja', '', '{}', '{}', now(), now()) RETURNING id`, name, latin).Scan(&id).Error)
	return id
}

func seedAlias(t *testing.T, charID int64, name, lang string, kind, provenance int16, primary bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_character_alias
		(character_id, name, lang, kind, is_primary_for_locale, provenance, mt_model)
		VALUES (?, ?, ?, ?, ?, ?, '')`, charID, name, lang, kind, primary, provenance).Error)
}

func truncateCharacters(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_character RESTART IDENTITY CASCADE`).Error)
}

type aliasRow struct {
	Name       string `gorm:"column:name"`
	Lang       string `gorm:"column:lang"`
	Kind       int16  `gorm:"column:kind"`
	IsPrimary  bool   `gorm:"column:is_primary_for_locale"`
	Provenance int16  `gorm:"column:provenance"`
	MTModel    string `gorm:"column:mt_model"`
}

func loadAliases(t *testing.T, charID int64) []aliasRow {
	t.Helper()
	var rows []aliasRow
	require.NoError(t, testDB.Raw(`SELECT name, lang, kind, is_primary_for_locale, provenance, mt_model
		FROM catalog_character_alias WHERE character_id = ? ORDER BY id`, charID).Scan(&rows).Error)
	return rows
}

func TestPassthroughWritesIdentityRow(t *testing.T) {
	truncateCharacters(t)
	kanji := seedCharacter(t, "雪村時音", nil)
	kana := seedCharacter(t, "涼宮ハルヒ", nil)

	require.NoError(t, runPassthrough(context.Background(), testDB, true, 0, 0))

	rows := loadAliases(t, kanji)
	require.Len(t, rows, 1)
	assert.Equal(t, "雪村時音", rows[0].Name)
	assert.Equal(t, "zh-Hans", rows[0].Lang)
	assert.True(t, rows[0].IsPrimary, "the identity row claims the locale primary")
	assert.EqualValues(t, 0, rows[0].Provenance, "passthrough is source provenance, not machine")
	assert.Empty(t, loadAliases(t, kana), "a kana name belongs to the --mt lane, never the passthrough")
}

func TestPassthroughSkipsCharactersWithAnyZhName(t *testing.T) {
	truncateCharacters(t)
	dictNamed := seedCharacter(t, "時雨", nil)
	seedAlias(t, dictNamed, "时雨", "zh-Hans", 0, 0, true)

	require.NoError(t, runPassthrough(context.Background(), testDB, true, 0, 0))

	rows := loadAliases(t, dictNamed)
	require.Len(t, rows, 1, "a character already named in zh gains nothing from the passthrough")
	assert.Equal(t, "时雨", rows[0].Name)
}

func TestPassthroughIsIdempotent(t *testing.T) {
	truncateCharacters(t)
	id := seedCharacter(t, "雪村時音", nil)

	require.NoError(t, runPassthrough(context.Background(), testDB, true, 0, 0))
	require.NoError(t, runPassthrough(context.Background(), testDB, true, 0, 0))

	assert.Len(t, loadAliases(t, id), 1, "a second sweep inserts nothing")
}
