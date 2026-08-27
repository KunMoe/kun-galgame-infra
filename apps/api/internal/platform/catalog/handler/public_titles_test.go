// public_titles_test.go — A2-R1 区 A wire-level cases (refs/proj/136): the
// claimed work's TITLE face end to end.
//
// Originally an A/B harness over the wiki title bridge, then over the wave-140
// mirror that replaced it. Wave 161 retired both the wiki galgame / galgame_alias
// layer and the mirror, so these are now plain CONTRACT tests over native
// catalog_work_title rows: the fixtures below ARE the rows the mirror produced
// for the old wiki bodies, which is why the frozen expectations never moved.
//
// The regression the original wave fixed: 87% of claimed works had ZERO
// catalog_work_title rows, so their Chinese names and aliases were absent from
// every consumer. The cases pin the resulting shape — the four-key pivot, aliases
// as lang-less alias rows, and byte-identity for a bodyless work.
package handler

import (
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func titleRows(t *testing.T, body map[string]any) [][3]string {
	t.Helper()
	raw, ok := body["data"].(map[string]any)["titles"].([]any)
	require.True(t, ok, "titles[] must be present")
	out := make([][3]string, 0, len(raw))
	for _, r := range raw {
		m := r.(map[string]any)
		out = append(out, [3]string{m["lang"].(string), m["title"].(string), m["kind"].(string)})
	}
	return out
}

func TestClaimedWorkTitlesBridge(t *testing.T) {
	db := openCatalogTestDB(t)
	ensureGalgameStub(t, db)
	ensureGalgameRatingStub(t, db)
	for _, tbl := range []string{"edit_suppressed_row", "catalog_work_title", "catalog_work"} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}

	claimed := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "認領作品", ContentRating: 0, Status: 0,
		Site: strptr("galgame_wiki"), ProductWorkID: ptrI64(5001)}
	require.NoError(t, db.Create(&claimed).Error)
	for _, row := range []model.CatalogWorkTitle{
		{WorkID: claimed.ID, Lang: "ja", Title: "日本語名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "en", Title: "English Name", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "zh-Hans", Title: "简体中文名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "zh-Hant", Title: "繁體中文名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Title: "べつめい", Kind: model.WorkTitleKindAlias},
	} {
		require.NoError(t, db.Create(&row).Error)
	}

	bodyless := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "無体作品", ContentRating: 0, Status: 0}
	require.NoError(t, db.Create(&bodyless).Error)
	latin := "Mutai Sakuhin"
	require.NoError(t, db.Create(&model.CatalogWorkTitle{
		WorkID: bodyless.ID, Lang: "ja", Title: "無体作品", Latin: &latin, Kind: model.WorkTitleKindOfficial,
	}).Error)
	require.NoError(t, db.Create(&model.CatalogWorkTitle{
		WorkID: bodyless.ID, Lang: "ja", Title: "けんさくヒント", Kind: model.WorkTitleKindSearchHint,
	}).Error)

	app := supplyApp(db)

	code, body := getJSON(t, app, "/v1/catalog/works/"+itoa(claimed.ID))
	require.Equal(t, 200, code)
	assert.Equal(t, [][3]string{
		{"en", "English Name", "official"},
		{"ja", "日本語名", "official"},
		{"zh-Hans", "简体中文名", "official"},
		{"zh-Hant", "繁體中文名", "official"},
		{"", "べつめい", "alias"},
	}, titleRows(t, body), "one row per name, (kind, lang) ordered")

	code, body = getJSON(t, app, "/v1/catalog/works/"+itoa(bodyless.ID))
	require.Equal(t, 200, code)
	assert.Equal(t, [][3]string{{"ja", "無体作品", "official"}}, titleRows(t, body),
		"a bodyless work still reads its native rows, search hints excluded")
	first := body["data"].(map[string]any)["titles"].([]any)[0].(map[string]any)
	assert.Equal(t, "Mutai Sakuhin", first["latin"], "native latin survives the shared row shape")
}

func TestClaimedWorkNamesBlockBridged(t *testing.T) {
	db := openCatalogTestDB(t)
	ensureGalgameStub(t, db)
	ensureGalgameRatingStub(t, db)
	for _, tbl := range []string{"edit_suppressed_row", "catalog_work_title", "catalog_work"} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}

	claimed := model.CatalogWork{MediumID: 1, OLang: "ja", DisplayName: "認領一覧", ContentRating: 0, Status: 0,
		Site: strptr("galgame_wiki"), ProductWorkID: ptrI64(5002)}
	require.NoError(t, db.Create(&claimed).Error)
	for _, row := range []model.CatalogWorkTitle{
		{WorkID: claimed.ID, Lang: "ja", Title: "日本語名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "en", Title: "English Name", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "zh-Hans", Title: "简体中文名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Lang: "zh-Hant", Title: "繁體中文名", Kind: model.WorkTitleKindOfficial},
		{WorkID: claimed.ID, Title: "べつめい", Kind: model.WorkTitleKindAlias},
	} {
		require.NoError(t, db.Create(&row).Error)
	}

	app := supplyApp(db)
	code, body := getJSON(t, app, "/v1/catalog/works?include=names")
	require.Equal(t, 200, code)
	items := body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 1)
	names := items[0].(map[string]any)["localized"].(map[string]any)
	value := func(locale string) any {
		row, _ := names[locale].(map[string]any)
		return row["value"]
	}
	assert.Equal(t, "日本語名", value("ja"))
	assert.Equal(t, "English Name", value("en"))
	assert.Equal(t, "简体中文名", value("zh-Hans"))
	assert.Equal(t, "繁體中文名", value("zh-Hant"))
	assert.Len(t, names, 4, "a lang-less alias answers no locale")
}
