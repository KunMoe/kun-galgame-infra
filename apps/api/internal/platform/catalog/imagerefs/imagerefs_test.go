package imagerefs

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func TestKindsCoversEveryImageColumn(t *testing.T) {
	assert.Equal(t, []string{
		KindWorkCover, KindWorkScreenshot, KindCharacterBust, KindCharacterBustSource,
		KindCharacterFigure, KindCharacterFigureSource, KindLabelLogo, KindPersonPhoto,
	}, Kinds())
}

func TestDetachSetMatchesColumnNullability(t *testing.T) {
	want := map[string]string{
		KindWorkCover: "", KindWorkScreenshot: "",
		KindCharacterBust: "NULL", KindCharacterBustSource: "NULL", KindCharacterFigure: "NULL",
		KindCharacterFigureSource: "NULL",
		KindLabelLogo:             "''", KindPersonPhoto: "''",
	}
	for _, s := range specs {
		assert.Equalf(t, want[s.Kind], s.DetachSet, "detach value for %s", s.Kind)
	}
}

var (
	testOnce  sync.Once
	testDB    *gorm.DB
	testNoDSN bool
	testErr   error
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testOnce.Do(func() {
		dsn, ok := dbtest.DSN()
		if !ok {
			testNoDSN = true
			return
		}
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
		if err != nil {
			testErr = fmt.Errorf("catalog test database unreachable: %w", err)
			return
		}
		if err := migrate.Run(db); err != nil {
			testErr = fmt.Errorf("catalog migrate failed: %w", err)
			return
		}
		if err := seed.Run(db); err != nil {
			testErr = fmt.Errorf("catalog seed failed: %w", err)
			return
		}
		testDB = db
	})
	if testNoDSN {
		dbtest.Skip(t)
	}
	if testErr != nil {
		dbtest.Skipf(t, "%s", testErr)
	}
	return testDB
}

const (
	hashShared = "1111111111111111111111111111111111111111111111111111111111111111"
	hashOther  = "2222222222222222222222222222222222222222222222222222222222222222"
	hashDead   = "3333333333333333333333333333333333333333333333333333333333333333"
)

func fixture(t *testing.T, db *gorm.DB) (workID, charID, labelID, personID int64) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_work_cover", "catalog_work_screenshot", "catalog_work",
		"catalog_character", "catalog_label", "catalog_person",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}

	work := &model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "作品", Status: model.WorkStatusStub}
	require.NoError(t, db.Create(work).Error)
	require.NoError(t, db.Create(&model.CatalogWorkCover{WorkID: work.ID, ImageHash: hashShared, SourceID: 1}).Error)
	require.NoError(t, db.Create(&model.CatalogWorkCover{WorkID: work.ID, ImageHash: hashOther, SourceID: 1}).Error)
	require.NoError(t, db.Create(&model.CatalogWorkScreenshot{WorkID: work.ID, ImageHash: hashShared, SourceID: 1}).Error)

	shared := hashShared
	char := &model.CatalogCharacter{DisplayName: "角色", ImageHash: &shared, ImageSourceHash: &shared, FigureHash: &shared, FigureSourceHash: &shared}
	require.NoError(t, db.Create(char).Error)
	label := &model.CatalogLabel{DisplayName: "会社", LogoHash: hashShared}
	require.NoError(t, db.Create(label).Error)
	person := &model.CatalogPerson{DisplayName: "人物", PhotoHash: hashShared}
	require.NoError(t, db.Create(person).Error)

	deadChar := &model.CatalogCharacter{DisplayName: "亡角色", ImageHash: &shared}
	require.NoError(t, db.Create(deadChar).Error)
	require.NoError(t, db.Delete(deadChar).Error)
	deadLabel := &model.CatalogLabel{DisplayName: "亡会社", LogoHash: hashShared}
	require.NoError(t, db.Create(deadLabel).Error)
	require.NoError(t, db.Delete(deadLabel).Error)
	deadPerson := &model.CatalogPerson{DisplayName: "亡人物", PhotoHash: hashShared}
	require.NoError(t, db.Create(deadPerson).Error)
	require.NoError(t, db.Delete(deadPerson).Error)

	return work.ID, char.ID, label.ID, person.ID
}

