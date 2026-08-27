package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"api/internal/platform/catalog/dto"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectionFixtureWork() dto.PublicCatalogWork {
	date := "2021-06-04"
	return dto.PublicCatalogWork{
		ID: 4242, Medium: "galgame", DisplayName: "投影テスト", Latin: "Toei Test",
		Localized: map[string]dto.PublicLocalizedName{
			"zh-Hans": {Value: "投影测试", Kind: "official"},
		},
		OLang: "ja", ContentRating: "all_ages", ReleaseDate: &date,
		Created: "2026-01-02T03:04:05Z", Updated: "2026-01-02T03:04:05Z",
		Titles:      []dto.PublicCatalogTitle{{Lang: "ja", Title: "投影テスト", Kind: "official"}},
		Refs:        []dto.PublicCatalogRef{{Source: "vndb", ExternalID: "v42"}},
		ClaimedBy:   &dto.PublicClaimedBy{Site: "kungal", WorkID: 9, State: "live", ContentLimit: "sfw"},
		Releases:    []dto.PublicRelease{{ID: 7, Kind: "default", Date: &date, Refs: []dto.PublicCatalogRef{}, Labels: []dto.PublicWorkLabel{}}},
		Popularity:  []dto.PublicPopularity{{Source: "vndb", Metric: "wishlist", Value: 10}},
		Ratings:     []dto.PublicRating{{Source: "vndb", Score: 8.5, VoteCount: 1200}},
		Tags:        []dto.PublicTag{{Name: "moe", Source: "vndb"}},
		Playtimes:   []dto.PublicPlaytime{},
		Series:      []dto.PublicSeries{},
		Platforms:   []dto.PublicPlatform{},
		Intros:      []dto.PublicIntro{{Lang: "ja", Intro: "紹介", Source: "vndb"}},
		Covers:      []dto.PublicCover{},
		Screenshots: []dto.PublicScreenshot{},
		Characters:  []dto.PublicRosterCharacter{},
		Labels:      []dto.PublicWorkLabel{},
		Engines:     []dto.PublicWorkEngine{},
		Links:       []dto.PublicWorkLink{},

		SeriesSiblings: []dto.PublicWorkBrief{},
	}
}

func projectionFixtureList() dto.PublicWorksListData {
	date := "2021-06-04"
	next := "cursor-token"
	return dto.PublicWorksListData{
		Items: []dto.PublicWorkListItem{
			{
				ID: 1, Medium: "galgame", DisplayName: "一件目", ContentRating: "all_ages",
				OLang: "ja", ReleaseDate: &date, Cover: "https://cdn/x.webp", Updated: "2026-01-02T03:04:05Z",
				Ratings: []dto.PublicRating{{Source: "vndb", Score: 7, VoteCount: 3}},
			},
			{
				ID: 2, Medium: "galgame", DisplayName: "二件目", ContentRating: "r18",
				OLang: "ja", Updated: "2026-01-02T03:04:06Z",
			},
		},
		NextCursor: &next,
	}
}

// bodyFor runs one response through the real fiber JSON path, which is the only
// place "byte-identical" can be asserted.
func bodyFor(t *testing.T, url string, send func(fiber.Ctx) error) []byte {
	t.Helper()
	app := fiber.New()
	app.Get("/x", send)
	resp, err := app.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}

func projectedBody(t *testing.T, url string, data any, items bool) []byte {
	t.Helper()
	return bodyFor(t, url, func(c fiber.Ctx) error {
		sel := fieldsQuery(c)
		if items {
			return successProjected(c, data, sel, sel.ProjectItems)
		}
		return successProjected(c, data, sel, sel.ProjectObject)
	})
}

func dataKeys(t *testing.T, body []byte) []string {
	t.Helper()
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	keys := make([]string, 0, len(env.Data))
	for k := range env.Data {
		keys = append(keys, k)
	}
	return keys
}

