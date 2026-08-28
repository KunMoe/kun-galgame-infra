package main

import (
	"context"
	"net/http/httptest"
	"testing"

	v2handler "api/internal/platform/apiv2/handler"
	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
)

// setupPublicCatalog needs Postgres, Redis and Meilisearch, so this mounts the
// real v2 route table under a stand-in for the usage recorder rather than
// calling it: the recorder is a prefix Use that reads c.Route().Path AFTER
// c.Next() returns, and what it must get back is the registered PATTERN. A
// concrete id here would put one developer_api_usage row per work per key per
// day. Wave R3 moved the recorder off the v1 groups onto /v2, so the patterns it
// sees are huma-registered ones.
//
// The Authorization header is load-bearing: without it catalogAuth answers 401
// from a middleware registered at "/" and c.Route().Path is "/" for every path,
// which would make this test green on nothing. A real credential store is
// load-bearing for the same reason — a nil one now answers 503 there.
func routePatternApp(seen *[]string) *fiber.App {
	f := fiber.New()
	f.Use("/v2", func(c fiber.Ctx) error {
		err := c.Next()
		*seen = append(*seen, c.Route().Path)
		return err
	})
	v2handler.SetupWith(f, v2handler.Options{
		LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
			if raw != probeKey {
				return nil, nil
			}
			return &devapi.Credential{
				KeyID: 1, ClientID: "probe", Tier: devapi.TierInternal,
				Scopes: []string{devapi.ScopeCatalogRead, devapi.ScopeStoreRead},
			}, nil
		},
	})
	return f
}

var probeKey = func() string {
	k, err := devapi.GenerateV2Key(true)
	if err != nil {
		panic(err)
	}
	return k
}()

func TestUsageRecordsTheRoutePatternNotTheConcretePath(t *testing.T) {
	for _, tc := range []struct {
		method, path, want string
	}{
		{"GET", "/v2/catalog/works/123456", "/v2/catalog/works/:id"},
		{"GET", "/v2/catalog/works", "/v2/catalog/works"},
		{"GET", "/v2/catalog/works/7?include=relations", "/v2/catalog/works/:id"},
		{"GET", "/v2/store/purchase-links/RJ01000000", "/v2/store/purchase-links/:product_id"},
		{"GET", "/v2/problems", "/v2/problems"},
		// An unmatched path under the prefix records the root middleware's own
		// route, which is still a bounded value — not the request path.
		{"GET", "/v2/no-such-route", "/"},
	} {
		var seen []string
		f := routePatternApp(&seen)
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer "+probeKey)
		if _, err := f.Test(r); err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if len(seen) != 1 || seen[0] != tc.want {
			t.Errorf("%s %s recorded %v, want [%s]", tc.method, tc.path, seen, tc.want)
		}
	}
}