func kindCounts(refs []Ref) map[string]int {
	out := map[string]int{}
	for _, r := range refs {
		out[r.Kind]++
	}
	return out
}

func TestCollectSeesEveryKindAndSkipsSoftDeleted(t *testing.T) {
	db := openTestDB(t)
	fixture(t, db)

	refs, err := Collect(context.Background(), db)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{
		KindWorkCover: 2, KindWorkScreenshot: 1, KindCharacterBust: 1, KindCharacterBustSource: 1,
		KindCharacterFigure: 1, KindCharacterFigureSource: 1,
		KindLabelLogo: 1, KindPersonPhoto: 1,
	}, kindCounts(refs))
	for _, r := range refs {
		assert.Emptyf(t, r.Label, "the full sweep skips the label joins (%s)", r.Kind)
	}
}

func TestCollectByHashCarriesLabels(t *testing.T) {
	db := openTestDB(t)
	workID, charID, labelID, personID := fixture(t, db)

	refs, err := CollectByHash(context.Background(), db, hashShared)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{
		KindWorkCover: 1, KindWorkScreenshot: 1, KindCharacterBust: 1, KindCharacterBustSource: 1,
		KindCharacterFigure: 1, KindCharacterFigureSource: 1,
		KindLabelLogo: 1, KindPersonPhoto: 1,
	}, kindCounts(refs))

	byKind := map[string]Ref{}
	for _, r := range refs {
		byKind[r.Kind] = r
	}
	assert.Equal(t, workID, byKind[KindWorkCover].EntityID)
	assert.Equal(t, "作品", byKind[KindWorkCover].Label)
	assert.Equal(t, "作品", byKind[KindWorkScreenshot].Label)
	assert.Equal(t, charID, byKind[KindCharacterBust].EntityID)
	assert.Equal(t, "角色", byKind[KindCharacterFigure].Label)
	assert.Equal(t, labelID, byKind[KindLabelLogo].EntityID)
	assert.Equal(t, personID, byKind[KindPersonPhoto].EntityID)

	none, err := CollectByHash(context.Background(), db, hashDead)
	require.NoError(t, err)
	assert.Empty(t, none, "an unreferenced hash answers with an empty list, not an error")
}

func TestDistinctHashesDedupesAndSorts(t *testing.T) {
	db := openTestDB(t)
	fixture(t, db)

	hashes, err := DistinctHashes(context.Background(), db)
	require.NoError(t, err)
	assert.Equal(t, []string{hashShared, hashOther}, hashes)
}

func TestDetachReleasesEveryKind(t *testing.T) {
	db := openTestDB(t)
	_, charID, labelID, personID := fixture(t, db)
	ctx := context.Background()

	removed, err := Detach(ctx, db, hashShared)
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{
		KindWorkCover: 1, KindWorkScreenshot: 1, KindCharacterBust: 1, KindCharacterBustSource: 1,
		KindCharacterFigure: 1, KindCharacterFigureSource: 1,
		KindLabelLogo: 1, KindPersonPhoto: 1,
	}, removed)

	refs, err := CollectByHash(ctx, db, hashShared)
	require.NoError(t, err)
	assert.Empty(t, refs, "nothing references the hash after a detach")

	hashes, err := DistinctHashes(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, []string{hashOther}, hashes)

	var char model.CatalogCharacter
	require.NoError(t, db.First(&char, charID).Error)
	assert.Nil(t, char.ImageHash)
	assert.Nil(t, char.ImageSourceHash)
	assert.Nil(t, char.FigureHash)
	assert.Nil(t, char.FigureSourceHash)
	var label model.CatalogLabel
	require.NoError(t, db.First(&label, labelID).Error)
	assert.Equal(t, "", label.LogoHash)
	var person model.CatalogPerson
	require.NoError(t, db.First(&person, personID).Error)
	assert.Equal(t, "", person.PhotoHash)

	again, err := Detach(ctx, db, hashShared)
	require.NoError(t, err)
	for kind, n := range again {
		assert.Zerof(t, n, "second detach touched %s", kind)
	}
}
