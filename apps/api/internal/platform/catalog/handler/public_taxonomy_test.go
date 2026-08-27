package handler

import (
	"strings"
	"testing"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func taxonomyApp(db *gorm.DB) *fiber.App {
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/works", h.WorksList)
	app.Get("/v1/catalog/labels", h.LabelsList)
	app.Get("/v1/catalog/tags", h.TagsList)
	app.Get("/v1/catalog/engines", h.EnginesList)
	app.Get("/v1/catalog/engines/:id", h.EngineDetail)
	return app
}

func seedTaxonomy(t *testing.T, db *gorm.DB) (labelID, tagID, engineID int64) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_work_engine", "catalog_engine", "catalog_tag_intro",
		"catalog_work_tag", "catalog_tag_source_map", "catalog_tag",
		"catalog_work_label", "catalog_label",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
	ids := seedPublicWorks(t, db, 1)
	require.NoError(t, db.Exec(
		`UPDATE catalog_work SET site = 'galgame_wiki', product_work_id = 9350, claim_state = ? WHERE id = ?`,
		model.ClaimStateLive, ids[0]).Error)

	label := model.CatalogLabel{DisplayName: "Wire Brand", Kind: model.LabelKindGameBrand, LogoHash: fixtureLogoHash}
	require.NoError(t, db.Create(&label).Error)
	require.NoError(t, db.Create(&model.CatalogWorkLabel{
		WorkID: ids[0], LabelID: label.ID, Kind: model.WorkLabelKindBrand,
	}).Error)

	tag := model.CatalogTag{Name: "wire-tag", Tier: model.TagTierCore, Kind: model.TagKindContent}
	require.NoError(t, db.Create(&tag).Error)

	engine := model.CatalogEngine{Name: "WireEngine", Description: "", Aliases: []byte("[]")}
	require.NoError(t, db.Create(&engine).Error)
	require.NoError(t, db.Create(&model.CatalogWorkEngine{
		WorkID: ids[0], EngineID: engine.ID, SourceID: 2,
	}).Error)

	return label.ID, tag.ID, engine.ID
}

func TestTaxonomyClosedVocabularies(t *testing.T) {
	db := openCatalogTestDB(t)
	seedTaxonomy(t, db)
	app := taxonomyApp(db)

	cases := []struct{ name, url, msg string }{
		{"label kind unknown", "/v1/catalog/labels?kind=studio", msgBadLabelKind},
		{"label kind numeric", "/v1/catalog/labels?kind=0", msgBadLabelKind},
		{"label kind uppercase", "/v1/catalog/labels?kind=GAME_BRAND", msgBadLabelKind},
		{"label kind other", "/v1/catalog/labels?kind=other", msgBadLabelKind},
		{"tag tier unknown", "/v1/catalog/tags?tier=popular", msgBadTagTier},
		{"tag tier numeric", "/v1/catalog/tags?tier=1", msgBadTagTier},
		{"tag kind unknown", "/v1/catalog/tags?kind=platform", msgBadTagKind},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := getJSON(t, app, tc.url)
			require.Equal(t, 400, code)
			assert.Equal(t, tc.msg, body["message"])
		})
	}

	for _, url := range []string{
		"/v1/catalog/labels", "/v1/catalog/labels?kind=game_brand", "/v1/catalog/labels?kind=bunko",
		"/v1/catalog/labels?kind=publisher", "/v1/catalog/labels?kind=anime_studio",
		"/v1/catalog/labels?kind=doujin_circle", "/v1/catalog/labels?kind=group",
		"/v1/catalog/tags", "/v1/catalog/tags?tier=core", "/v1/catalog/tags?tier=longtail",
		"/v1/catalog/tags?tier=hidden", "/v1/catalog/tags?kind=content", "/v1/catalog/tags?kind=meta",
		"/v1/catalog/tags?tier=core&kind=content", "/v1/catalog/engines",
	} {
		t.Run("legal "+url, func(t *testing.T) {
			code, _ := getJSON(t, app, url)
			assert.Equal(t, 200, code)
		})
	}
}

