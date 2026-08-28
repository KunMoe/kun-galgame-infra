package main

import (
	"fmt"
	"io"
	"os"
	"testing"

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

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("cmd/import-entity-aliases")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/import-entity-aliases", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("cmd/import-entity-aliases", "catalog migration failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("cmd/import-entity-aliases", "catalog seeding failed: %v", err)
	}
	if err := srcb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("cmd/import-entity-aliases", "src_bangumi schema failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func TestFold(t *testing.T) {
	assert.Equal(t, "test", foldName(" Te st (歌手) "))
	assert.Equal(t, "緒方剛志", foldName("緒方剛志(ぼうのうと)"))
	assert.Equal(t, []string{"ささきむつみ"}, parenItemsRaw("藤宮博也(ささきむつみ)"))
	assert.Equal(t, []string{"今井楓人", "野村美月"}, parenItemsRaw("村中志帆(今井楓人、野村美月)"))
	assert.Equal(t, []string{"ささきむつみ"}, parenAliases("藤宮博也(ささきむつみ)"))
	assert.True(t, isRoleTag("声優"))
	assert.True(t, isRoleTag(" CV "))
	assert.False(t, isRoleTag("ささきむつみ"))
}

func cleanCatalog(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_name_alias, catalog_match_candidate,
		catalog_external_ref, catalog_credit_name, catalog_person RESTART IDENTITY CASCADE`).Error)
}

var extSeq int64

func seedCN(t *testing.T, name string, source int16, personID *int64) int64 {
	t.Helper()
	cn := &model.CatalogCreditName{Name: name, Kind: model.CreditNameKindMain, LinkVisibility: model.LinkVisibilityPublic, PersonID: personID}
	require.NoError(t, testDB.Create(cn).Error)
	rule := map[int16]string{sourceBangumi: "rule:bangumi-person-import", sourceEG: "rule:eg-creater-import", sourceDLsite: "rule:dlsite-creater-import"}[source]
	extSeq++
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_external_ref
		(entity_type, entity_id, source_id, external_id, link_kind, matched_by)
		VALUES (1, ?, ?, ?, 0, ?)`, cn.ID, source, fmt.Sprintf("x%d", extSeq), rule).Error)
	return cn.ID
}

const sourceDLsite int16 = 4

func hintNames(t *testing.T, owner int64) []string {
	t.Helper()
	var names []string
	require.NoError(t, testDB.Raw(
		`SELECT name FROM catalog_name_alias WHERE credit_name_id=? AND kind=? ORDER BY name`,
		owner, model.AliasKindSearchHint).Scan(&names).Error)
	return names
}

func TestLegAEGHints(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	cleanCatalog(t)

	x := seedCN(t, "藤宮博也(ささきむつみ)", sourceEG, nil)
	seedCN(t, "七瀬(声優)", sourceEG, nil)
	seedCN(t, "緒方剛志(緒方剛志)", sourceEG, nil)

	_, err := runHints(testDB, io.Discard, false)
	require.NoError(t, err)
	assert.Empty(t, hintNames(t, x), "dry writes nothing")

	st, err := runHints(testDB, io.Discard, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"ささきむつみ"}, hintNames(t, x), "paren alias becomes a search hint")
	assert.GreaterOrEqual(t, st.SkippedRole, 1, "role tag skipped")
	assert.GreaterOrEqual(t, st.SkippedSame, 1, "alias equal to main name skipped")

	st2, err := runHints(testDB, io.Discard, true)
	require.NoError(t, err)
	assert.Zero(t, st2.EGNames, "re-run writes no new hints")
	assert.Equal(t, []string{"ささきむつみ"}, hintNames(t, x))
}

func candidateReason(t *testing.T, a, b int64) (int16, bool) {
	t.Helper()
	var r []int16
	require.NoError(t, testDB.Raw(
		`SELECT reason FROM catalog_match_candidate WHERE entity_type=1 AND a_id=? AND b_id=?`, a, b).Scan(&r).Error)
	if len(r) == 0 {
		return 0, false
	}
	return r[0], true
}

func TestLegBAliasDeclared(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	cleanCatalog(t)

	x := seedCN(t, "藤宮博也(ささきむつみ)", sourceEG, nil)
	y := seedCN(t, "ささきむつみ", sourceBangumi, nil)

	_, err := runCandidates(testDB, io.Discard, false)
	require.NoError(t, err)
	_, ok := candidateReason(t, min(x, y), max(x, y))
	assert.False(t, ok, "dry writes nothing")

	st, err := runCandidates(testDB, io.Discard, true)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Generated)
	reason, ok := candidateReason(t, min(x, y), max(x, y))
	require.True(t, ok)
	assert.Equal(t, model.CandidateReasonAliasDeclared, reason)

	st2, err := runCandidates(testDB, io.Discard, true)
	require.NoError(t, err)
	assert.Zero(t, st2.Generated)
	assert.Equal(t, 1, st2.AlreadyCand)
}

func TestLegBGuards(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}

	t.Run("ambiguous", func(t *testing.T) {
		cleanCatalog(t)
		seedCN(t, "本名A(共通名)", sourceEG, nil)
		seedCN(t, "共通名", sourceBangumi, nil)
		seedCN(t, "共通名", sourceDLsite, nil)
		st, err := runCandidates(testDB, io.Discard, true)
		require.NoError(t, err)
		assert.Zero(t, st.Generated)
		assert.Equal(t, 1, st.Ambiguous)
	})

	t.Run("same source not matched", func(t *testing.T) {
		cleanCatalog(t)
		seedCN(t, "本名B(同源名)", sourceEG, nil)
		seedCN(t, "同源名", sourceEG, nil)
		st, err := runCandidates(testDB, io.Discard, true)
		require.NoError(t, err)
		assert.Zero(t, st.Generated)
		assert.Zero(t, st.Ambiguous)
	})

	t.Run("same person skipped", func(t *testing.T) {
		cleanCatalog(t)
		p := &model.CatalogPerson{DisplayName: "P", FieldProvenance: []byte(`{}`)}
		require.NoError(t, testDB.Create(p).Error)
		seedCN(t, "本名C(既同人)", sourceEG, &p.ID)
		seedCN(t, "既同人", sourceBangumi, &p.ID)
		st, err := runCandidates(testDB, io.Discard, true)
		require.NoError(t, err)
		assert.Zero(t, st.Generated)
		assert.Equal(t, 1, st.AlreadySame)
	})
}
