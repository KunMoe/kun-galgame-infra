package curatedseries

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
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
		dbtest.SkipMain("jobs/curatedseries")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/curatedseries", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/curatedseries", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/curatedseries", "catalog seed failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func TestParseSeriesKeepsMultilineDescriptions(t *testing.T) {
	rows, err := parseSeries("testdata")
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, seriesRow{ID: 9001, Name: "Series Alpha", Description: "Alpha line one\nAlpha line two"}, rows[0])
	assert.Equal(t, seriesRow{ID: 9002, Name: "Series Beta", Description: ""}, rows[1])
	assert.Equal(t, int64(9003), rows[2].ID)
}

func fixture(t *testing.T, members []memberRow) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", seriesFile))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, seriesFile), src, 0o644))

	var body string
	for _, m := range members {
		body += fmt.Sprintf("%d\tfixture\t%d\t%d\t0\n", m.SeriesID, m.WorkID, m.WorkID)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, membersFile), []byte(body), 0o644))
	return dir
}

func mkWork(t *testing.T) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: 1}
	require.NoError(t, testDB.Create(&w).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM catalog_series_member WHERE work_id = ?`, w.ID)
		testDB.Exec(`DELETE FROM catalog_work WHERE id = ?`, w.ID)
	})
	return w.ID
}

func resetCurated(t *testing.T) {
	t.Helper()
	clean := func() {
		testDB.Exec(`DELETE FROM catalog_series_intro WHERE series_id IN
			(SELECT id FROM catalog_series WHERE external_id LIKE 'wiki:900%')`)
		testDB.Exec(`DELETE FROM catalog_series_member WHERE series_id IN
			(SELECT id FROM catalog_series WHERE external_id LIKE 'wiki:900%')`)
		testDB.Exec(`DELETE FROM catalog_series WHERE external_id LIKE 'wiki:900%'`)
	}
	clean()
	t.Cleanup(clean)
}

func run(t *testing.T, dir string, apply bool) *Stats {
	t.Helper()
	st, err := RunWithDB(context.Background(), testDB, Opts{
		Apply: apply, ArtifactsDir: dir, Receipts: filepath.Join(t.TempDir(), "receipts.jsonl"),
	})
	require.NoError(t, err)
	return st
}

func curatedSeriesID(t *testing.T, wikiID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, testDB.Model(&model.CatalogSeries{}).
		Where("external_id = ?", fmt.Sprintf("wiki:%d", wikiID)).Pluck("id", &id).Error)
	return id
}

func TestSeedCreatesCuratedLane(t *testing.T) {
	resetCurated(t)
	w1, w2, w3 := mkWork(t), mkWork(t), mkWork(t)
	dir := fixture(t, []memberRow{
		{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: w2},
		{SeriesID: 9002, WorkID: w3},
	})

	dry := run(t, dir, false)
	assert.Equal(t, 3, dry.SeriesInFile)
	assert.Equal(t, 2, dry.SeriesWithMembers)
	assert.Equal(t, 2, dry.SeriesSeeded)
	assert.Equal(t, 3, dry.MembersInserted)
	assert.Equal(t, 1, dry.IntrosInserted)
	assert.Equal(t, 0, dry.TouchedWorks)

	var before int64
	require.NoError(t, testDB.Model(&model.CatalogSeries{}).
		Where("external_id LIKE 'wiki:900%'").Count(&before).Error)
	assert.Zero(t, before, "a dry run must write nothing")

	got := run(t, dir, true)
	assert.Equal(t, 2, got.SeriesSeeded)
	assert.Equal(t, 3, got.MembersInserted)
	assert.Equal(t, 1, got.IntrosInserted)
	assert.Equal(t, 3, got.TouchedWorks)
	assert.Zero(t, curatedSeriesID(t, 9003), "a member-less series is not a grouping worth having")

	alpha := curatedSeriesID(t, 9001)
	require.NotZero(t, alpha)
	var row model.CatalogSeries
	require.NoError(t, testDB.First(&row, alpha).Error)
	assert.Equal(t, "Series Alpha", row.DisplayName)

	var curated int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'curated'`).Scan(&curated).Error)
	assert.Equal(t, curated, row.SourceID)

	var members []int64
	require.NoError(t, testDB.Model(&model.CatalogSeriesMember{}).
		Where("series_id = ?", alpha).Order("work_id").Pluck("work_id", &members).Error)
	assert.Equal(t, []int64{w1, w2}, members)

	var intro model.CatalogSeriesIntro
	require.NoError(t, testDB.Where("series_id = ?", alpha).First(&intro).Error)
	assert.Equal(t, "Alpha line one\nAlpha line two", intro.Intro)
	assert.Equal(t, "zh-Hans", intro.Lang)
	assert.Equal(t, curated, intro.SourceID)

	beta := curatedSeriesID(t, 9002)
	require.NotZero(t, beta)
	assert.ErrorIs(t, testDB.Where("series_id = ?", beta).First(&intro).Error, gorm.ErrRecordNotFound)
}

func TestSecondApplyIsAZeroWrite(t *testing.T) {
	resetCurated(t)
	w1, w2 := mkWork(t), mkWork(t)
	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: w2}})

	first := run(t, dir, true)
	require.Equal(t, 1, first.SeriesSeeded)
	require.Equal(t, 2, first.MembersInserted)
	require.Equal(t, 1, first.IntrosInserted)

	second := run(t, dir, true)
	assert.Zero(t, second.SeriesSeeded)
	assert.Zero(t, second.MembersInserted)
	assert.Zero(t, second.IntrosInserted)
	assert.Zero(t, second.TouchedWorks)
	assert.Equal(t, 1, second.SeriesExisting)
	assert.Equal(t, 1, second.SeriesUntouched)
	assert.Zero(t, second.MembersExisting)
	assert.Zero(t, second.IntrosExisting)
}