func TestTaxonomyLimitAndCursorSemantics(t *testing.T) {
	db := openCatalogTestDB(t)
	seedTaxonomy(t, db)
	app := taxonomyApp(db)

	for _, url := range []string{
		"/v1/catalog/labels?limit=abc", "/v1/catalog/labels?limit=0", "/v1/catalog/labels?limit=-1",
		"/v1/catalog/tags?limit=abc", "/v1/catalog/tags?limit=0",
		"/v1/catalog/engines?limit=abc", "/v1/catalog/engines?limit=0",
	} {
		t.Run(url, func(t *testing.T) {
			code, body := getJSON(t, app, url)
			require.Equal(t, 400, code)
			assert.Equal(t, msgBadLimit, body["message"])
		})
	}
	for _, url := range []string{
		"/v1/catalog/labels?cursor=!!!nope!!!",
		"/v1/catalog/tags?cursor=!!!nope!!!",
		"/v1/catalog/engines?cursor=!!!nope!!!",
	} {
		t.Run(url, func(t *testing.T) {
			code, body := getJSON(t, app, url)
			require.Equal(t, 400, code)
			assert.Equal(t, msgBadCursor, body["message"])
		})
	}
}

func TestTaxonomyItemsCarryWorkCount(t *testing.T) {
	db := openCatalogTestDB(t)
	labelID, tagID, engineID := seedTaxonomy(t, db)
	app := taxonomyApp(db)

	code, body := getJSON(t, app, "/v1/catalog/labels")
	require.Equal(t, 200, code)
	items := body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 1)
	row := items[0].(map[string]any)
	assert.EqualValues(t, labelID, row["id"])
	assert.Equal(t, "Wire Brand", row["display_name"])
	assert.Equal(t, "game_brand", row["kind"])
	assert.EqualValues(t, 1, row["work_count"])
	assert.Equal(t, fixtureLogoHash, row["logo_hash"])
	assert.Equal(t, false, row["has_relations"])
	assert.Nil(t, body["data"].(map[string]any)["next_cursor"])

	parent := model.CatalogLabel{DisplayName: "Wire Holdings", Kind: model.LabelKindPublisher}
	require.NoError(t, db.Create(&parent).Error)
	require.NoError(t, db.Create(&model.CatalogLabelRelation{
		LabelID: labelID, OtherLabelID: parent.ID, Relation: model.LabelRelationParent,
		SourceID: 1, MatchedBy: "rule:test",
	}).Error)
	code, body = getJSON(t, app, "/v1/catalog/labels")
	require.Equal(t, 200, code)
	items = body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, true, items[0].(map[string]any)["has_relations"], "the label with the edge")
	assert.Equal(t, false, items[1].(map[string]any)["has_relations"],
		"the other end is reached by the MIRROR row, which this fixture does not write")

	code, body = getJSON(t, app, "/v1/catalog/tags")
	require.Equal(t, 200, code)
	row = body["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
	assert.EqualValues(t, tagID, row["id"])
	assert.Equal(t, "core", row["tier"])
	assert.Equal(t, "content", row["kind"])
	assert.EqualValues(t, 0, row["work_count"], "an unmapped canonical tag reaches no work")

	code, body = getJSON(t, app, "/v1/catalog/engines?nsfw=1")
	require.Equal(t, 200, code)
	row = body["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
	assert.EqualValues(t, engineID, row["id"])
	assert.Equal(t, "WireEngine", row["name"])
	assert.EqualValues(t, 1, row["work_count"])
}

func TestEngineDetailWire(t *testing.T) {
	db := openCatalogTestDB(t)
	_, _, engineID := seedTaxonomy(t, db)
	app := taxonomyApp(db)

	code, _ := getJSON(t, app, "/v1/catalog/engines/not-a-number")
	assert.Equal(t, 400, code)

	code, _ = getJSON(t, app, "/v1/catalog/engines/999999")
	assert.Equal(t, 404, code)

	code, body := getJSON(t, app, "/v1/catalog/engines/"+itoa(engineID))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)
	assert.Equal(t, "WireEngine", data["name"])
	assert.EqualValues(t, 1, data["work_count"])
	assert.Equal(t, []any{}, data["refs"], "refs is always present, [] when the engine has no anchor")
}

