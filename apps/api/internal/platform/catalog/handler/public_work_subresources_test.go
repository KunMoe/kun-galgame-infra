package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getWithHeader(t *testing.T, app *fiber.App, url, header string) (int, string, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, resp.Header.Get(header), out
}

func subresData(t *testing.T, app *fiber.App, url string) map[string]any {
	t.Helper()
	code, body := getJSON(t, app, url)
	require.Equalf(t, 200, code, "GET %s", url)
	return body["data"].(map[string]any)
}

// TestWorkSubresourceItemsMatchTheParentBlock is the anti-drift test: every
// sub-resource must serve the SAME rows the corresponding works/{id} block
// serves, byte for byte, or the face has grown a second representation of one
// concept — the legacy this decomposition exists to avoid.
func TestWorkSubresourceItemsMatchTheParentBlock(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)

	parent := subresData(t, app, "/v1/catalog/works/"+itoa(f.work)+"?include=relations,credits&spoilers=2&nsfw=1")
	for _, lane := range workSubresourceLanes {
		t.Run(lane.suffix, func(t *testing.T) {
			block, ok := parent[lane.block]
			require.Truef(t, ok, "works/{id} must carry a %s block to compare against", lane.block)
			sub := subresData(t, app, "/v1/catalog/works/"+itoa(f.work)+"/"+lane.suffix+"?spoilers=2&nsfw=1")
			assert.Equal(t, block, sub["items"],
				"works/{id}.%s and works/{id}/%s must be the same rows", lane.block, lane.suffix)
			assert.NotEmptyf(t, sub["items"], "the fixture must actually populate %s", lane.block)
			assert.Nil(t, sub["next_offset"], "a full block fits one page at the default limit")
		})
	}
}

func TestWorkSubresourcePagingAndNextOffset(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)
	base := "/v1/catalog/works/" + itoa(f.work)

	all := subresData(t, app, base+"/covers")["items"].([]any)
	require.Len(t, all, 3, "the hash-less cover row is dropped before paging, not after")

	first := subresData(t, app, base+"/covers?limit=2")
	assert.Len(t, first["items"], 2)
	assert.EqualValues(t, 2, first["next_offset"], "more rows remain, so the caller is told where to resume")

	second := subresData(t, app, base+"/covers?limit=2&offset=2")
	assert.Len(t, second["items"], 1)
	assert.Nil(t, second["next_offset"], "the last page carries no next_offset")
	assert.Equal(t, all[2], second["items"].([]any)[0], "paging does not reorder the block")

	past := subresData(t, app, base+"/covers?offset=99")
	assert.Empty(t, past["items"], "an offset past the end is an empty page, not an error")
	assert.Nil(t, past["next_offset"])

	// Credits page over credit ROWS: five credits across two roles, so a limit of
	// three splits the scenario role across both pages.
	p1 := subresData(t, app, base+"/credits?limit=3")
	assert.EqualValues(t, 3, p1["next_offset"])
	p2 := subresData(t, app, base+"/credits?limit=3&offset=3")
	assert.Nil(t, p2["next_offset"])
	rows := 0
	for _, page := range []map[string]any{p1, p2} {
		for _, g := range page["items"].([]any) {
			rows += len(g.(map[string]any)["credits"].([]any))
		}
	}
	assert.Equal(t, 5, rows, "concatenating the pages yields every credit exactly once")

	for _, bad := range []string{"limit=0", "limit=-1", "limit=abc"} {
		code, body := getJSON(t, app, base+"/covers?"+bad)
		require.Equalf(t, 400, code, "%s must be refused", bad)
		assert.Equal(t, msgBadLimit, body["message"])
	}
	assert.Len(t, subresData(t, app, base+"/covers?limit=500")["items"], 3, "limit above the cap clamps")
}

