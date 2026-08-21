package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cacheHeaderFor(t *testing.T, app *fiber.App, url, token string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Cache-Control")
}

// The pending lane varies on a credential a shared cache does not key on, so
// it must never be stored: one tenant's review queue served to the next tenant
// that asked for the same URL is a cross-tenant disclosure, not a stale page.
func TestPendingQueueIsNeverSharedCacheable(t *testing.T) {
	db := openCatalogTestDB(t)
	seedQueueWorks(t, db)
	app := queueApp(db, queueClients())
	mod := userTokenRoles(t, 730, ScopeCatalogEdit, "kungal-client", "user", "moderator")

	status, cache := cacheHeaderFor(t, app, "/v1/catalog/works?status=pending", mod)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, cacheModeration, cache)

	for _, url := range []string{"/v1/catalog/works", "/v1/catalog/works?status=live"} {
		for _, tok := range []string{"", mod} {
			status, cache := cacheHeaderFor(t, app, url, tok)
			require.Equalf(t, fiber.StatusOK, status, "%s must stay a plain 200", url)
			assert.Equalf(t, cacheSearch, cache,
				"%s must keep the shared-cache header byte-identical", url)
		}
	}
}

func TestWorksListCacheControlVocabulary(t *testing.T) {
	app := fiber.New()
	app.Get("/v1/catalog/works", func(c fiber.Ctx) error {
		return c.SendString(worksListCacheControl(c))
	})
	for url, want := range map[string]string{
		"/v1/catalog/works":                cacheSearch,
		"/v1/catalog/works?status=live":    cacheSearch,
		"/v1/catalog/works?status=pending": cacheModeration,
	} {
		resp, err := app.Test(httptest.NewRequest("GET", url, nil))
		require.NoError(t, err)
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		assert.Equalf(t, want, string(buf[:n]), "cache header for %s", url)
	}
}
