package protocol

import (
	"net/http"
	"testing"

	"api/internal/platform/apiv2/problem"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func cacheApp(t *testing.T) *fiber.App {
	t.Helper()
	store := NewMemory()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	app.Use(Middleware(store))
	app.Use(RateLimit(store, nil))
	app.Use(Idempotency(store, nil))
	for _, p := range []string{
		"/v2/catalog/works", "/v2/catalog/claim-events", "/v2/me/claims",
		"/v2/moderation/claims", "/v2/news", "/v2/catalog/stats",
		"/v2/catalog/schemas/work", "/v2/vocabularies", "/v2/problems",
		"/v2/catalog/openapi.json",
	} {
		app.Get(p, func(c fiber.Ctx) error {
			return c.JSON(fiber.Map{"object": "list", "items": []any{}})
		})
	}
	return app
}

// Every one of these declared no cache intent before the default-deny rule, so
// Cloudflare invented one: /v2/Catalog/works and /v2/Catalog/claim-events were
// found cached at max-age=14400, a value this origin never sets, one body
// carrying actor_uid.
func TestCacheIntentDefaultDeniesEveryCredentialedLane(t *testing.T) {
	app := cacheApp(t)
	for _, path := range []string{
		"/v2/catalog/works", "/v2/catalog/claim-events", "/v2/me/claims",
		"/v2/moderation/claims", "/v2/news", "/v2/catalog/stats",
	} {
		status, h, _ := do(t, app, http.MethodGet, path, nil, "")
		require.Equal(t, 200, status, path)
		require.Equal(t, cachePrivate, h.Get("Cache-Control"), path)
	}
}

// The control: default-deny that also denied the public lanes would prove
// nothing except that no-store can be stamped on everything.
func TestCacheIntentPublicLanesStillOptIn(t *testing.T) {
	app := cacheApp(t)
	for _, path := range []string{
		"/v2/problems", "/v2/vocabularies", "/v2/catalog/schemas/work",
		"/v2/catalog/openapi.json",
	} {
		status, h, _ := do(t, app, http.MethodGet, path, nil, "")
		require.Equal(t, 200, status, path)
		require.Equal(t, cacheVocab, h.Get("Cache-Control"), path)
		require.Contains(t, h.Get("Cache-Control"), "max-age=300", path)
	}
}

// fiber matched these to the public routes above (CaseSensitive and
// StrictRouting are both off here, as in cmd/oauth), while applyHeaders read
// the raw c.Path() and saw neither.
func TestCacheIntentReadsWhatFiberMatched(t *testing.T) {
	app := cacheApp(t)
	for _, path := range []string{
		"/v2/catalog/schemas/work/", "/v2/Catalog/schemas/work", "/v2/PROBLEMS",
		"/v2/vocabularies/",
	} {
		status, h, _ := do(t, app, http.MethodGet, path, nil, "")
		require.Equal(t, 200, status, path)
		require.Equal(t, cacheVocab, h.Get("Cache-Control"), path)
	}
}

// A trailing slash routes to the same handler with StrictRouting off, so it is
// the same write; keying the replay record on the raw path gave it a second
// key and let the write run twice under one Idempotency-Key.
func TestIdempotencyKeyIgnoresPathCasingAndTrailingSlash(t *testing.T) {
	store := NewMemory()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	app.Use(Middleware(store))
	app.Use(Idempotency(store, nil))
	calls := 0
	app.Post("/v2/probe", func(c fiber.Ctx) error {
		calls++
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"object": "probe", "calls": calls})
	})

	hdr := map[string]string{"Idempotency-Key": "k-slash"}
	status, _, first := do(t, app, http.MethodPost, "/v2/probe", hdr, `{"a":1}`)
	require.Equal(t, 201, status)
	require.Equal(t, 1, calls)

	for _, variant := range []string{"/v2/probe/", "/v2/Probe"} {
		status, h, body := do(t, app, http.MethodPost, variant, hdr, `{"a":1}`)
		require.Equal(t, 201, status, variant)
		require.Equal(t, "true", h.Get("Idempotency-Replayed"), variant)
		require.Equal(t, first, body, variant)
		require.Equal(t, 1, calls, variant)
	}
}
