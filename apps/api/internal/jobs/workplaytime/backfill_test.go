package workplaytime

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	testDB      *gorm.DB
	testDSN     string
	egTestDSN   string
	hltbTestDSN string
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/workplaytime")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/workplaytime", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workplaytime", "catalog migrate failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("jobs/workplaytime", "catalog seed failed: %v", err)
	}
	if err := srcvndb.EnsureSchema(db); err != nil {
		dbtest.SkipMainf("jobs/workplaytime", "src_vndb schema failed: %v", err)
	}
	for _, ddl := range []string{
		`CREATE SCHEMA IF NOT EXISTS workplaytime_eg`,
		`CREATE TABLE IF NOT EXISTS workplaytime_eg.games (id bigint PRIMARY KEY, raw jsonb)`,
		`CREATE SCHEMA IF NOT EXISTS workplaytime_hltb`,
		`CREATE TABLE IF NOT EXISTS workplaytime_hltb.games (hltb_id bigint PRIMARY KEY, raw jsonb)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			dbtest.SkipMainf("jobs/workplaytime", "mirror fixture failed: %v", err)
		}
	}
	egTestDSN = testDSN + " options='-csearch_path=workplaytime_eg'"
	hltbTestDSN = testDSN + " options='-csearch_path=workplaytime_hltb'"
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_playtime", "catalog_external_ref", "catalog_work",
		"src_vndb.vn", "workplaytime_eg.games", "workplaytime_hltb.games",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func sourceID(t *testing.T, key string) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func mediumID(t *testing.T) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func mkWork(t *testing.T, medium int16, name string, site *string) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: name, Site: site}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkAnchor(t *testing.T, workID int64, externalID string, source int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: source,
		ExternalID: externalID, LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
	}).Error)
}

func mkEGGame(t *testing.T, id int64, playtimeHours string) {
	t.Helper()
	raw := "{}"
	if playtimeHours != "" {
		raw = fmt.Sprintf(`{"total_play_time_median": %s}`, playtimeHours)
	}
	require.NoError(t, testDB.Exec(`INSERT INTO workplaytime_eg.games (id, raw) VALUES (?, ?::jsonb)`, id, raw).Error)
}

func mkVN(t *testing.T, id string, cLength *int, cLengthnum int) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcvndb.VN{
		ID: id, OLang: "ja", Image: "", CImage: "", Description: "",
		CVotecount: 0, CLength: cLength, CLengthnum: cLengthnum,
		Length: 0, Devstatus: 0, Alias: "", IngestedAt: time.Now(),
	}).Error)
}

func intPtr(n int) *int       { return &n }
func strPtr(s string) *string { return &s }

func TestBackfillWorkPlaytime(t *testing.T) {
	clean(t)
	medium := mediumID(t)
	egSrc := sourceID(t, "erogamescape")
	vndbSrc := sourceID(t, "vndb")

	wA := mkWork(t, medium, "eg-bodyless", nil)
	wB := mkWork(t, medium, "eg-overcap", nil)
	wC := mkWork(t, medium, "vndb-lane", nil)
	wD := mkWork(t, medium, "eg-claimed", strPtr("galgame_wiki"))
	mkAnchor(t, wA, "101", egSrc)
	mkAnchor(t, wB, "102", egSrc)
	mkAnchor(t, wC, "v11", vndbSrc)
	mkAnchor(t, wD, "103", egSrc)
	mkEGGame(t, 101, "5")
	mkEGGame(t, 102, "2000")
	mkEGGame(t, 103, "3")
	mkVN(t, "v11", intPtr(1234), 7)

	ctx := context.Background()
	opts := Opts{DSN: testDSN, EGDSN: egTestDSN, Source: "all"}

	st, err := Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 2, st.EGPlanned, "A + claimed D")
	assert.Equal(t, 1, st.EGRejected, "2000h over cap")
	assert.Equal(t, 1, st.VndbPlanned)
	assert.Equal(t, 0, st.EGWritten+st.VndbWritten)
	var n int64
	require.NoError(t, testDB.Table("catalog_work_playtime").Count(&n).Error)
	assert.Zero(t, n, "dry run must not write")

	opts.Apply = true
	st, err = Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 2, st.EGWritten)
	assert.Equal(t, 1, st.VndbWritten)
	assert.Zero(t, st.Errors)

	var row model.CatalogWorkPlaytime
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wA, egSrc).First(&row).Error)
	assert.Equal(t, 300, row.Minutes, "5h ×60")
	assert.Equal(t, 0, row.VoteCount, "EG publishes no per-work count")
	row = model.CatalogWorkPlaytime{}
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wC, vndbSrc).First(&row).Error)
	assert.Equal(t, 1234, row.Minutes)
	assert.Equal(t, 7, row.VoteCount)
	row = model.CatalogWorkPlaytime{}
	require.NoError(t, testDB.Where("work_id = ?", wD).First(&row).Error)
	assert.Equal(t, 180, row.Minutes, "claimed work admitted — no XOR arm for this facet")
	err = testDB.Where("work_id = ?", wB).First(&model.CatalogWorkPlaytime{}).Error
	assert.Error(t, err, "over-cap estimate never lands")

	st, err = Run(ctx, opts)
	require.NoError(t, err)
	assert.Zero(t, st.EGWritten+st.VndbWritten, "idempotent re-run")
	assert.Equal(t, 3, st.EGUnchanged+st.VndbUnchanged)

	require.NoError(t, testDB.Exec(`UPDATE workplaytime_eg.games SET raw = '{"total_play_time_median": 6}' WHERE id = 101`).Error)
	st, err = Run(ctx, opts)
	require.NoError(t, err)
	assert.Equal(t, 1, st.EGWritten, "refresh updates in place")
	row = model.CatalogWorkPlaytime{}
	require.NoError(t, testDB.Where("work_id = ? AND source_id = ?", wA, egSrc).First(&row).Error)
	assert.Equal(t, 360, row.Minutes)
}
