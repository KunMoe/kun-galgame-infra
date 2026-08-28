package main

import (
	"fmt"
	"os"
	"testing"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/platform/editing"
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
		fmt.Fprintln(os.Stderr, "SKIP: no TEST_DATABASE_DSN — the DB-backed test here skips individually")
		os.Exit(m.Run())
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/catalog-char-xsrc", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: catalog migrate failed: %v\n", err)
		os.Exit(1)
	}
	if err := seed.Run(db); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: catalog seed failed: %v\n", err)
		os.Exit(1)
	}
	testDB = db
	os.Exit(m.Run())
}

// TestCreditsSuppressionExcludedOnEveryReadSite is this package's share of the
// R2c-1 read-site sweep: a VA bridge says "these two characters share a voice
// actor on the same work", which is the evidence a cross-source pairing is
// built from. A suppressed credit is the statement that the voicing is wrong,
// and a merge candidate derived from it hands the reviewer back the mistake
// they just recorded.
func TestCreditsSuppressionExcludedOnEveryReadSite(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	for _, tbl := range []string{
		"catalog_credit", "catalog_credit_name", "catalog_character", "catalog_work",
		"edit_suppressed_row",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
	const vaRole = int64(1)
	work := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "作品",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive}
	require.NoError(t, testDB.Create(&work).Error)
	cn := model.CatalogCreditName{Name: "声優", Lang: "ja"}
	require.NoError(t, testDB.Create(&cn).Error)

	mkChar := func(name string) int64 {
		c := model.CatalogCharacter{DisplayName: name}
		require.NoError(t, testDB.Create(&c).Error)
		return c.ID
	}
	a, b := mkChar("キャラA"), mkChar("キャラB")
	for _, id := range []int64{a, b} {
		charID := id
		require.NoError(t, testDB.Create(&model.CatalogCredit{
			WorkID: work.ID, CreditNameID: cn.ID, RoleID: vaRole, CharacterID: &charID,
		}).Error)
	}

	bridges, err := loadVABridges(testDB)
	require.NoError(t, err)
	require.Len(t, bridges, 1, "the two characters share a VA on one work")

	require.NoError(t, testDB.Create(&editing.SuppressedRow{
		EntityType: editspec.TypeWork, EntityID: work.ID, FieldKey: editspec.FieldWorkCredits,
		IdentityKey: editspec.CreditIdentity(vaRole, cn.ID, b),
	}).Error)
	bridges, err = loadVABridges(testDB)
	require.NoError(t, err)
	assert.Empty(t, bridges, "a suppressed credit is not pairing evidence")
}
