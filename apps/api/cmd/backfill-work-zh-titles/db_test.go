package main

import (
	"fmt"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcb "api/internal/platform/catalog/srcbangumi"
	srcv "api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("cmd/backfill-work-zh-titles")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/backfill-work-zh-titles", "cannot connect to test database: %v", err)
	}
	for _, step := range []struct {
		name string
		run  func(*gorm.DB) error
	}{
		{"catalog migrate", migrate.Run},
		{"catalog seed", seed.Run},
		{"src_bangumi schema", srcb.EnsureSchema},
		{"src_vndb schema", srcv.EnsureSchema},
	} {
		if err := step.run(db); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %s: %v\n", step.name, err)
			os.Exit(1)
		}
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_work_title", "catalog_external_ref", "catalog_series_member", "catalog_series",
		"catalog_work_relation", "catalog_work_popularity", "catalog_work",
		"src_bangumi.subject", "src_vndb.releases_titles", "src_vndb.releases_vn",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func galgameMedium(t *testing.T) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func sourceID(t *testing.T, key string) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func mkWork(t *testing.T, name string) int64 {
	t.Helper()
	w := model.CatalogWork{MediumID: galgameMedium(t), OLang: "ja", DisplayName: name}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkClaimedWork(t *testing.T, name string, productWorkID int64) int64 {
	t.Helper()
	site := forumSite
	w := model.CatalogWork{
		MediumID: galgameMedium(t), OLang: "ja", DisplayName: name,
		Site: &site, ProductWorkID: &productWorkID,
	}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkTitle(t *testing.T, workID int64, lang, title string, kind, provenance int16) int64 {
	t.Helper()
	row := model.CatalogWorkTitle{
		WorkID: workID, Lang: lang, Title: title, Kind: kind, Provenance: provenance,
	}
	require.NoError(t, testDB.Create(&row).Error)
	return row.ID
}

func mkAnchor(t *testing.T, workID int64, source, externalID string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: sourceID(t, source),
		ExternalID: externalID, LinkKind: model.LinkKindExact, MatchedBy: "rule:test",
	}).Error)
}

func mkSubject(t *testing.T, id int64, name, nameCN string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_bangumi.subject (id, type, name, name_cn, infobox_raw, parse_error,
			platform, summary, nsfw, date, series, score, rank, parser_version, ingested_at)
		 VALUES (?, 4, ?, ?, '', '', 0, '', false, '', false, 0, 0, 't', now())`,
		id, name, nameCN).Error)
}

func mkRelease(t *testing.T, releaseID, vid, lang, title string, mtl bool) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.releases_vn (id, vid, rtype) VALUES (?, ?, 'complete')
		 ON CONFLICT DO NOTHING`, releaseID, vid).Error)
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.releases_titles (id, lang, mtl, title, latin)
		 VALUES (?, ?, ?, ?, '')`, releaseID, lang, mtl, title).Error)
}

func titlesOf(t *testing.T, workID int64) []model.CatalogWorkTitle {
	t.Helper()
	var rows []model.CatalogWorkTitle
	require.NoError(t, testDB.Where("work_id = ?", workID).Order("id").Find(&rows).Error)
	return rows
}
