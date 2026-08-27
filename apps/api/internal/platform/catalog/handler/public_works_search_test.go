package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func worksSearchApp(db *gorm.DB) *fiber.App {
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/works/search", h.WorksSearch)
	return app
}

func TestWorksSearchParamValidation(t *testing.T) {
	db := openCatalogTestDB(t)
	app := worksSearchApp(db)

	rejected := []struct{ url, msg string }{
		{"/v1/catalog/works/search?sort=view", msgBadSearchSort},
		{"/v1/catalog/works/search?sort=nonsense", msgBadSearchSort},
		{"/v1/catalog/works/search?facets=bogus", msgBadSearchFacet},
		{"/v1/catalog/works/search?facets=content_rating,bogus", msgBadSearchFacet},
		{"/v1/catalog/works/search?facets=tag_ids", msgBadSearchFacet},
		{"/v1/catalog/works/search?page=0", msgBadPage},
		{"/v1/catalog/works/search?page=-3", msgBadPage},
		{"/v1/catalog/works/search?page=abc", msgBadPage},
		{"/v1/catalog/works/search?limit=0", msgBadLimit},
		{"/v1/catalog/works/search?limit=abc", msgBadLimit},
		{"/v1/catalog/works/search?tag_id=0", "tag_id must be up to 10 comma-separated positive integers"},
		{"/v1/catalog/works/search?label_id=-1", "label_id must be a positive integer"},
		{"/v1/catalog/works/search?engine_id=abc", "engine_id must be a positive integer"},
		{"/v1/catalog/works/search?series_id=x", "series_id must be a positive integer"},
		{"/v1/catalog/works/search?released_after=2024", "released_after must be YYYY-MM-DD"},
		{"/v1/catalog/works/search?released_before=nope", "released_before must be YYYY-MM-DD"},
		{"/v1/catalog/works/search?content_rating=adult", "content_rating must be all_ages|sensitive|r18"},
		{"/v1/catalog/works/search?claimed=maybe", "claimed must be true|false"},
		{"/v1/catalog/works/search?claim_state=liev", msgBadClaimState},
		{"/v1/catalog/works/search?claim_state=LIVE", msgBadClaimState},
		{"/v1/catalog/works/search?claim_state=published", msgBadClaimState},
		{"/v1/catalog/works/search?claim_state=live,bogus", msgBadClaimState},
		{"/v1/catalog/works/search?claim_state=true", msgBadClaimState},
		{"/v1/catalog/works/search?content_rating=r18", "content_rating=r18 requires nsfw=1"},
	}
	for _, c := range rejected {
		code, body := getJSON(t, app, c.url)
		assert.Equalf(t, fiber.StatusBadRequest, code, "%s must 400", c.url)
		assert.Equalf(t, c.msg, body["message"], "%s message", c.url)
	}
}

func TestWorksSearchOpenVocabulariesDoNotReject(t *testing.T) {
	db := openCatalogTestDB(t)
	app := worksSearchApp(db)

	for _, url := range []string{
		"/v1/catalog/works/search?olang=klingon",
		"/v1/catalog/works/search?olang=all",
		"/v1/catalog/works/search?claim_state=live",
		"/v1/catalog/works/search?claim_state=none,live,draft,hidden",
		"/v1/catalog/works/search?include=names,bogus",
		"/v1/catalog/works/search?sort=relevance&facets=content_rating,olang,claimed,tag_id,label_id,engine_id,series_id,source",
		"/v1/catalog/works/search?page=99999&limit=100",
	} {
		code, _ := getJSON(t, app, url)
		assert.NotEqualf(t, fiber.StatusBadRequest, code, "%s must not 400", url)
	}
}

func TestWorksSearchWithoutIndexerIs500(t *testing.T) {
	db := openCatalogTestDB(t)
	code, _ := getJSON(t, worksSearchApp(db), "/v1/catalog/works/search?q=x")
	assert.Equal(t, fiber.StatusInternalServerError, code)
}

func TestPageNumPub(t *testing.T) {
	n, ok := pageNumPub("")
	assert.True(t, ok)
	assert.Equal(t, 1, n, "absent page means the first page")
	n, ok = pageNumPub("  7 ")
	assert.True(t, ok)
	assert.Equal(t, 7, n)
	for _, bad := range []string{"0", "-1", "abc", "1.5"} {
		_, ok := pageNumPub(bad)
		assert.Falsef(t, ok, "page=%q must be rejected", bad)
	}
	n, ok = pageNumPub("1000000")
	assert.True(t, ok)
	assert.Equal(t, 1000000, n)
}

