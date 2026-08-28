package releaselabels

import (
	"context"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcv "api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	testDB  *gorm.DB
	testDSN string
)

func TestMain(m *testing.M) {
	var ok bool
	testDSN, ok = dbtest.DSN()
	if !ok {
		dbtest.SkipMain("jobs/releaselabels")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/releaselabels", "no test db: %v", err)
	}
	for _, step := range []func(*gorm.DB) error{migrate.Run, seed.Run, srcv.EnsureSchema} {
		if err := step(db); err != nil {
			dbtest.SkipMainf("jobs/releaselabels", "setup: %v", err)
		}
	}
	testDB = db
	os.Exit(m.Run())
}

const vndbSource int16 = 2

func seedEditions(t *testing.T) (jaRelease, enRelease, maker, localiser int64) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_release_label", "catalog_external_ref", "catalog_release", "catalog_label", "catalog_work",
		"src_vndb.releases", "src_vndb.releases_producers",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}

	work := model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "あまいろショコラータ",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
	}
	require.NoError(t, testDB.Create(&work).Error)

	mkRelease := func(lang string) int64 {
		r := model.CatalogRelease{WorkID: work.ID, Kind: 0, Lang: &lang}
		require.NoError(t, testDB.Create(&r).Error)
		return r.ID
	}
	jaRelease, enRelease = mkRelease("ja"), mkRelease("en")

	mkLabel := func(name string) int64 {
		l := model.CatalogLabel{DisplayName: name, Kind: model.LabelKindGameBrand}
		require.NoError(t, testDB.Create(&l).Error)
		return l.ID
	}
	maker, localiser = mkLabel("きゃべつそふと"), mkLabel("Sekai Project")

	anchor := func(entityType int16, id int64, ext string) {
		require.NoError(t, testDB.Exec(
			`INSERT INTO catalog_external_ref (entity_type, entity_id, source_id, external_id, link_kind, matched_by, created_at)
			 VALUES (?, ?, ?, ?, 0, 'rule:test', now())`, entityType, id, vndbSource, ext).Error)
	}
	anchor(model.EntityTypeRelease, jaRelease, "rJA")
	anchor(model.EntityTypeRelease, enRelease, "rEN")
	anchor(model.EntityTypeLabel, maker, "p6193")
	anchor(model.EntityTypeLabel, localiser, "p4859")

	require.NoError(t, testDB.Create(&srcv.Release{ID: "rJA", OLang: "ja", Official: true}).Error)
	require.NoError(t, testDB.Create(&srcv.Release{ID: "rEN", OLang: "en", Official: true}).Error)
	require.NoError(t, testDB.Exec(`INSERT INTO src_vndb.releases_producers (id,pid,developer,publisher) VALUES
		('rJA','p6193',true,true),('rEN','p4859',false,true)`).Error)
	return
}

func kindsOn(t *testing.T, release int64) map[int16]int64 {
	t.Helper()
	var rows []model.CatalogReleaseLabel
	require.NoError(t, testDB.Where("release_id = ?", release).Find(&rows).Error)
	out := map[int16]int64{}
	for _, r := range rows {
		out[r.Kind] = r.LabelID
	}
	return out
}

func TestLocalisationPublisherLandsOnItsOwnRelease(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	ja, en, maker, localiser := seedEditions(t)

	st, err := Run(context.Background(), Opts{Apply: true, DSN: testDSN})
	require.NoError(t, err)
	assert.Equal(t, 3, st.Written, "dev+pub on the original, pub on the localisation")

	jaKinds := kindsOn(t, ja)
	assert.Equal(t, maker, jaKinds[model.WorkLabelKindDeveloper])
	assert.Equal(t, maker, jaKinds[model.WorkLabelKindPublisher])

	enKinds := kindsOn(t, en)
	assert.Equal(t, localiser, enKinds[model.WorkLabelKindPublisher],
		"the English publisher belongs to the English release")
	assert.NotContains(t, enKinds, model.WorkLabelKindDeveloper,
		"publishing an edition is not making it")
	for _, id := range jaKinds {
		assert.NotEqual(t, localiser, id, "the localiser never reaches the original")
	}
}

func TestRerunWritesNothing(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	seedEditions(t)

	_, err := Run(context.Background(), Opts{Apply: true, DSN: testDSN})
	require.NoError(t, err)
	st, err := Run(context.Background(), Opts{Apply: true, DSN: testDSN})
	require.NoError(t, err)
	assert.Zero(t, st.Written)
	assert.Equal(t, 3, st.SkippedDup, "every planned row was already there")
}

func TestDryRunWritesNothingButStillPlans(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	seedEditions(t)

	st, err := Run(context.Background(), Opts{Apply: false, DSN: testDSN})
	require.NoError(t, err)
	assert.Equal(t, 1, st.DevPlanned)
	assert.Equal(t, 2, st.PubPlanned)
	assert.Zero(t, st.Written)

	var n int64
	require.NoError(t, testDB.Model(&model.CatalogReleaseLabel{}).Count(&n).Error)
	assert.Zero(t, n, "a dry run touched the table")
}

func TestUnanchoredProducerIsCountedNotGuessed(t *testing.T) {
	if testDB == nil {
		t.Skip("no test db")
	}
	seedEditions(t)
	require.NoError(t, testDB.Exec(
		`INSERT INTO src_vndb.releases_producers (id,pid,developer,publisher) VALUES ('rJA','p99999',false,true)`).Error)

	st, err := Run(context.Background(), Opts{Apply: true, DSN: testDSN})
	require.NoError(t, err)
	assert.Equal(t, 1, st.Unresolved)
	assert.Equal(t, 3, st.Written, "the unresolvable pair added nothing")
}
