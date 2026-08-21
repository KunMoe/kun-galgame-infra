package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

const testCacheControl = "public, max-age=0, s-maxage=300"

// The middleware is mounted on the GROUP, so the test mounts it on a group too:
// what has to hold is that every route registered under the prefix inherits it,
// including ones added later by someone who never read this file.
func etagApp() *fiber.App {
	f := fiber.New()
	g := f.Group("/v1/catalog", ETag())
	g.Get("/works/:id", func(c fiber.Ctx) error {
		c.Set("Cache-Control", testCacheControl)
		return c.JSON(fiber.Map{"id": c.Params("id")})
	})
	g.Get("/missing", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"code": 404})
	})
	g.Post("/lookup/batch", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"items": []string{}})
	})
	// The shape /v1/catalog/releases and /v1/catalog/calendar already have: a
	// validator computed from a cheap meta query, checked before the page is
	// built.
	g.Get("/releases", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"cheap-meta-tag"`)
		c.Set("Cache-Control", testCacheControl)
		if c.Get(fiber.HeaderIfNoneMatch) == `"cheap-meta-tag"` {
			return c.SendStatus(fiber.StatusNotModified)
		}
		return c.JSON(fiber.Map{"items": []string{"a"}})
	})
	return f
}

func do(t *testing.T, f *fiber.App, method, path, ifNoneMatchHeader string) (int, string, string, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if ifNoneMatchHeader != "" {
		req.Header.Set(fiber.HeaderIfNoneMatch, ifNoneMatchHeader)
	}
	resp, err := f.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get(fiber.HeaderETag), resp.Header.Get("Cache-Control"), string(body)
}

func TestETagIsStrongAndBodyDerived(t *testing.T) {
	f := etagApp()

	status, tag, _, body := do(t, f, "GET", "/v1/catalog/works/1", "")
	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.HasPrefix(tag, `"`) || strings.HasPrefix(tag, `W/`) {
		t.Fatalf("etag = %q, want a strong quoted tag", tag)
	}
	if body != `{"id":"1"}` {
		t.Fatalf("body = %q (the middleware must not disturb it)", body)
	}

	_, same, _, _ := do(t, f, "GET", "/v1/catalog/works/1", "")
	if same != tag {
		t.Errorf("etag is not stable across identical responses: %q vs %q", tag, same)
	}
	_, other, _, _ := do(t, f, "GET", "/v1/catalog/works/2", "")
	if other == tag {
		t.Errorf("two different bodies share the etag %q", tag)
	}
}

func TestETagConditionalGet(t *testing.T) {
	f := etagApp()
	_, tag, _, _ := do(t, f, "GET", "/v1/catalog/works/1", "")

	for _, tc := range []struct {
		name   string
		header string
	}{
		{"exact", tag},
		{"weak candidate matches a strong tag", "W/" + tag},
		{"star", "*"},
		{"one of a list", `"nope", ` + tag},
	} {
		status, gotTag, cache, body := do(t, f, "GET", "/v1/catalog/works/1", tc.header)
		if status != fiber.StatusNotModified {
			t.Errorf("%s: status = %d, want 304", tc.name, status)
		}
		if body != "" {
			t.Errorf("%s: 304 carried a body %q", tc.name, body)
		}
		if gotTag != tag {
			t.Errorf("%s: 304 etag = %q, want %q — the client needs it to revalidate again", tc.name, gotTag, tag)
		}
		if cache != testCacheControl {
			t.Errorf("%s: 304 Cache-Control = %q, want %q", tc.name, cache, testCacheControl)
		}
	}

	status, _, _, body := do(t, f, "GET", "/v1/catalog/works/1", `"stale"`)
	if status != fiber.StatusOK || body != `{"id":"1"}` {
		t.Errorf("non-matching If-None-Match = %d %q, want a full 200", status, body)
	}
}

// A conditional POST must never become a 304: the request is not safe and the
// group carries POST routes (/lookup/batch, /resolve).
func TestETagSkipsPostAndNon200(t *testing.T) {
	f := etagApp()

	status, tag, _, body := do(t, f, "POST", "/v1/catalog/lookup/batch", "*")
	if status != fiber.StatusOK {
		t.Errorf("POST status = %d, want 200", status)
	}
	if tag != "" {
		t.Errorf("POST got an etag %q", tag)
	}
	if body != `{"items":[]}` {
		t.Errorf("POST body = %q", body)
	}

	status, tag, _, _ = do(t, f, "GET", "/v1/catalog/missing", "*")
	if status != fiber.StatusNotFound {
		t.Errorf("404 status = %d, want 404", status)
	}
	if tag != "" {
		t.Errorf("a 404 must not be validated, got etag %q", tag)
	}
}

// A handler that already validates keeps its own tag: the release/calendar feeds
// derive theirs from a meta query and skip building the page on a match, which
// a body hash computed after the fact cannot do.
func TestETagDefersToAHandlerThatAlreadyValidates(t *testing.T) {
	f := etagApp()

	status, tag, _, body := do(t, f, "GET", "/v1/catalog/releases", "")
	if status != fiber.StatusOK || tag != `"cheap-meta-tag"` {
		t.Fatalf("status/etag = %d %q, want 200 and the handler's own tag", status, tag)
	}
	if body != `{"items":["a"]}` {
		t.Errorf("body = %q", body)
	}

	status, tag, _, body = do(t, f, "GET", "/v1/catalog/releases", `"cheap-meta-tag"`)
	if status != fiber.StatusNotModified {
		t.Errorf("the handler's own conditional path = %d, want 304", status)
	}
	if body != "" {
		t.Errorf("304 carried a body %q", body)
	}
	if tag != `"cheap-meta-tag"` {
		t.Errorf("304 etag = %q, want the handler's tag", tag)
	}
}
