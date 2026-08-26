package main

import (
	"testing"

	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
)

// setupPublicCatalog needs Postgres, Redis and Meilisearch, so these pin the
// shape of the /v1/news group rather than calling it: the scope gate came off on
// 2026-08-25 and the credential requirement did not.
func newsGroupApp(gated bool) *fiber.App {
	f := fiber.New()
	mw := devapi.NewMiddleware(nil, nil)
	stack := []any{mw.ResolveCredential}
	if gated {
		stack = append(stack, devapi.RequireScope(devapi.ScopeNewsRead))
	}
	g := f.Group("/v1/news", stack...)
	g.Get("/sources", func(c fiber.Ctx) error { return c.SendString("sources") })
	return f
}

func TestNewsFaceStillNeedsAKey(t *testing.T) {
	if got := statusOf(t, newsGroupApp(false), "/v1/news/sources"); got != fiber.StatusUnauthorized {
		t.Errorf("anonymous GET /v1/news/sources = %d, want 401 — dropping the scope gate must not open the face", got)
	}
}

// A key minted before the retirement carries catalog:read and nothing else, and
// that key must now reach the news handlers. The gated variant is the control:
// without it a green test would only prove the injected credential was accepted
// somewhere, not that removing the scope gate is what let it through.
func TestNewsFaceTakesAKeyWithoutNewsRead(t *testing.T) {
	build := func(gated bool) *fiber.App {
		f := fiber.New()
		stack := []any{fiber.Handler(func(c fiber.Ctx) error {
			devapi.WithCredential(c, &devapi.Credential{KeyID: 1, Scopes: []string{devapi.ScopeCatalogRead}})
			return c.Next()
		})}
		if gated {
			stack = append(stack, devapi.RequireScope(devapi.ScopeNewsRead))
		}
		g := f.Group("/v1/news", stack...)
		g.Get("/sources", func(c fiber.Ctx) error { return c.SendString("sources") })
		return f
	}

	if got := statusOf(t, build(false), "/v1/news/sources"); got != fiber.StatusOK {
		t.Errorf("catalog:read key on the ungated news face = %d, want 200", got)
	}
	if got := statusOf(t, build(true), "/v1/news/sources"); got != fiber.StatusForbidden {
		t.Errorf("control: the same key behind RequireScope(news:read) = %d, want 403", got)
	}
}