func TestWorksSearchOLangDefaultIsUngated(t *testing.T) {
	assert.Equal(t, service.PublicOLang{All: true}, worksSearchOLang(""),
		"omitted olang on the search lane = no gate")
	assert.Equal(t, service.PublicOLang{All: true}, worksSearchOLang("   "))
	assert.Equal(t, service.PublicOLang{All: true}, worksSearchOLang(" , , "))
	assert.Equal(t, service.PublicOLang{}, parsePublicOLang(""))
	assert.Equal(t, service.PublicOLang{}, parsePublicOLang(" , , "))

	for _, raw := range []string{"all", "ja", " ja , zh-Hans ", "xx-Nope"} {
		assert.Equalf(t, parsePublicOLang(raw), worksSearchOLang(raw),
			"olang=%q must mean the same thing on both lanes", raw)
	}
	assert.Equal(t, service.PublicOLang{All: true}, worksSearchOLang("all"))
	assert.Equal(t, service.PublicOLang{Values: []string{"en"}}, worksSearchOLang("en"))

	assert.Equal(t, "all", worksSearchOLang("").Key())
	assert.Equal(t, "jazh", parsePublicOLang("").Key())
	assert.NotEqual(t, parsePublicOLang("").Key(), worksSearchOLang("").Key())
	assert.Equal(t, "sfw-jazh-all",
		service.CalendarFilter{OLang: parsePublicOLang("")}.PopulationKey())
}

func TestPublicSearchIndexTagsType(t *testing.T) {
	for typ, wantEntity := range map[string]string{
		"names": "name", "characters": "character", "labels": "label",
		"works": "work", "tags": "tag",
	} {
		uid, entity, ok := publicSearchIndex(typ)
		require.Truef(t, ok, "type=%s must resolve", typ)
		assert.NotEmpty(t, uid)
		assert.Equalf(t, wantEntity, entity, "type=%s entity_type", typ)
	}
	_, _, ok := publicSearchIndex("engines")
	assert.False(t, ok, "the vocabulary stays closed — engines is not a search type")
	_, _, ok = publicSearchIndex("")
	assert.False(t, ok, "an absent type is still a 400, as before")
}

func TestEntityHitShapeFrozenForNonTagFamilies(t *testing.T) {
	for _, entity := range []string{"name", "character", "label", "work"} {
		hit := dto.PublicEntityHit{
			ID: 7, EntityType: entity, DisplayName: "テスト", Sources: []string{"vndb:v1"},
		}
		if entity == "work" {
			hit.ContentRating = "all_ages"
		}
		raw, err := json.Marshal(hit)
		require.NoError(t, err)
		assert.NotContainsf(t, string(raw), `"tier"`, "%s hit gained a tier key", entity)
		assert.NotContainsf(t, string(raw), `"kind"`, "%s hit gained a kind key", entity)
	}
	raw, err := json.Marshal(dto.PublicEntityHit{
		ID: 3, EntityType: "tag", DisplayName: "純愛", Sources: []string{}, Tier: "core", Kind: "content",
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"tier":"core"`)
	assert.Contains(t, string(raw), `"kind":"content"`)
}

func TestPublicTagKeyProjections(t *testing.T) {
	assert.Equal(t, "core", publicTagTierKey(0))
	assert.Equal(t, "longtail", publicTagTierKey(1))
	assert.Equal(t, "hidden", publicTagTierKey(2))
	assert.Equal(t, "core", publicTagTierKey(99), "an out-of-vocabulary tier falls back, never blank")
	assert.Equal(t, "content", publicTagKindKey(0))
	assert.Equal(t, "meta", publicTagKindKey(1))
}

func TestWorksSearchEnvelopeShape(t *testing.T) {
	raw, err := json.Marshal(dto.PublicWorksSearchData{
		Total: 42, Page: 2, Limit: 20, Items: []dto.PublicWorkListItem{},
	})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	for _, k := range []string{"total", "page", "limit", "items"} {
		assert.Containsf(t, got, k, "envelope must carry %s", k)
	}
	assert.NotContains(t, got, "facets")

	withFacets, err := json.Marshal(dto.PublicWorksSearchData{
		Facets: map[string]map[string]int64{"content_rating": {"all_ages": 3}},
	})
	require.NoError(t, err)
	assert.Contains(t, string(withFacets), `"facets":{"content_rating":{"all_ages":3}}`)
}

func TestWorksSearchRouteOrder(t *testing.T) {
	db := openCatalogTestDB(t)
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/works", h.WorksList)
	app.Get("/v1/catalog/works/search", h.WorksSearch)
	app.Get("/v1/catalog/works/:id", h.WorkDetail)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/catalog/works/search?q=x", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode,
		"/works/search must not be swallowed by /works/:id")

	code, _ := getJSON(t, app, "/v1/catalog/works/999999999")
	assert.Equal(t, fiber.StatusNotFound, code)
}
