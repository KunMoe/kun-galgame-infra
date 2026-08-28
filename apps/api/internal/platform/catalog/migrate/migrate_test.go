package migrate

import (
	"os"
	"testing"

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
		dbtest.SkipMain("catalog/migrate")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("catalog/migrate", "no test db: %v", err)
	}
	if err := Run(db); err != nil {
		dbtest.SkipMainf("catalog/migrate", "setup: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

type columnShape struct {
	Nullable string `gorm:"column:is_nullable"`
	Type     string `gorm:"column:data_type"`
	Default  string `gorm:"column:column_default"`
}

func shapeOf(t *testing.T, table, column string) columnShape {
	t.Helper()
	var got columnShape
	require.NoError(t, testDB.Raw(`
		SELECT is_nullable, data_type, coalesce(column_default,'') AS column_default
		  FROM information_schema.columns
		 WHERE table_name = ? AND column_name = ?`, table, column).Scan(&got).Error)
	require.NotEmpty(t, got.Type, "%s.%s does not exist", table, column)
	return got
}

// The three provenance columns are the whole point of wave 195, and each one's
// nullability is a decision rather than an accident — a test that only checked
// "the column exists" would let the next AutoMigrate quietly relax any of them.
func TestAliasProvenanceColumnShape(t *testing.T) {
	for _, table := range []string{"catalog_character_alias", "catalog_name_alias", "catalog_label_alias"} {
		// provenance: 0=source is a MEANINGFUL value, so the column must be NOT
		// NULL with NO default — a default would let an INSERT that means to say
		// "source" be indistinguishable from one that forgot the column, which is
		// the GORM default-tag zero-value trap this schema keeps hitting.
		prov := shapeOf(t, table, "provenance")
		assert.Equal(t, "NO", prov.Nullable, "%s.provenance must be NOT NULL", table)
		assert.Empty(t, prov.Default, "%s.provenance must have no default", table)

		// source_id is NULLABLE on purpose: the pre-wave-195 corpus is only
		// partly attributable, and NULL is how a row says "not recorded" instead
		// of naming an upstream that may not have written it.
		src := shapeOf(t, table, "source_id")
		assert.Equal(t, "YES", src.Nullable, "%s.source_id must stay nullable", table)
		assert.Equal(t, "smallint", src.Type)

		mt := shapeOf(t, table, "mt_model")
		assert.Equal(t, "NO", mt.Nullable, "%s.mt_model must be NOT NULL", table)
	}
}

// Wave 210 puts the same provenance axis on catalog_work_title, and the
// backfill half of it can only be exercised by taking the column away again:
// on a fresh database AutoMigrate creates it NOT NULL from the model and the
// preMigrate block is a no-op, which is exactly the path production does NOT
// take.
func TestWorkTitleProvenanceBackfillsExistingRows(t *testing.T) {
	prov := shapeOf(t, "catalog_work_title", "provenance")
	assert.Equal(t, "NO", prov.Nullable, "catalog_work_title.provenance must be NOT NULL")
	assert.Empty(t, prov.Default, "catalog_work_title.provenance must have no default")
	for _, col := range []string{"src_hash", "mt_model"} {
		// Nullable: only a machine row has a source hash or a model name, and
		// NULL is how a source row says it has neither.
		assert.Equal(t, "YES", shapeOf(t, "catalog_work_title", col).Nullable,
			"catalog_work_title.%s must stay nullable", col)
	}

	var mediumID int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium ORDER BY id LIMIT 1`).Scan(&mediumID).Error)
	if mediumID == 0 {
		t.Skip("registry not seeded in this database")
	}
	var workID int64
	require.NoError(t, testDB.Raw(`
		INSERT INTO catalog_work (medium_id, olang, display_name, content_rating, status,
			display_nsfw, extra, field_provenance, created_at, updated_at)
		VALUES (?, 'ja', 'wave210 backfill fixture', 0, 0, false, '{}', '{}', now(), now())
		RETURNING id`, mediumID).Scan(&workID).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM catalog_work_title WHERE work_id = ?`, workID)
		testDB.Exec(`DELETE FROM catalog_work WHERE id = ?`, workID)
	})

	require.NoError(t, testDB.Exec(`ALTER TABLE catalog_work_title DROP COLUMN provenance`).Error)
	require.NoError(t, testDB.Exec(`
		INSERT INTO catalog_work_title (work_id, lang, title, kind)
		VALUES (?, 'ja', 'wave210 pre-column row', 0)`, workID).Error)

	require.NoError(t, Run(testDB))

	var got []int16
	require.NoError(t, testDB.Raw(
		`SELECT provenance FROM catalog_work_title WHERE work_id = ?`, workID).Scan(&got).Error)
	assert.Equal(t, []int16{0}, got, "a row that predates the column is source, not machine")
	assert.Equal(t, "NO", shapeOf(t, "catalog_work_title", "provenance").Nullable)
}

// The lang heal is data repair living in a schema migration, which is only
// defensible while it stays exactly as narrow as it claims to be: it must fix
// the malformed values, leave a legally-empty lang alone, and be a no-op on
// every later run.
func TestLangHealIsNarrowAndIdempotent(t *testing.T) {
	var labelID int64
	require.NoError(t, testDB.Raw(`
		INSERT INTO catalog_label (display_name, lang, kind, field_provenance, created_at, updated_at)
		VALUES ('wave195 heal fixture', 'japanese', 0, '{}', now(), now()) RETURNING id`).
		Scan(&labelID).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM catalog_label_alias WHERE label_id = ?`, labelID)
		testDB.Exec(`DELETE FROM catalog_label WHERE id = ?`, labelID)
	})

	require.NoError(t, testDB.Exec(`
		INSERT INTO catalog_label_alias (label_id, name, lang, kind, is_primary_for_locale, provenance)
		VALUES (?, 'wave195 malformed', '日语', 1, false, 0),
		       (?, 'wave195 unrecorded', '',   1, false, 0)`, labelID, labelID).Error)

	require.NoError(t, Run(testDB))

	var labelLang string
	require.NoError(t, testDB.Raw(`SELECT lang FROM catalog_label WHERE id = ?`, labelID).
		Scan(&labelLang).Error)
	assert.Equal(t, "ja", labelLang, "'japanese' is the author's own answer, normalized")

	var langs []string
	require.NoError(t, testDB.Raw(`
		SELECT lang FROM catalog_label_alias WHERE label_id = ? ORDER BY name`, labelID).
		Scan(&langs).Error)
	// An empty lang is "unrecorded", which is legal — the heal must not decide
	// it means Japanese just because a sibling row did.
	assert.Equal(t, []string{"ja", ""}, langs)

	// A second pass changes nothing: the UPDATEs match the malformed strings,
	// which no longer exist.
	require.NoError(t, Run(testDB))
	var after []string
	require.NoError(t, testDB.Raw(`
		SELECT lang FROM catalog_label_alias WHERE label_id = ? ORDER BY name`, labelID).
		Scan(&after).Error)
	assert.Equal(t, langs, after)
}
