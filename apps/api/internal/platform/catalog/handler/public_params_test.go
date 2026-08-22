package handler

import (
	"encoding/json"
	"net/http/httptest"
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

func publicApp(db *gorm.DB) *fiber.App {
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/works", h.WorksList)
	app.Get("/v1/catalog/changes", h.Changes)
	app.Get("/v1/catalog/labels/:id", h.Label)
	app.Get("/v1/catalog/labels/:id/relation-graph", h.LabelRelationGraph)
	app.Get("/v1/catalog/names/:id", h.Name)
	app.Get("/v1/catalog/characters/:id", h.Character)
	app.Get("/v1/catalog/lookup", h.Lookup)
	app.Post("/v1/catalog/lookup/batch", h.LookupBatch)
	return app
}

func postJSON(t *testing.T, app *fiber.App, url, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func seedPublicWorks(t *testing.T, db *gorm.DB, n int) []int64 {
	t.Helper()
	for _, tbl := range []string{
		"catalog_credit", "catalog_work_character", "catalog_work_label", "catalog_external_ref",
		"edit_suppressed_row", "catalog_work_title", "catalog_release", "catalog_work",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		w := model.CatalogWork{
			MediumID: 1, OLang: "ja", DisplayName: "限界作品" + itoa(int64(i)),
			ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		}
		require.NoError(t, db.Create(&w).Error)
		ids = append(ids, w.ID)
	}
	return ids
}

func TestPublicWorksListStrictIDFilters(t *testing.T) {
	db := openCatalogTestDB(t)
	seedPublicWorks(t, db, 3)
	app := publicApp(db)

	cases := []struct{ name, url, msg string }{
		{"label_id non numeric", "/v1/catalog/works?label_id=abc", "label_id must be a positive integer"},
		{"label_id fractional", "/v1/catalog/works?label_id=1.5", "label_id must be a positive integer"},
		{"tag_id zero", "/v1/catalog/works?tag_id=0", "tag_id must be up to 10 comma-separated positive integers"},
		{"series_id negative", "/v1/catalog/works?series_id=-5", "series_id must be a positive integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := getJSON(t, app, tc.url)
			require.Equal(t, 400, code)
			assert.Equal(t, tc.msg, body["message"])
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/works")
	require.Equal(t, 200, code)
	assert.Len(t, body["data"].(map[string]any)["items"], 3)

	code, body = getJSON(t, app, "/v1/catalog/works?label_id=999999")
	require.Equal(t, 200, code)
	assert.Empty(t, body["data"].(map[string]any)["items"])
}

func TestPublicWorksListIDsFilter(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedPublicWorks(t, db, 3)
	app := publicApp(db)

	listedIDs := func(t *testing.T, url string) (out []int64) {
		t.Helper()
		code, body := getJSON(t, app, url)
		require.Equal(t, 200, code)
		for _, it := range body["data"].(map[string]any)["items"].([]any) {
			out = append(out, int64(it.(map[string]any)["id"].(float64)))
		}
		return out
	}

	assert.Equal(t, []int64{ids[0], ids[2]},
		listedIDs(t, "/v1/catalog/works?ids="+itoa(ids[0])+","+itoa(ids[2])))
	assert.Equal(t, []int64{ids[1]},
		listedIDs(t, "/v1/catalog/works?ids="+itoa(ids[1])+",999999"),
		"an unknown id matches nothing and is not an error")

	for _, raw := range []string{"abc", "0", "-5", "1,,2"} {
		t.Run("400 ids="+raw, func(t *testing.T) {
			code, body := getJSON(t, app, "/v1/catalog/works?ids="+raw)
			require.Equal(t, 400, code)
			assert.Equal(t, "ids must be positive integers", body["message"])
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/works?ids="+repeatIDs(101))
	require.Equal(t, 400, code)
	assert.Equal(t, "at most 100 ids", body["message"])

	code, _ = getJSON(t, app, "/v1/catalog/works?ids="+repeatIDs(100))
	assert.Equal(t, 200, code, "100 ids is the ceiling, not one past it")
}

func TestWorksListClaimStateVocabulary(t *testing.T) {
	db := openCatalogTestDB(t)
	seedPublicWorks(t, db, 3)
	app := publicApp(db)

	for _, raw := range []string{"liev", "LIVE", "published", "true", "claimed", "live,bogus", "live,", ",live"} {
		t.Run("400 "+raw, func(t *testing.T) {
			code, body := getJSON(t, app, "/v1/catalog/works?claim_state="+raw)
			require.Equal(t, 400, code)
			assert.Equal(t, msgBadClaimState, body["message"])
		})
	}

	for _, raw := range []string{
		"none", "live", "draft", "pending", "declined", "hidden",
		"none,live,draft,pending,declined,hidden", "live,draft,pending",
		"%20live%20,%20draft%20",
	} {
		t.Run("200 "+raw, func(t *testing.T) {
			code, _ := getJSON(t, app, "/v1/catalog/works?claim_state="+raw)
			require.Equal(t, 200, code)
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/works")
	require.Equal(t, 200, code)
	assert.Len(t, body["data"].(map[string]any)["items"], 3)

	code, body = getJSON(t, app, "/v1/catalog/works?claim_state=none")
	require.Equal(t, 200, code)
	assert.Len(t, body["data"].(map[string]any)["items"], 3)

	code, body = getJSON(t, app, "/v1/catalog/works?claim_state=live,draft,hidden")
	require.Equal(t, 200, code)
	assert.Empty(t, body["data"].(map[string]any)["items"])
}

func TestPublicLimitSemantics(t *testing.T) {
	db := openCatalogTestDB(t)
	seedPublicWorks(t, db, 25)
	app := publicApp(db)

	for _, url := range []string{
		"/v1/catalog/works?limit=abc", "/v1/catalog/works?limit=0", "/v1/catalog/works?limit=-1",
		"/v1/catalog/changes?limit=abc", "/v1/catalog/changes?limit=0",
		"/v1/catalog/labels/1?limit=abc", "/v1/catalog/labels/1?limit=0",
	} {
		t.Run(url, func(t *testing.T) {
			code, body := getJSON(t, app, url)
			require.Equal(t, 400, code)
			assert.Equal(t, "limit must be a positive integer", body["message"])
		})
	}

	code, body := getJSON(t, app, "/v1/catalog/works?limit=1000")
	require.Equal(t, 200, code)
	items := body["data"].(map[string]any)["items"].([]any)
	assert.Len(t, items, 25, "over-max limit clamps to the ceiling, never falls back to the default")

	code, body = getJSON(t, app, "/v1/catalog/works")
	require.Equal(t, 200, code)
	assert.Len(t, body["data"].(map[string]any)["items"], 20)
}

func TestPublicLookupTypeVocabulary(t *testing.T) {
	db := openCatalogTestDB(t)
	seedPublicWorks(t, db, 1)
	app := publicApp(db)

	for _, raw := range []string{"person", "org", "release", "works", "WORK", "1"} {
		t.Run("GET type="+raw, func(t *testing.T) {
			code, body := getJSON(t, app, "/v1/catalog/lookup?source=vndb&external_id=v1&type="+raw)
			require.Equal(t, 400, code)
			assert.Equal(t, "type must be one of work, name, character, label", body["message"])
		})
	}

	for _, url := range []string{
		"/v1/catalog/lookup?source=vndb&external_id=v1",
		"/v1/catalog/lookup?source=vndb&external_id=v1&type=work",
		"/v1/catalog/lookup?source=vndb&external_id=p1&type=label",
		"/v1/catalog/lookup?source=bangumi&external_id=1&type=character",
		"/v1/catalog/lookup?source=bangumi&external_id=1&type=name",
		"/v1/catalog/lookup?source=nosuchsource&external_id=1&type=label",
	} {
		t.Run("GET "+url, func(t *testing.T) {
			code, _ := getJSON(t, app, url)
			assert.Equal(t, 404, code)
		})
	}

	code, body := postJSON(t, app, "/v1/catalog/lookup/batch",
		`{"items":[{"source":"vndb","external_id":"v1"},{"source":"vndb","external_id":"v2","type":"release"}]}`)
	require.Equal(t, 400, code, "one illegal pair fails the whole batch")
	assert.Equal(t, "type must be one of work, name, character, label", body["message"])

	code, body = postJSON(t, app, "/v1/catalog/lookup/batch",
		`{"items":[{"source":"vndb","external_id":"v1"},{"source":"vndb","external_id":"p1","type":"label"}]}`)
	require.Equal(t, 200, code)
	items := body["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, "work", items[0].(map[string]any)["type"])
	assert.Equal(t, "label", items[1].(map[string]any)["type"])
}

func TestLimitPubClampArithmetic(t *testing.T) {
	cases := []struct {
		raw      string
		def, max int
		want     int
		ok       bool
	}{
		{"", 20, 100, 20, true},
		{"  ", 20, 100, 20, true},
		{"1", 20, 100, 1, true},
		{"100", 20, 100, 100, true},
		{"101", 20, 100, 100, true},
		{"999999", 20, 100, 100, true},
		{"501", 100, 500, 500, true},
		{"51", 50, 50, 50, true},
		{"0", 20, 100, 0, false},
		{"-1", 20, 100, 0, false},
		{"abc", 20, 100, 0, false},
		{"1.5", 20, 100, 0, false},
	}
	for _, tc := range cases {
		got, ok := limitPub(tc.raw, tc.def, tc.max)
		assert.Equal(t, tc.ok, ok, "limitPub(%q) ok", tc.raw)
		assert.Equal(t, tc.want, got, "limitPub(%q) value", tc.raw)
	}
}

func TestPosIntQueryPub(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		v, ok := posIntQueryPub(raw)
		assert.True(t, ok)
		assert.EqualValues(t, 0, v, "absent = no filter")
	}
	v, ok := posIntQueryPub(" 42 ")
	assert.True(t, ok)
	assert.EqualValues(t, 42, v)
	for _, raw := range []string{"abc", "0", "-5", "1.5", "+"} {
		_, ok := posIntQueryPub(raw)
		assert.False(t, ok, "posIntQueryPub(%q) must be rejected", raw)
	}
}
