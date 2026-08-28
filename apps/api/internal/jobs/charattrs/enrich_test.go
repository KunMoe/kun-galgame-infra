package charattrs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	srcb "api/internal/platform/catalog/srcbangumi"
	srcv "api/internal/platform/catalog/srcvndb"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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
		dbtest.SkipMain("jobs/charattrs")
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("jobs/charattrs", "cannot connect to test database: %v", err)
	}
	for _, step := range []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"catalog migrate", migrate.Run}, {"catalog seed", seed.Run},
		{"src_bangumi schema", srcb.EnsureSchema}, {"src_vndb schema", srcv.EnsureSchema},
	} {
		if err := step.fn(db); err != nil {
			dbtest.SkipMainf("jobs/charattrs", "%s failed: %v", step.name, err)
		}
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"catalog_external_ref", "catalog_character",
		"src_bangumi.character", "src_vndb.chars",
	} {
		require.NoError(t, testDB.Exec("TRUNCATE "+table+" RESTART IDENTITY CASCADE").Error)
	}
}

func mkChar(t *testing.T, name string) int64 {
	t.Helper()
	c := model.CatalogCharacter{DisplayName: name}
	require.NoError(t, testDB.Create(&c).Error)
	return c.ID
}

func mkAnchor(t *testing.T, entityID int64, source int16, externalID, matchedBy string, kind int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeCharacter, EntityID: entityID, SourceID: source,
		ExternalID: externalID, LinkKind: kind, MatchedBy: matchedBy,
	}).Error)
}

func mkVNDB(t *testing.T, id, sex, bloodt, cup string, birthday, height, bust, waist, hip int16, weight *int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcv.Char{
		ID: id, Sex: sex, BloodT: bloodt, CupSize: cup, Birthday: birthday, Height: height,
		SBust: bust, SWaist: waist, SHip: hip, Weight: weight, Description: "", IngestedAt: time.Now(),
	}).Error)
}

func mkBGM(t *testing.T, id int64, infobox string) {
	t.Helper()
	require.NoError(t, testDB.Create(&srcb.Character{
		ID: id, Role: 1, Name: fmt.Sprintf("char-%d", id), InfoboxRaw: "",
		InfoboxParsed: datatypes.JSON(infobox), ParseError: "", Summary: "",
		ParserVersion: srcb.ParserVersion, IngestedAt: time.Now(),
	}).Error)
}

func reg(t *testing.T) registry {
	t.Helper()
	r, err := resolveRegistry(context.Background(), testDB)
	require.NoError(t, err)
	return r
}

func loadChar(t *testing.T, id int64) charState {
	t.Helper()
	states, err := preloadStates(context.Background(), testDB, []int64{id})
	require.NoError(t, err)
	return states[id]
}

func bgmExtra(t *testing.T, id int64) map[string]any {
	t.Helper()
	cs := loadChar(t, id)
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(cs.Extra, &doc))
	ns, ok := doc["bgm"]
	require.True(t, ok, "character %d has no bgm extra namespace", id)
	var out map[string]any
	require.NoError(t, json.Unmarshal(ns, &out))
	return out
}

func provSource(t *testing.T, id int64, col string) string {
	t.Helper()
	var s string
	require.NoError(t, testDB.Raw(
		`SELECT field_provenance->?->0->>'source' FROM catalog_character WHERE id = ?`, col, id).Scan(&s).Error)
	return s
}

func i16(v int16) *int16 { return &v }