func TestTagsListIDsFilter(t *testing.T) {
	db := openCatalogTestDB(t)
	_, coreTagID, _ := seedTaxonomy(t, db)
	app := taxonomyApp(db)

	longtail := model.CatalogTag{Name: "wire-longtail", Tier: model.TagTierLongtail, Kind: model.TagKindContent}
	require.NoError(t, db.Create(&longtail).Error)
	meta := model.CatalogTag{Name: "wire-meta", Tier: model.TagTierCore, Kind: model.TagKindMeta}
	require.NoError(t, db.Create(&meta).Error)

	tagPage := func(t *testing.T, url string) (names []string, total float64) {
		t.Helper()
		code, body := getJSON(t, app, url)
		require.Equal(t, 200, code)
		data := body["data"].(map[string]any)
		for _, it := range data["items"].([]any) {
			names = append(names, it.(map[string]any)["name"].(string))
		}
		return names, data["total"].(float64)
	}

	names, total := tagPage(t, "/v1/catalog/tags?ids="+itoa(coreTagID)+","+itoa(longtail.ID))
	assert.Equal(t, []string{"wire-tag", "wire-longtail"}, names)
	assert.EqualValues(t, 2, total, "total converges with the ids filter like every other predicate")

	names, total = tagPage(t, "/v1/catalog/tags?ids="+itoa(coreTagID)+","+itoa(longtail.ID)+"&tier=core")
	assert.Equal(t, []string{"wire-tag"}, names, "ids is conjunctive with tier, not a bypass")
	assert.EqualValues(t, 1, total)

	names, total = tagPage(t, "/v1/catalog/tags?ids="+itoa(coreTagID)+",999999")
	assert.Equal(t, []string{"wire-tag"}, names, "an unknown id matches nothing and is not an error")
	assert.EqualValues(t, 1, total)

	for _, raw := range []string{"abc", "0", "-5", "1.5", "1,,2", itoa(coreTagID) + ",abc"} {
		t.Run("400 ids="+raw, func(t *testing.T) {
			code, body := getJSON(t, app, "/v1/catalog/tags?ids="+raw)
			require.Equal(t, 400, code)
			assert.Equal(t, "ids must be positive integers", body["message"])
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/tags?ids="+repeatIDs(101))
	require.Equal(t, 400, code)
	assert.Equal(t, "at most 100 ids", body["message"])

	code, _ = getJSON(t, app, "/v1/catalog/tags?ids="+repeatIDs(100))
	assert.Equal(t, 200, code, "100 ids is the ceiling, not one past it")
}

func repeatIDs(n int) string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = itoa(int64(i + 1))
	}
	return strings.Join(ids, ",")
}

func TestWorksListEngineIDFilter(t *testing.T) {
	db := openCatalogTestDB(t)
	_, _, engineID := seedTaxonomy(t, db)
	app := taxonomyApp(db)

	for _, raw := range []string{"abc", "0", "-5", "1.5"} {
		t.Run("engine_id="+raw, func(t *testing.T) {
			code, body := getJSON(t, app, "/v1/catalog/works?engine_id="+raw)
			require.Equal(t, 400, code)
			assert.Equal(t, "engine_id must be a positive integer", body["message"])
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/works?engine_id="+itoa(engineID))
	require.Equal(t, 200, code)
	assert.Len(t, body["data"].(map[string]any)["items"], 1)

	code, body = getJSON(t, app, "/v1/catalog/works?engine_id=999999")
	require.Equal(t, 200, code)
	assert.Empty(t, body["data"].(map[string]any)["items"], "no work uses engine 999999")
}
