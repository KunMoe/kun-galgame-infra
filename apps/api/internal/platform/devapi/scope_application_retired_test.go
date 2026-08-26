package devapi

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// The grant-only scope application retired on 2026-08-25 with the news:read
// gate it existed to hand out. Both route tables are read back rather than
// probed with a request: a 404 from a request would also be what a route
// registered under a typo returns. The live routes are the positive control —
// without them an empty route table would pass this vacuously.
func TestScopeApplicationRoutesAreGone(t *testing.T) {
	app := fiber.New()
	noop := func(c fiber.Ctx) error { return c.Next() }
	NewSelfServiceHandler(nil).Register(app.Group("/dev"))
	NewAdminHandler(nil).Register(app.Group("/admin/devapi"), noop)

	var paths []string
	for _, r := range app.GetRoutes() {
		paths = append(paths, r.Path)
		if strings.Contains(r.Path, "scope-application") {
			t.Errorf("route %s %s survived the scope-application retirement", r.Method, r.Path)
		}
	}

	for _, want := range []string{"/dev/apps", "/admin/devapi/keys"} {
		found := false
		for _, p := range paths {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("positive control: %s is missing, so this test proves nothing about %d routes", want, len(paths))
		}
	}
}
