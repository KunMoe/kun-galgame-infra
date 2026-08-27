package handler

import (
	"net/http/httptest"
	"testing"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func releasesApp(db *gorm.DB) *fiber.App {
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/releases", h.Releases)
	return app
}

type releaseSeed struct {
	name       string
	y, m, d    int16
	kind       int16
	lang       *string
	platform   *string
	extra      string
	wantInFeed bool
}

func seedReleases(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	for _, tbl := range []string{
		"catalog_credit", "catalog_work_character", "catalog_work_label", "catalog_external_ref",
		"edit_suppressed_row", "catalog_work_title", "catalog_release", "catalog_work",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
	ids := map[string]int64{}
	mkWork := func(label, olang string, rating int16) int64 {
		w := model.CatalogWork{
			MediumID: 1, OLang: olang, DisplayName: label,
			ContentRating: rating, Status: model.WorkStatusLive,
		}
		require.NoError(t, db.Create(&w).Error)
		ids["work:"+label] = w.ID
		return w.ID
	}
	mkRelease := func(workID int64, s releaseSeed) {
		r := model.CatalogRelease{WorkID: workID, Kind: s.kind, Lang: s.lang, Platform: s.platform}
		if s.extra != "" {
			r.Extra = datatypes.JSON(s.extra)
		}
		if s.y != 0 {
			y := s.y
			r.ReleasedY = &y
			if s.m != 0 {
				m := s.m
				r.ReleasedM = &m
				if s.d != 0 {
					d := s.d
					r.ReleasedD = &d
				}
			}
		}
		require.NoError(t, db.Create(&r).Error)
		ids[s.name] = r.ID
	}
	ja, en := "ja", "en"
	win := "win"

	a := mkWork("Alpha", "ja", model.ContentRatingAllAges)
	mkRelease(a, releaseSeed{name: "a_original", y: 2024, m: 6, d: 14, kind: model.ReleaseKindDefault, lang: &ja, platform: &win})
	mkRelease(a, releaseSeed{name: "a_fanpatch", y: 2025, m: 3, d: 1, kind: model.ReleaseKindDigital, lang: &en, extra: `{"official":false}`})
	mkRelease(a, releaseSeed{name: "a_trial", y: 2025, m: 8, d: 1, kind: model.ReleaseKindTrial, lang: &ja})
	mkRelease(a, releaseSeed{name: "a_patch", y: 2025, m: 9, d: 1, kind: model.ReleaseKindPatch, lang: &ja})
	mkRelease(a, releaseSeed{name: "a_yearonly", y: 2020, kind: model.ReleaseKindDefault})
	mkRelease(a, releaseSeed{name: "a_undated", kind: model.ReleaseKindDefault})

	b := mkWork("Bravo", "ja", model.ContentRatingAllAges)
	mkRelease(b, releaseSeed{name: "b_sku", y: 2024, m: 6, kind: model.ReleaseKindPhysical})

	c := mkWork("Charlie", "en", model.ContentRatingAllAges)
	mkRelease(c, releaseSeed{name: "c_en", y: 2024, m: 7, d: 4, kind: model.ReleaseKindDefault, lang: &en})
	d := mkWork("Delta", "ja", model.ContentRatingR18)
	mkRelease(d, releaseSeed{name: "d_r18", y: 2024, m: 5, d: 5, kind: model.ReleaseKindDefault, lang: &ja})
	return ids
}

func feedIDs(t *testing.T, body map[string]any) []int64 {
	t.Helper()
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "envelope carries data")
	items, ok := data["items"].([]any)
	require.True(t, ok, "data carries items")
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = int64(it.(map[string]any)["id"].(float64))
	}
	return out
}

func getFeed(t *testing.T, app *fiber.App, url string) (int, map[string]any) {
	t.Helper()
	code, _, body := getWithHeaders(t, app, url, nil)
	return code, body
}