func TestFieldsInactiveIsByteIdentical(t *testing.T) {
	rec := projectionFixtureWork()
	want := bodyFor(t, "/x", func(c fiber.Ctx) error { return response.Success(c, rec) })

	for _, url := range []string{"/x", "/x?fields=", "/x?fields=%20", "/x?fields=++"} {
		got := projectedBody(t, url, rec, false)
		assert.Equalf(t, string(want), string(got),
			"%s must be byte-identical to the unprojected response", url)
	}

	page := projectionFixtureList()
	wantList := bodyFor(t, "/x", func(c fiber.Ctx) error { return response.Success(c, page) })
	for _, url := range []string{"/x", "/x?fields=", "/x?fields=%20"} {
		got := projectedBody(t, url, page, true)
		assert.Equalf(t, string(wantList), string(got), "%s (list) must be byte-identical", url)
	}
}

func TestFieldsAlwaysKeepsID(t *testing.T) {
	rec := projectionFixtureWork()
	for _, url := range []string{"/x?fields=display_name", "/x?fields=id,display_name", "/x?fields=nonsense"} {
		keys := dataKeys(t, projectedBody(t, url, rec, false))
		assert.Containsf(t, keys, "id", "%s must still carry id", url)
	}
	assert.ElementsMatch(t, []string{"id"}, dataKeys(t, projectedBody(t, "/x?fields=nonsense", rec, false)),
		"an all-unknown selection is an id-only response, not a 400")
}

func TestFieldsIgnoresUnknownTokens(t *testing.T) {
	rec := projectionFixtureWork()
	withJunk := projectedBody(t, "/x?fields=display_name,not_a_key,medium", rec, false)
	clean := projectedBody(t, "/x?fields=display_name,medium", rec, false)
	assert.Equal(t, string(clean), string(withJunk),
		"an unknown token must be ignored, never a 400 and never a shape change")
}

func TestFieldsIsOrderAndDuplicateInsensitive(t *testing.T) {
	rec := projectionFixtureWork()
	a := projectedBody(t, "/x?fields=display_name,medium,tags", rec, false)
	b := projectedBody(t, "/x?fields=tags,medium,display_name", rec, false)
	c := projectedBody(t, "/x?fields=medium,%20tags%20,display_name,medium", rec, false)
	assert.Equal(t, string(a), string(b), "two orderings of one selection must be byte-identical")
	assert.Equal(t, string(a), string(c), "whitespace and duplicates must not change the bytes")
}

func TestFieldsIsTrimOnly(t *testing.T) {
	rec := projectionFixtureWork()
	var full struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(projectedBody(t, "/x", rec, false), &full))

	var trimmed struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(projectedBody(t, "/x?fields=claimed_by,localized,ratings,releases", rec, false), &trimmed))

	assert.ElementsMatch(t, []string{"id", "claimed_by", "localized", "ratings", "releases"}, keysOf(trimmed.Data))
	for _, k := range []string{"claimed_by", "localized", "ratings", "releases"} {
		assert.Equalf(t, string(full.Data[k]), string(trimmed.Data[k]),
			"%s must be byte-identical to its unprojected value — fields= never reshapes inside a key", k)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFieldsOnAListLeavesTheEnvelopeAlone(t *testing.T) {
	page := projectionFixtureList()
	body := projectedBody(t, "/x?fields=display_name", page, true)

	var env struct {
		Data struct {
			Items      []map[string]json.RawMessage `json:"items"`
			NextCursor *string                      `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.NotNil(t, env.Data.NextCursor, "next_cursor is an ENVELOPE key; fields= names item keys only")
	assert.Equal(t, "cursor-token", *env.Data.NextCursor)
	require.Len(t, env.Data.Items, 2)
	for _, item := range env.Data.Items {
		assert.ElementsMatch(t, []string{"id", "display_name"}, keysOf(item))
	}
}

func TestFieldsPreservesTheDeclaredKeyOrder(t *testing.T) {
	rec := projectionFixtureWork()
	body := projectedBody(t, "/x?fields=tags,display_name,medium", rec, false)
	assert.Contains(t, string(body), `"id":4242,"medium":"galgame","display_name":"投影テスト"`,
		"kept keys must stay in the DTO's declared order, not the order the caller typed")
	assert.Contains(t, string(body), `"tags":[`)
}
