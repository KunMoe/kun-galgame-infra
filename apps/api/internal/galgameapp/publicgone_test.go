package galgameapp

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api/internal/app"
	"api/pkg/errors"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

var retiredPublicPaths = []string{
	"/v1/galgame",
	"/v1/galgame/1",
	"/v1/galgame/batch?ids=1",
	"/v1/galgame/search?q=x",
	"/v1/galgame/changes",
	"/v1/galgame/stats",
	"/v1/galgame/lookup?vndb_id=v17",
	"/v1/galgame/calendar",
	"/v1/galgame/calendar/pending",
	"/v1/galgame/calendar/tba",
	"/v1/galgame/officials/1/galgames",
	"/v1/galgame/tags/1/galgames",
	"/v1/galgame/tags",
	"/v1/galgame/tags/search?q=x",
	"/v1/galgame/tags/multi?ids=1",
	"/v1/galgame/tags/1",
	"/v1/galgame/tags/1/galgame-ids",
	"/v1/galgame/officials",
	"/v1/galgame/officials/search?q=x",
	"/v1/galgame/officials/1",
	"/v1/galgame/officials/1/galgame-ids",
	"/v1/galgame/engines",
	"/v1/galgame/engines/1",
	"/v1/galgame/engines/1/galgame-ids",
	"/v1/galgame/series",
	"/v1/galgame/series/1",
	"/v1/galgame/openapi.json",
	"/v1/galgame/whatever/deeply/nested",
}

func goneApp() *fiber.App {
	a := &app.App{Fiber: fiber.New()}
	MountRetiredPublic(a)
	return a.Fiber
}

func getGone(t *testing.T, f *fiber.App, method, path, apiKey string) (int, map[string]string, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if apiKey != "" {
		r.Header.Set("X-API-Key", apiKey)
	}
	resp, err := f.Test(r, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	var env map[string]any
	require.NoErrorf(t, json.Unmarshal(raw, &env), "%s %s did not return JSON: %s", method, path, raw)
	hdr := map[string]string{
		"Link":        resp.Header.Get("Link"),
		"Deprecation": resp.Header.Get("Deprecation"),
		"Sunset":      resp.Header.Get("Sunset"),
	}
	return resp.StatusCode, hdr, env
}

func TestRetiredPublicFaceIsGone(t *testing.T) {
	f := goneApp()

	require.Len(t, retiredPublicPaths, 28, "census = the 26 published ops + openapi.json + one unregistered path")
	for _, path := range retiredPublicPaths {
		status, hdr, env := getGone(t, f, "GET", path, "")
		require.Equalf(t, fiber.StatusGone, status, "GET %s must be 410 Gone", path)
		require.EqualValuesf(t, errors.ErrGone, env["code"], "GET %s must carry the ErrGone code", path)
		msg, _ := env["message"].(string)
		require.Containsf(t, msg, "v2 API", "GET %s must name the successor face", path)
		require.NotContainsf(t, msg, "/v1/catalog", "GET %s must not point at the retired v1 catalog face", path)
		require.Containsf(t, msg, "2026-07-30", "GET %s must state the retirement date", path)
		require.Containsf(t, msg, "https://developer.nextmoe.dev/docs/v2", "GET %s must link the docs", path)
		require.NotContainsf(t, env, "data", "GET %s must not carry a data block", path)
		require.Equalf(t, retiredSuccessorLink, hdr["Link"], "GET %s must keep the successor Link", path)
		require.Emptyf(t, hdr["Deprecation"], "GET %s must not still announce a pending deprecation", path)
		require.Emptyf(t, hdr["Sunset"], "GET %s must not still announce a pending sunset", path)
	}
}

func TestRetiredPublicFaceIgnoresCredentialsAndMethods(t *testing.T) {
	f := goneApp()

	for _, key := range []string{"", "not-a-key", "nm_live_0000000000000000"} {
		status, _, env := getGone(t, f, "GET", "/v1/galgame/1", key)
		require.Equal(t, fiber.StatusGone, status, "410 must not depend on the credential")
		require.EqualValues(t, errors.ErrGone, env["code"])
	}

	for _, m := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		r := httptest.NewRequest(m, "/v1/galgame/tags/1", nil)
		resp, err := f.Test(r, fiber.TestConfig{Timeout: 5 * time.Second})
		require.NoError(t, err)
		require.Equalf(t, fiber.StatusGone, resp.StatusCode, "%s must also be 410", m)
	}
}

func TestRetiredPublicCatchAllStaysInsideItsPrefix(t *testing.T) {
	a := &app.App{Fiber: fiber.New()}
	MountRetiredPublic(a)
	ok := func(c fiber.Ctx) error { return c.SendString("survivor") }
	for _, p := range []string{
		"/v1/catalog/works/1",
		"/v1/catalogue",
		"/api/galgame/catalog/stats",
		"/internal/galgame/mine",
		"/v1/galgamesque",
	} {
		a.Fiber.Get(p, ok)
	}
	f := a.Fiber

	for _, p := range []string{
		"/v1/catalog/works/1", "/v1/catalogue",
		"/api/galgame/catalog/stats", "/internal/galgame/mine", "/v1/galgamesque",
	} {
		r := httptest.NewRequest("GET", p, nil)
		resp, err := f.Test(r, fiber.TestConfig{Timeout: 5 * time.Second})
		require.NoError(t, err)
		raw, _ := io.ReadAll(resp.Body)
		require.Equalf(t, fiber.StatusOK, resp.StatusCode, "surviving route %s must not be captured by the 410 catch-all", p)
		require.Equalf(t, "survivor", strings.TrimSpace(string(raw)), "surviving route %s must reach its own handler", p)
	}
}

func TestRetiredSiblingFacesFallThroughTo404(t *testing.T) {
	a := &app.App{Fiber: fiber.New()}
	MountRetiredPublic(a)

	for _, p := range []string{
		"/api/galgame/catalog/stats",
		"/api/admin/galgame/123",
		"/api/admin/galgame/messages",
		"/api/tag/search", "/api/tag/123",
		"/api/official/123/revert",
		"/internal/galgame/mine",
		"/internal/galgame/123",
		"/internal/galgame/messages/feed",
		"/internal/galgame/revisions/recent",
		"/internal/tag/1/revisions",
		"/internal/edit/proposals",
	} {
		for _, m := range []string{"GET", "POST", "DELETE"} {
			r := httptest.NewRequest(m, p, nil)
			resp, err := a.Fiber.Test(r, fiber.TestConfig{Timeout: 5 * time.Second})
			require.NoError(t, err)
			require.Equalf(t, fiber.StatusNotFound, resp.StatusCode,
				"%s %s must fall through to a plain 404, not be captured by a surviving pattern", m, p)
		}
	}
}