// TestWorkSubresource404ParityWithTheParent is the one that matters most:
// leaking a hidden or non-live work through a new door would be a visibility
// regression, not a missing feature.
func TestWorkSubresource404ParityWithTheParent(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)

	for _, tc := range []struct {
		name  string
		id    int64
		query string
	}{
		{"unknown id", 99999999, ""},
		{"soft-deleted work", f.deleted, ""},
		{"non-live stub", f.stub, ""},
		{"r18 without nsfw", f.r18, ""},
		{"r18 with nsfw", f.r18, "?nsfw=1"},
		{"live work", f.work, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parentCode, _ := getJSON(t, app, "/v1/catalog/works/"+itoa(tc.id)+tc.query)
			for _, lane := range workSubresourceLanes {
				code, _ := getJSON(t, app, "/v1/catalog/works/"+itoa(tc.id)+"/"+lane.suffix+tc.query)
				assert.Equalf(t, parentCode, code,
					"works/%d%s answers %d — works/%d/%s must answer the same",
					tc.id, tc.query, parentCode, tc.id, lane.suffix)
			}
		})
	}

	code, _ := getJSON(t, app, "/v1/catalog/works/not-a-number/covers")
	assert.Equal(t, 400, code, "a non-numeric id is a bad request, not a miss")
}

func TestWorkSubresourceNSFWAndSpoilerParity(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)
	base := "/v1/catalog/works/" + itoa(f.work)

	names := func(url string) []string {
		out := []string{}
		for _, it := range subresData(t, app, url)["items"].([]any) {
			out = append(out, it.(map[string]any)["name"].(string))
		}
		return out
	}
	assert.NotContains(t, names(base+"/tags"), "重大ネタバレ", "the default ceiling hides a major-spoiler tag")
	assert.Contains(t, names(base+"/tags?spoilers=2"), "重大ネタバレ")
	assert.Equal(t, len(names(base+"/tags?spoilers=2")), 1+len(names(base+"/tags")),
		"raising the ceiling adds exactly the spoiler row")

	sfw := subresData(t, app, base+"/relations")["items"].([]any)
	nsfw := subresData(t, app, base+"/relations?nsfw=1")["items"].([]any)
	assert.Len(t, sfw, 1, "the r18 relation end is dropped whole for an sfw caller")
	assert.Len(t, nsfw, 2)

	parentTags := subresData(t, app, base+"?spoilers=1")["tags"]
	assert.Equal(t, parentTags, subresData(t, app, base+"/tags?spoilers=1")["items"],
		"the spoiler ceiling means the same thing on both faces")

	roster := subresData(t, app, base+"/characters?spoilers=0")["items"].([]any)
	assert.Equal(t, subresData(t, app, base)["characters"], any(roster),
		"the roster carries its own spoiler level and is never ceilinged, exactly like the parent block")
	spoilt := 0
	for _, r := range roster {
		if r.(map[string]any)["spoiler"].(float64) > 0 {
			spoilt++
		}
	}
	assert.Equal(t, 1, spoilt, "the fixture must contain a major-spoiler roster row for that claim to mean anything")
}

func TestWorkSubresourceRoutingBesideWorksSearch(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)

	code, body := getJSON(t, app, "/v1/catalog/works/search?sort=view")
	require.Equal(t, 400, code)
	assert.Equal(t, msgBadSearchSort, body["message"],
		"works/search must still reach the search handler, not be parsed as a work id")

	code, _ = getJSON(t, app, "/v1/catalog/works/"+itoa(f.work))
	assert.Equal(t, 200, code, "the parent route still resolves with the sub-routes registered")
}

func TestWorkSubresourceCacheControl(t *testing.T) {
	db := openCatalogTestDB(t)
	f := seedRichWork(t, db)
	app := workSubresourceApp(db)
	base := "/v1/catalog/works/" + itoa(f.work)

	for _, lane := range workSubresourceLanes {
		want := cacheDetail
		if lane.suffix == "ratings" || lane.suffix == "relations" {
			want = cacheWorkImported
		}
		code, got, _ := getWithHeader(t, app, base+"/"+lane.suffix, "Cache-Control")
		require.Equal(t, 200, code)
		assert.Equalf(t, want, got, "%s must carry its own cache tier", lane.suffix)
	}
}
