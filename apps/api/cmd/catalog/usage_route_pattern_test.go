package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// setupPublicCatalog needs Postgres, Redis and Meilisearch, so this reproduces
// the shape recordUsage depends on rather than calling it: the recorder is a
// group-level Use that reads c.Route().Path AFTER c.Next() returns, and what it
// must get back is the registered PATTERN. A concrete id here would put one
// developer_api_usage row per work per key per day.
func routePatternApp(seen *[]string) *fiber.App {
	f := fiber.New()
	record := func(c fiber.Ctx) error {
		err := c.Next()
		*seen = append(*seen, c.Route().Path)
		return err
	}
	ok := func(c fiber.Ctx) error { return c.SendString("ok") }
	g := f.Group("/v1/catalog", record)
	g.Get("/works", ok)
	g.Get("/works/:id", ok)
	g.Get("/labels/:id/relation-graph", ok)
	g.Post("/lookup/batch", ok)
	return f
}

func TestUsageRecordsTheRoutePatternNotTheConcretePath(t *testing.T) {
	for _, tc := range []struct {
		method, path, want string
	}{
		{"GET", "/v1/catalog/works/123456", "/v1/catalog/works/:id"},
		{"GET", "/v1/catalog/works", "/v1/catalog/works"},
		{"GET", "/v1/catalog/labels/42/relation-graph", "/v1/catalog/labels/:id/relation-graph"},
		{"POST", "/v1/catalog/lookup/batch", "/v1/catalog/lookup/batch"},
		{"GET", "/v1/catalog/works/7?include=relations", "/v1/catalog/works/:id"},
		// An unmatched path under the prefix falls back to the group's own Use
		// route, which is still a bounded value — not the request path.
		{"GET", "/v1/catalog/no-such-route", "/v1/catalog"},
	} {
		var seen []string
		f := routePatternApp(&seen)
		if _, err := f.Test(httptest.NewRequest(tc.method, tc.path, nil)); err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if len(seen) != 1 || seen[0] != tc.want {
			t.Errorf("%s %s recorded %v, want [%s]", tc.method, tc.path, seen, tc.want)
		}
	}
}