func TestExistingDisplayNameSurvives(t *testing.T) {
	resetCurated(t)
	w1 := mkWork(t)
	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}})

	require.Equal(t, 1, run(t, dir, true).SeriesSeeded)
	id := curatedSeriesID(t, 9001)
	require.NoError(t, testDB.Model(&model.CatalogSeries{}).Where("id = ?", id).
		Update("display_name", "Renamed By A Human").Error)

	run(t, dir, true)
	var row model.CatalogSeries
	require.NoError(t, testDB.First(&row, id).Error)
	assert.Equal(t, "Renamed By A Human", row.DisplayName)
}

func TestExistingSeriesDoesNotResurrectHumanEdits(t *testing.T) {
	resetCurated(t)
	w1, w2 := mkWork(t), mkWork(t)
	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: w2}})

	require.Equal(t, 1, run(t, dir, true).SeriesSeeded)
	id := curatedSeriesID(t, 9001)
	require.NotZero(t, id)
	require.NoError(t, testDB.Where("series_id = ? AND work_id = ?", id, w2).Delete(&model.CatalogSeriesMember{}).Error)
	require.NoError(t, testDB.Where("series_id = ?", id).Delete(&model.CatalogSeriesIntro{}).Error)

	dry := run(t, dir, false)
	assert.Equal(t, 1, dry.SeriesExisting)
	assert.Equal(t, 1, dry.SeriesUntouched)
	assert.Zero(t, dry.MembersInserted)
	assert.Zero(t, dry.IntrosInserted)
	assert.Zero(t, dry.TouchedWorks)

	got := run(t, dir, true)
	assert.Equal(t, 1, got.SeriesExisting)
	assert.Equal(t, 1, got.SeriesUntouched)
	assert.Zero(t, got.MembersInserted)
	assert.Zero(t, got.IntrosInserted)
	assert.Zero(t, got.TouchedWorks)

	var members []int64
	require.NoError(t, testDB.Model(&model.CatalogSeriesMember{}).
		Where("series_id = ?", id).Order("work_id").Pluck("work_id", &members).Error)
	assert.Equal(t, []int64{w1}, members)

	var n int64
	require.NoError(t, testDB.Model(&model.CatalogSeriesIntro{}).Where("series_id = ?", id).Count(&n).Error)
	assert.Zero(t, n)
}

func TestDlsiteCoveredSeriesIsSkipped(t *testing.T) {
	resetCurated(t)
	w1, w2, w3 := mkWork(t), mkWork(t), mkWork(t)

	var dlsite int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'dlsite'`).Scan(&dlsite).Error)
	up := model.CatalogSeries{DisplayName: "Upstream", SourceID: dlsite,
		ExternalID: fmt.Sprintf("curatedseries-test-%d", w1)}
	require.NoError(t, testDB.Create(&up).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM catalog_series_member WHERE series_id = ?`, up.ID)
		testDB.Exec(`DELETE FROM catalog_series WHERE id = ?`, up.ID)
	})
	for _, w := range []int64{w1, w2, w3} {
		require.NoError(t, testDB.Create(&model.CatalogSeriesMember{SeriesID: up.ID, WorkID: w}).Error)
	}

	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: w2}})
	st := run(t, dir, true)
	assert.Equal(t, 1, st.SeriesSkippedCovered)
	assert.Zero(t, st.SeriesSeeded)
	assert.Zero(t, st.MembersInserted)
	assert.Zero(t, curatedSeriesID(t, 9001))

	var n int64
	require.NoError(t, testDB.Model(&model.CatalogSeriesMember{}).
		Where("series_id = ?", up.ID).Count(&n).Error)
	assert.EqualValues(t, 3, n, "the dlsite series must be left exactly as found")
}

func TestPartialDlsiteOverlapStillSeeds(t *testing.T) {
	resetCurated(t)
	w1, w2 := mkWork(t), mkWork(t)

	var dlsite int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = 'dlsite'`).Scan(&dlsite).Error)
	up := model.CatalogSeries{DisplayName: "Upstream", SourceID: dlsite,
		ExternalID: fmt.Sprintf("curatedseries-test-partial-%d", w1)}
	require.NoError(t, testDB.Create(&up).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM catalog_series_member WHERE series_id = ?`, up.ID)
		testDB.Exec(`DELETE FROM catalog_series WHERE id = ?`, up.ID)
	})
	require.NoError(t, testDB.Create(&model.CatalogSeriesMember{SeriesID: up.ID, WorkID: w1}).Error)

	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: w2}})
	st := run(t, dir, true)
	assert.Zero(t, st.SeriesSkippedCovered)
	assert.Equal(t, 1, st.SeriesSeeded)
	assert.Equal(t, 2, st.MembersInserted)
}

func TestUnmappedWorkIsDropped(t *testing.T) {
	resetCurated(t)
	w1 := mkWork(t)
	dir := fixture(t, []memberRow{{SeriesID: 9001, WorkID: w1}, {SeriesID: 9001, WorkID: 1 << 40}})

	st := run(t, dir, true)
	assert.Equal(t, 1, st.MembersSkippedNoWork)
	assert.Equal(t, 1, st.MembersInserted)
}
