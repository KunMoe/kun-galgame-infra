package galgametouch

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
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
		dbtest.SkipMain("jobs/galgametouch")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/galgametouch", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/galgametouch", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/galgametouch", "catalog seed failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

var backdated = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func claim(t *testing.T, galgameID int) int64 {
	t.Helper()
	site := siteGalgameWiki
	pid := int64(galgameID)
	w := model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: fmt.Sprintf("claimed-%d", galgameID),
		Site: &site, ProductWorkID: &pid,
	}
	require.NoError(t, testDB.Create(&w).Error)
	require.NoError(t, testDB.Exec(
		`UPDATE catalog_work SET updated_at = ? WHERE id = ?`, backdated, w.ID).Error)
	return w.ID
}

func updatedAt(t *testing.T, workID int64) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, testDB.Raw(`SELECT updated_at FROM catalog_work WHERE id = ?`, workID).Scan(&ts).Error)
	return ts
}

var nextGalgameID = 700_000 + os.Getpid()%50_000

func galgameID() int {
	nextGalgameID++
	return nextGalgameID
}

func TestTouchClaimedOnly(t *testing.T) {
	ctx := context.Background()
	tou := New(testDB)

	claimed, unclaimed := galgameID(), galgameID()
	work := claim(t, claimed)

	require.NoError(t, tou.Touch(ctx, []int{claimed, unclaimed}))
	assert.True(t, updatedAt(t, work).After(backdated), "the claimed work is stamped")
	assert.Equal(t, 1, tou.Count(), "only the mapped work counts as touched")

	require.NoError(t, tou.Touch(ctx, []int{unclaimed}))
	assert.Equal(t, 1, tou.Count(), "an unmapped galgame adds nothing")
}

func TestTouchIgnoresOtherSites(t *testing.T) {
	ctx := context.Background()
	tou := New(testDB)

	gid := galgameID()
	mine := claim(t, gid)

	other := "moyu"
	pid := int64(gid)
	foreign := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "foreign", Site: &other, ProductWorkID: &pid}
	require.NoError(t, testDB.Create(&foreign).Error)
	require.NoError(t, testDB.Exec(`UPDATE catalog_work SET updated_at = ? WHERE id = ?`, backdated, foreign.ID).Error)

	require.NoError(t, tou.Touch(ctx, []int{gid}))
	assert.True(t, updatedAt(t, mine).After(backdated), "the galgame_wiki claim is stamped")
	assert.True(t, updatedAt(t, foreign.ID).Equal(backdated), "another tenant's work is never stamped")
}

func TestTouchSkipsSoftDeleted(t *testing.T) {
	ctx := context.Background()
	tou := New(testDB)

	gid := galgameID()
	work := claim(t, gid)
	require.NoError(t, testDB.Exec(
		`UPDATE catalog_work SET deleted_at = now(), updated_at = ? WHERE id = ?`, backdated, work).Error)

	require.NoError(t, tou.Touch(ctx, []int{gid}))
	assert.True(t, updatedAt(t, work).Equal(backdated), "a soft-deleted work is left alone")
	assert.Zero(t, tou.Count())
}

func TestTouchDedupsAndIgnoresEmpty(t *testing.T) {
	ctx := context.Background()
	tou := New(testDB)

	gid := galgameID()
	work := claim(t, gid)

	require.NoError(t, tou.Touch(ctx, nil))
	require.NoError(t, tou.Touch(ctx, []int{}))
	require.NoError(t, tou.Touch(ctx, []int{0, -1}))
	assert.True(t, updatedAt(t, work).Equal(backdated), "no input means no write")
	assert.Zero(t, tou.Count())

	require.NoError(t, tou.Touch(ctx, []int{gid, gid, gid}))
	assert.Equal(t, 1, tou.Count(), "duplicates collapse to one work")
}

func TestNilToucherIsNoop(t *testing.T) {
	ctx := context.Background()
	gid := galgameID()
	work := claim(t, gid)

	var tou *Toucher
	require.NoError(t, tou.Touch(ctx, []int{gid}))
	tou.Close()
	assert.Zero(t, tou.Count())
	assert.True(t, updatedAt(t, work).Equal(backdated), "a dry run must not move any watermark")
}