func TestReleaseFeedDefaultPopulation(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	code, body := getFeed(t, app, "/v1/catalog/releases")
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)
	assert.EqualValues(t, 3, data["count"], "count is the whole filtered set")
	assert.Equal(t, []int64{ids["a_fanpatch"], ids["a_original"], ids["b_sku"]}, feedIDs(t, body),
		"date_desc: 2025-03-01, 2024-06-14, then the 2024-06 month-precision SKU")
	assert.Nil(t, data["next_cursor"], "a short page ends the walk")

	items := data["items"].([]any)
	first := items[1].(map[string]any)
	assert.EqualValues(t, ids["a_original"], first["id"])
	assert.Equal(t, "2024-06-14", first["date"])
	assert.Equal(t, "default", first["kind"])
	assert.Equal(t, true, first["is_first"], "the work's earliest DATED release")
	assert.Equal(t, []any{}, first["refs"], "release refs are always present, [] when none")
	work := first["work"].(map[string]any)
	assert.EqualValues(t, ids["work:Alpha"], work["id"])
	assert.Equal(t, "Alpha", work["display_name"])
	assert.Equal(t, "2020", work["release_date"], "work grain: earliest year-carrying release")
	assert.Nil(t, work["names"], "include= blocks stay absent by default")

	port := items[0].(map[string]any)
	assert.EqualValues(t, ids["a_fanpatch"], port["id"])
	assert.Equal(t, false, port["is_first"], "a later edition of an already-released work")
	assert.EqualValues(t, ids["work:Alpha"], port["work"].(map[string]any)["id"],
		"two rows of one work carry the same work block")

	sku := items[2].(map[string]any)
	assert.Equal(t, "2024-06", sku["date"])
	assert.Equal(t, "physical", sku["kind"])
	assert.Nil(t, sku["lang"], "an unrecorded release language is omitted, never guessed")
	assert.Equal(t, true, sku["is_first"])
}