func TestBothLanesSurvivorshipAndIdempotency(t *testing.T) {
	clean(t)
	ctx := context.Background()
	r := reg(t)

	chBoth := mkChar(t, "both")
	mkVNDB(t, "c1", "f", "unknown", "", 705, 160, 0, 0, 0, nil)
	mkBGM(t, 1, `{"Fields":[
		{"Key":"性别","Value":"男"},{"Key":"生日","Value":"6月17日"},{"Key":"身高","Value":"999cm"},
		{"Key":"血型","Value":"A型"},{"Key":"星座","Value":"巨蟹座"}]}`)
	mkAnchor(t, chBoth, r.vndbSource, "c1", "rule:vndb-character-import", model.LinkKindExact)
	mkAnchor(t, chBoth, r.bangumiSource, "1", "rule:bangumi-character-import", model.LinkKindExact)

	chBgm := mkChar(t, "bgm-only")
	mkBGM(t, 2, `{"Fields":[
		{"Key":"性别","Value":"女"},{"Key":"体重","Value":"48kg"},{"Key":"BWH","Value":"B85(E)/W58/H86"},
		{"Key":"CV","Value":"someVA"},{"Key":"别名","Value":"nick"},
		{"Key":"能力","Value":"","Array":true,"Items":[{"Key":"","Value":"飞行"},{"Key":"","Value":"隐身"}]}]}`)
	mkAnchor(t, chBgm, r.bangumiSource, "2", "rule:bgm-type4-gated", model.LinkKindExact)

	chVndb := mkChar(t, "vndb-only")
	mkVNDB(t, "c3", "m", "o", "d", 1224, 175, 90, 60, 88, i16(65))
	mkAnchor(t, chVndb, r.vndbSource, "c3", "rule:same-work-character-name", model.LinkKindExact)

	chProb := mkChar(t, "probable")
	mkVNDB(t, "c4", "f", "a", "", 0, 0, 0, 0, 0, nil)
	mkAnchor(t, chProb, r.vndbSource, "c4", "rule:same-work-character-name", model.LinkKindProbable)

	st, err := Run(ctx, Opts{DSN: testDSN})
	require.NoError(t, err)
	assert.Equal(t, 2, st.VNDB.Candidates)
	assert.Equal(t, 2, st.Bangumi.Candidates, "chBoth + chBgm")
	assert.Positive(t, st.VNDB.RowsUpdated, "dry decides a plan")
	assert.Nil(t, loadChar(t, chBoth).Gender, "dry writes nothing")

	_, err = Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)

	a := loadChar(t, chBoth)
	assert.Equal(t, model.GenderFemale, deref(a.Gender), "vndb sex=f wins over bgm 男")
	assert.Equal(t, int16(7), deref(a.Month), "vndb birthday wins")
	assert.Equal(t, int16(5), deref(a.Day))
	assert.Equal(t, int16(160), deref(a.Height), "vndb height wins; bgm 999 never lands")
	assert.Equal(t, model.BloodTypeA, deref(a.Blood), "bgm fills the gap vndb lacks")
	assert.Equal(t, sourceVNDB, provSource(t, chBoth, "gender"))
	assert.Equal(t, sourceVNDB, provSource(t, chBoth, "height_cm"))
	assert.Equal(t, sourceBangumi, provSource(t, chBoth, "blood_type"))
	aExtra := bgmExtra(t, chBoth)
	assert.Equal(t, "巨蟹座", aExtra["星座"])
	assert.Equal(t, "999cm", aExtra["身高"], "out-of-range raw preserved")
	assert.NotContains(t, aExtra, "生日", "clean 6月17日 fully consumed")

	b := loadChar(t, chBgm)
	assert.Equal(t, model.GenderFemale, deref(b.Gender))
	assert.Equal(t, int16(48), deref(b.Weight))
	assert.Equal(t, int16(85), deref(b.Bust))
	assert.Equal(t, "E", derefS(b.Cup))
	assert.Equal(t, sourceBangumi, provSource(t, chBgm, "gender"))
	bExtra := bgmExtra(t, chBgm)
	assert.Equal(t, []any{"飞行", "隐身"}, bExtra["能力"], "Array folded")
	assert.NotContains(t, bExtra, "CV")
	assert.NotContains(t, bExtra, "别名")

	c := loadChar(t, chVndb)
	assert.Equal(t, model.GenderMale, deref(c.Gender))
	assert.Equal(t, int16(12), deref(c.Month))
	assert.Equal(t, "D", derefS(c.Cup))
	assert.Equal(t, int16(90), deref(c.Bust))

	assert.Nil(t, loadChar(t, chProb).Gender)

	st2, err := Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st2.VNDB.RowsUpdated, "vndb lane second-pass zero-write")
	assert.Zero(t, st2.Bangumi.RowsUpdated, "bgm lane second-pass zero-write")
	assert.Zero(t, st2.VNDB.Errors+st2.Bangumi.Errors)
}

func TestUserEditProtected(t *testing.T) {
	clean(t)
	ctx := context.Background()
	r := reg(t)

	ch := mkChar(t, "user-owned")
	require.NoError(t, testDB.Exec(
		`UPDATE catalog_character SET gender = ?, field_provenance = ? WHERE id = ?`,
		model.GenderOther, datatypes.JSON(`{"gender":[{"source":"user","at":"2026-07-01T00:00:00Z"}]}`), ch).Error)
	mkVNDB(t, "c1", "f", "", "", 0, 0, 0, 0, 0, nil)
	mkAnchor(t, ch, r.vndbSource, "c1", "rule:vndb-character-import", model.LinkKindExact)

	_, err := Run(ctx, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	assert.Equal(t, model.GenderOther, deref(loadChar(t, ch).Gender), "user edit untouched")
	assert.Equal(t, "user", provSource(t, ch, "gender"))
}

func TestUnknownLaneRejected(t *testing.T) {
	_, err := Run(context.Background(), Opts{DSN: testDSN, Only: "bogus"})
	require.Error(t, err)
}
