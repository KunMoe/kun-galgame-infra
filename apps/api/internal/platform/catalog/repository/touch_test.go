package repository

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
		dbtest.SkipMain("catalog/repository")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("catalog/repository", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("catalog/repository", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("catalog/repository", "catalog seed failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func mkWork(t *testing.T, name string) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: name}
	require.NoError(t, testDB.Create(&w).Error)
	require.NoError(t, testDB.Exec(
		`UPDATE catalog_work SET updated_at = ? WHERE id = ?`, backdated, w.ID).Error)
	return w.ID
}

var backdated = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func updatedAt(t *testing.T, id int64) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, testDB.Raw(`SELECT updated_at FROM catalog_work WHERE id = ?`, id).Scan(&ts).Error)
	return ts
}

func TestTouchWorks(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work RESTART IDENTITY CASCADE`).Error)
	ctx := context.Background()

	a := mkWork(t, "touch-a")
	b := mkWork(t, "touch-b")
	c := mkWork(t, "touch-c")

	require.NoError(t, TouchWorks(ctx, testDB, nil))
	require.NoError(t, TouchWorks(ctx, testDB, []int64{}))
	assert.True(t, updatedAt(t, a).Equal(backdated), "empty set must not touch anything")

	require.NoError(t, TouchWorks(ctx, testDB, []int64{a, b, a, b, a}))
	assert.True(t, updatedAt(t, a).After(backdated), "a bumped")
	assert.True(t, updatedAt(t, b).After(backdated), "b bumped")
	assert.True(t, updatedAt(t, c).Equal(backdated), "c untouched")

	require.NoError(t, TouchWorks(ctx, testDB, []int64{c, 9_000_001}))
	assert.True(t, updatedAt(t, c).After(backdated), "c bumped")

	require.NoError(t, TouchWorks(ctx, testDB, []int64{0}))
}

func TestTouchWorksChunking(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work RESTART IDENTITY CASCADE`).Error)
	ctx := context.Background()

	const n = touchChunk + 7
	works := make([]model.CatalogWork, n)
	for i := range works {
		works[i] = model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: fmt.Sprintf("bulk-%d", i)}
	}
	require.NoError(t, testDB.CreateInBatches(works, 500).Error)
	require.NoError(t, testDB.Exec(`UPDATE catalog_work SET updated_at = ?`, backdated).Error)

	ids := make([]int64, 0, n)
	for _, w := range works {
		ids = append(ids, w.ID)
	}
	require.NoError(t, TouchWorks(ctx, testDB, ids))

	var stale int64
	require.NoError(t, testDB.Raw(
		`SELECT count(*) FROM catalog_work WHERE updated_at = ?`, backdated).Scan(&stale).Error)
	assert.Zero(t, stale, "every work across both chunks bumped")
}