func TestReleaseFeedKindGate(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?kind=trial")
	assert.Equal(t, []int64{ids["a_trial"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?kind=patch")
	assert.Equal(t, []int64{ids["a_patch"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?kind=default,digital,physical,trial,patch")
	assert.Len(t, feedIDs(t, body), 5)
	_, body = getFeed(t, app, "/v1/catalog/releases?kind=trial,trial")
	assert.Equal(t, []int64{ids["a_trial"]}, feedIDs(t, body))
}

func TestReleaseFeedWorkGates(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?olang=all")
	assert.Contains(t, feedIDs(t, body), ids["c_en"], "olang=all reaches the English work")
	_, body = getFeed(t, app, "/v1/catalog/releases?olang=en")
	assert.Equal(t, []int64{ids["c_en"]}, feedIDs(t, body))
	code, body := getFeed(t, app, "/v1/catalog/releases?olang=xx-Nope")
	require.Equal(t, 200, code)
	assert.Empty(t, feedIDs(t, body))

	_, body = getFeed(t, app, "/v1/catalog/releases?nsfw=1")
	assert.Contains(t, feedIDs(t, body), ids["d_r18"])
	_, body = getFeed(t, app, "/v1/catalog/releases")
	assert.NotContains(t, feedIDs(t, body), ids["d_r18"])

	_, body = getFeed(t, app, "/v1/catalog/releases?content_limit=sfw")
	assert.Equal(t, []int64{ids["a_fanpatch"], ids["a_original"], ids["b_sku"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?nsfw=1&content_limit=nsfw")
	assert.Equal(t, []int64{ids["d_r18"]}, feedIDs(t, body))
}

func TestReleaseFeedLangCoalesce(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?lang=ja")
	assert.Equal(t, []int64{ids["a_original"], ids["b_sku"]}, feedIDs(t, body),
		"b_sku carries lang NULL and its work is olang=ja")
	_, body = getFeed(t, app, "/v1/catalog/releases?lang=en")
	assert.Equal(t, []int64{ids["a_fanpatch"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?lang=ja,en")
	assert.Len(t, feedIDs(t, body), 3)
	_, body = getFeed(t, app, "/v1/catalog/releases?lang=all")
	assert.Len(t, feedIDs(t, body), 3)
	code, body := getFeed(t, app, "/v1/catalog/releases?lang=xx")
	require.Equal(t, 200, code)
	assert.Empty(t, feedIDs(t, body))
}

func TestReleaseFeedOfficialGate(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases")
	assert.Len(t, feedIDs(t, body), 3, "absent = no gate")
	_, body = getFeed(t, app, "/v1/catalog/releases?official=true")
	assert.Equal(t, []int64{ids["a_original"], ids["b_sku"]}, feedIDs(t, body),
		"the keyless rows are official; only the explicit false drops out")
	_, body = getFeed(t, app, "/v1/catalog/releases?official=false")
	assert.Equal(t, []int64{ids["a_fanpatch"]}, feedIDs(t, body))
}

func TestReleaseFeedPlatformGate(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?platform=win")
	assert.Equal(t, []int64{ids["a_original"]}, feedIDs(t, body))
	code, body := getFeed(t, app, "/v1/catalog/releases?platform=nope")
	require.Equal(t, 200, code)
	assert.Empty(t, feedIDs(t, body))
}

func TestReleaseFeedDateWindow(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?date_from=2024-06-14&date_to=2024-06-14")
	assert.Equal(t, []int64{ids["a_original"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?date_from=2024-06-01&date_to=2024-06-30")
	assert.Equal(t, []int64{ids["a_original"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?date_from=2024-05-31&date_to=2024-06-30")
	assert.Equal(t, []int64{ids["a_original"], ids["b_sku"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?date_from=2025-01-01")
	assert.Equal(t, []int64{ids["a_fanpatch"]}, feedIDs(t, body))
	_, body = getFeed(t, app, "/v1/catalog/releases?date_to=2024-06-13")
	assert.Equal(t, []int64{ids["b_sku"]}, feedIDs(t, body))
}

func TestReleaseFeedSortAndCursor(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	_, body := getFeed(t, app, "/v1/catalog/releases?sort=date_asc")
	assert.Equal(t, []int64{ids["b_sku"], ids["a_original"], ids["a_fanpatch"]}, feedIDs(t, body),
		"date_asc is the exact reverse of the default here")

	var walked []int64
	url := "/v1/catalog/releases?limit=1"
	for range 4 {
		code, body := getFeed(t, app, url)
		require.Equal(t, 200, code)
		data := body["data"].(map[string]any)
		assert.EqualValues(t, 3, data["count"], "count is the feed, not the page")
		got := feedIDs(t, body)
		walked = append(walked, got...)
		cur, ok := data["next_cursor"].(string)
		if !ok {
			break
		}
		url = "/v1/catalog/releases?limit=1&cursor=" + cur
	}
	assert.Equal(t, []int64{ids["a_fanpatch"], ids["a_original"], ids["b_sku"]}, walked)

	_, body = getFeed(t, app, "/v1/catalog/releases?limit=1")
	cur := body["data"].(map[string]any)["next_cursor"].(string)
	code, body := getFeed(t, app, "/v1/catalog/releases?sort=date_asc&limit=1&cursor="+cur)
	require.Equal(t, 400, code)
	assert.Equal(t, msgBadCursor, body["message"])
}

func TestReleaseFeedIncludeBlocks(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)
	require.NoError(t, db.Create(&model.CatalogWorkTitle{
		WorkID: ids["work:Alpha"], Lang: "ja", Title: "アルファ", Kind: model.WorkTitleKindOfficial,
	}).Error)

	code, body := getFeed(t, app, "/v1/catalog/releases?include=names,nonsense")
	require.Equal(t, 200, code)
	items := body["data"].(map[string]any)["items"].([]any)
	work := items[0].(map[string]any)["work"].(map[string]any)
	names, ok := work["localized"].(map[string]any)
	require.True(t, ok, "include=names attaches the block to the work")
	assert.Equal(t, map[string]any{"value": "アルファ", "kind": "official"}, names["ja"])
}

func TestReleaseFeedParamValidation(t *testing.T) {
	db := openCatalogTestDB(t)
	seedReleases(t, db)
	app := releasesApp(db)

	for _, tc := range []struct{ url, msg string }{
		{"/v1/catalog/releases?sort=date", msgBadReleaseSort},
		{"/v1/catalog/releases?sort=popularity", msgBadReleaseSort},
		{"/v1/catalog/releases?kind=demo", msgBadReleaseKind},
		{"/v1/catalog/releases?kind=digital,demo", msgBadReleaseKind},
		{"/v1/catalog/releases?official=maybe", msgBadOfficialFlag},
		{"/v1/catalog/releases?official=1", msgBadOfficialFlag},
		{"/v1/catalog/releases?date_from=2024-06", msgBadReleaseDate},
		{"/v1/catalog/releases?date_to=nope", msgBadReleaseDate},
		{"/v1/catalog/releases?content_limit=all", msgBadDisplayLimit},
		{"/v1/catalog/releases?limit=0", msgBadLimit},
		{"/v1/catalog/releases?limit=abc", msgBadLimit},
		{"/v1/catalog/releases?limit=-1", msgBadLimit},
		{"/v1/catalog/releases?cursor=!!!nope!!!", msgBadCursor},
	} {
		t.Run(tc.url, func(t *testing.T) {
			code, body := getFeed(t, app, tc.url)
			require.Equal(t, 400, code)
			assert.Equal(t, tc.msg, body["message"])
		})
	}

	for _, url := range []string{
		"/v1/catalog/releases?kind=", "/v1/catalog/releases?sort=", "/v1/catalog/releases?official=",
		"/v1/catalog/releases?limit=100", "/v1/catalog/releases?date_from=1970-01-01",
		"/v1/catalog/releases?include=names,intros,labels,ratings,covers,refs",
	} {
		t.Run("legal "+url, func(t *testing.T) {
			code, _ := getFeed(t, app, url)
			assert.Equal(t, 200, code)
		})
	}
}

func TestReleaseFeedETagRoundTrip(t *testing.T) {
	db := openCatalogTestDB(t)
	ids := seedReleases(t, db)
	app := releasesApp(db)

	code, h, _ := getWithHeaders(t, app, "/v1/catalog/releases", nil)
	require.Equal(t, 200, code)
	require.NotEmpty(t, h["ETag"])
	assert.Equal(t, cacheSearch, h["Cache-Control"])

	code, h2, body := getWithHeaders(t, app, "/v1/catalog/releases", map[string]string{"If-None-Match": h["ETag"]})
	assert.Equal(t, 304, code)
	assert.Equal(t, h["ETag"], h2["ETag"], "a 304 still carries the validator")
	assert.Empty(t, body, "a 304 carries no body")

	code, _, body = getWithHeaders(t, app, "/v1/catalog/releases", map[string]string{"If-None-Match": `W/"relfeed-stale"`})
	assert.Equal(t, 200, code)
	assert.NotNil(t, body["data"])

	for _, url := range []string{
		"/v1/catalog/releases?nsfw=1",
		"/v1/catalog/releases?olang=all",
		"/v1/catalog/releases?content_limit=sfw",
		"/v1/catalog/releases?kind=trial",
		"/v1/catalog/releases?lang=ja",
		"/v1/catalog/releases?official=true",
		"/v1/catalog/releases?platform=win",
		"/v1/catalog/releases?date_from=2024-01-01",
		"/v1/catalog/releases?date_to=2024-12-31",
	} {
		_, other, _ := getWithHeaders(t, app, url, nil)
		assert.NotEqual(t, h["ETag"], other["ETag"], "%s must not share a validator with the ungated feed", url)
	}
	_, reversed, _ := getWithHeaders(t, app, "/v1/catalog/releases?sort=date_asc", nil)
	assert.Equal(t, h["ETag"], reversed["ETag"], "one population, one validator — sort is not a gate")

	require.NoError(t, db.Exec(`UPDATE catalog_release SET released_m = 4 WHERE id = ?`, ids["a_yearonly"]).Error)
	code, after, _ := getWithHeaders(t, app, "/v1/catalog/releases", nil)
	require.Equal(t, 200, code)
	assert.NotEqual(t, h["ETag"], after["ETag"])
	code, _, _ = getWithHeaders(t, app, "/v1/catalog/releases", map[string]string{"If-None-Match": h["ETag"]})
	assert.Equal(t, 200, code, "the stale validator must no longer 304")

	_, body = getFeed(t, app, "/v1/catalog/releases?sort=date_asc")
	items := body["data"].(map[string]any)["items"].([]any)
	assert.EqualValues(t, ids["a_yearonly"], items[0].(map[string]any)["id"])
	assert.Equal(t, true, items[0].(map[string]any)["is_first"])
	for _, it := range items[1:] {
		if int64(it.(map[string]any)["id"].(float64)) == ids["a_original"] {
			assert.Equal(t, false, it.(map[string]any)["is_first"],
				"a genuinely earlier release demotes the former first")
		}
	}
}

func TestReleaseKindVocabularyIsSymmetric(t *testing.T) {
	for _, k := range []int16{
		model.ReleaseKindDefault, model.ReleaseKindDigital, model.ReleaseKindPhysical,
		model.ReleaseKindTrial, model.ReleaseKindPatch,
	} {
		key := releaseKindKeyForTest(t, k)
		back, ok := service.ReleaseKindFromKey(key)
		require.True(t, ok, "the printed key %q must parse back", key)
		assert.Equal(t, k, back)
	}
	for _, bad := range []string{"", "demo", "Digital", "release", "0"} {
		_, ok := service.ReleaseKindFromKey(bad)
		assert.False(t, ok, "%q is outside the closed vocabulary", bad)
	}
	assert.Equal(t,
		[]int16{model.ReleaseKindDefault, model.ReleaseKindDigital, model.ReleaseKindPhysical},
		service.DefaultReleaseFeedKinds)
}

func releaseKindKeyForTest(t *testing.T, kind int16) string {
	t.Helper()
	switch kind {
	case model.ReleaseKindDigital:
		return "digital"
	case model.ReleaseKindPhysical:
		return "physical"
	case model.ReleaseKindTrial:
		return "trial"
	case model.ReleaseKindPatch:
		return "patch"
	default:
		return "default"
	}
}

func TestReleaseFeedRouteOrder(t *testing.T) {
	db := openCatalogTestDB(t)
	seedReleases(t, db)
	resolveSvc := service.NewResolveService(repository.NewRedirectRepository(db))
	publicSvc := service.NewPublicService(db, service.NewReadService(db), resolveSvc, "")
	h := NewPublicHandler(publicSvc, resolveSvc, nil, nil)
	app := fiber.New()
	app.Get("/v1/catalog/releases", h.Releases)
	app.Get("/v1/catalog/calendar", h.Calendar)

	resp, err := app.Test(httptest.NewRequest("GET", "/v1/catalog/releases", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
