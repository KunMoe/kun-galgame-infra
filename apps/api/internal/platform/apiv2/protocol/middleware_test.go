package protocol

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/settings/keys"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func protoApp(t *testing.T) *fiber.App {
	t.Helper()
	store := NewMemory()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	app.Use(Middleware(store))
	app.Use(RateLimit(store, nil))
	app.Use(Idempotency(store, nil))
	app.Get("/v2/problems", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"object": "list", "items": []any{}})
	})
	app.Get("/v2/catalog/schemas/work", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"object": "object_schema", "target_object": "work", "fields": []any{}})
	})
	app.Post("/v2/probe", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"object": "probe", "ok": true})
	})
	return app
}

func do(t *testing.T, app *fiber.App, method, path string, hdr map[string]string, body string) (int, http.Header, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header, b
}

func TestProtocolHeadersOnSchema(t *testing.T) {
	app := protoApp(t)
	status, h, _ := do(t, app, http.MethodGet, "/v2/catalog/schemas/work", nil, "")
	require.Equal(t, 200, status)
	require.Equal(t, cacheVocab, h.Get("Cache-Control"))
}

func TestProtocolHeadersOnSuccess(t *testing.T) {
	app := protoApp(t)
	status, h, body := do(t, app, http.MethodGet, "/v2/problems", nil, "")
	require.Equal(t, 200, status)
	require.True(t, strings.HasPrefix(h.Get("X-Request-ID"), "req_"))
	require.Equal(t, cacheVocab, h.Get("Cache-Control"))
	require.Equal(t, varyPublic, h.Get("Vary"))
	require.Equal(t, serviceDesc, h.Get("Link"))
	require.NotEmpty(t, h.Get("ETag"))
	require.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
	require.Contains(t, h.Get("Access-Control-Expose-Headers"), "ETag")
	require.Contains(t, h.Get("RateLimit-Policy"), strconv.Itoa(int(keys.APIV2DefaultRatePerMinute.Get()))+";w=60")
	require.Contains(t, string(body), `"object"`)
}

func TestProtocolCORSPreflight(t *testing.T) {
	app := protoApp(t)
	status, h, body := do(t, app, http.MethodOptions, "/v2/problems", map[string]string{
		"Origin":                        "https://example.com",
		"Access-Control-Request-Method": "GET",
	}, "")
	require.Equal(t, 204, status)
	require.Empty(t, body)
	require.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
	require.Contains(t, h.Get("Access-Control-Allow-Headers"), "Authorization")
	require.Contains(t, h.Get("Access-Control-Allow-Headers"), "If-Match")
}

func TestProtocolETag304(t *testing.T) {
	app := protoApp(t)
	_, h, _ := do(t, app, http.MethodGet, "/v2/problems", nil, "")
	tag := h.Get("ETag")
	require.NotEmpty(t, tag)
	status, h2, body := do(t, app, http.MethodGet, "/v2/problems", map[string]string{
		"If-None-Match": "W/" + tag,
	}, "")
	require.Equal(t, 304, status)
	require.Empty(t, body)
	require.Equal(t, tag, h2.Get("ETag"))
	require.Equal(t, cacheVocab, h2.Get("Cache-Control"))
}

func TestProtocolErrorIsNoStoreProblem(t *testing.T) {
	app := protoApp(t)
	status, h, body := do(t, app, http.MethodGet, "/v2/missing", nil, "")
	require.Equal(t, 404, status)
	require.Contains(t, h.Get("Content-Type"), "application/problem+json")
	require.Equal(t, "no-store", h.Get("Cache-Control"))
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p))
	require.Equal(t, problem.CodeNotFound, p.Code)
}

func TestIdempotencyReplayAndConflict(t *testing.T) {
	app := protoApp(t)
	hdr := map[string]string{"Idempotency-Key": "k1"}
	status, h, body := do(t, app, http.MethodPost, "/v2/probe", hdr, `{"a":1}`)
	require.Equal(t, 201, status)
	require.Empty(t, h.Get("Idempotency-Replayed"))

	status, h, body2 := do(t, app, http.MethodPost, "/v2/probe", hdr, `{"a":1}`)
	require.Equal(t, 201, status)
	require.Equal(t, "true", h.Get("Idempotency-Replayed"))
	require.Equal(t, body, body2)

	status, h, body = do(t, app, http.MethodPost, "/v2/probe", hdr, `{"a":2}`)
	require.Equal(t, 409, status)
	require.Contains(t, h.Get("Content-Type"), "application/problem+json")
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p))
	require.Equal(t, problem.CodeIdempotencyKeyReused, p.Code)
}

func TestRateLimit429(t *testing.T) {
	store := NewMemory()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	app.Use(Middleware(store))
	app.Use(RateLimit(store, nil))
	app.Get("/v2/problems", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"object": "list"})
	})
	for i := 0; i < int(keys.APIV2DefaultRatePerMinute.Get()); i++ {
		status, _, _ := do(t, app, http.MethodGet, "/v2/problems", nil, "")
		require.Equal(t, 200, status)
	}
	status, h, body := do(t, app, http.MethodGet, "/v2/problems", nil, "")
	require.Equal(t, 429, status)
	require.NotEmpty(t, h.Get("Retry-After"))
	require.Contains(t, h.Get("Content-Type"), "application/problem+json")
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p))
	require.Equal(t, problem.CodeRateLimited, p.Code)
}

func keyedApp(t *testing.T, id LimitIdentity, ok bool) *fiber.App {
	t.Helper()
	store := NewMemory()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	app.Use(Middleware(store))
	app.Use(RateLimit(store, func(fiber.Ctx) (LimitIdentity, bool) { return id, ok }))
	app.Get("/v2/catalog/works", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"object": "list"})
	})
	return app
}

func TestRateLimitUsesKeyIdentity(t *testing.T) {
	app := keyedApp(t, LimitIdentity{Key: "k7", Rate: 2, Quota: 100}, true)
	for i := 0; i < 2; i++ {
		status, h, _ := do(t, app, http.MethodGet, "/v2/catalog/works", nil, "")
		require.Equal(t, 200, status)
		require.Equal(t, "2", h.Get("X-RateLimit-Limit"))
		require.Contains(t, h.Get("RateLimit-Policy"), "2;w=60, 100;w=86400")
	}
	status, _, body := do(t, app, http.MethodGet, "/v2/catalog/works", nil, "")
	require.Equal(t, 429, status)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p))
	require.Equal(t, problem.CodeRateLimited, p.Code)
}

func TestRateLimitKeyQuota(t *testing.T) {
	app := keyedApp(t, LimitIdentity{Key: "k8", Rate: 100, Quota: 3}, true)
	for i := 0; i < 3; i++ {
		status, _, _ := do(t, app, http.MethodGet, "/v2/catalog/works", nil, "")
		require.Equal(t, 200, status)
	}
	status, _, body := do(t, app, http.MethodGet, "/v2/catalog/works", nil, "")
	require.Equal(t, 429, status)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p))
	require.Equal(t, problem.CodeQuotaExceeded, p.Code)
}

func TestRateLimitUnlimitedTier(t *testing.T) {
	app := keyedApp(t, LimitIdentity{Key: "k9", Unlimited: true}, true)
	for i := 0; i < int(keys.APIV2DefaultRatePerMinute.Get())+5; i++ {
		status, h, _ := do(t, app, http.MethodGet, "/v2/catalog/works", nil, "")
		require.Equal(t, 200, status)
		require.Empty(t, h.Get("X-RateLimit-Limit"))
	}
}
