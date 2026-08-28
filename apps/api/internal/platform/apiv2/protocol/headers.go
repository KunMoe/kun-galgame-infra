package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"api/pkg/routepath"

	"github.com/gofiber/fiber/v3"
)

const (
	cacheVocab   = "public, max-age=300, s-maxage=1800, stale-while-revalidate=3600"
	cachePrivate = "private, no-store"
	cacheError   = "no-store"
	varyPublic   = "Authorization, Accept-Encoding"
	serviceDesc  = `<https://developer.nextmoe.dev/>; rel="service-desc"`
)

func applyETag(c fiber.Ctx) {
	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
		return
	}
	if c.Response().StatusCode() != fiber.StatusOK {
		return
	}
	if len(c.Response().Header.Peek(fiber.HeaderETag)) == 0 {
		body := c.Response().Body()
		if len(body) == 0 {
			return
		}
		sum := sha256.Sum256(body)
		c.Set(fiber.HeaderETag, `"`+hex.EncodeToString(sum[:16])+`"`)
	}
	tag := string(c.Response().Header.Peek(fiber.HeaderETag))
	if !IfNoneMatch(c.Get(fiber.HeaderIfNoneMatch), tag) {
		return
	}
	c.Response().ResetBody()
	c.Status(fiber.StatusNotModified)
}

func applyHeaders(c fiber.Ctx) {
	status := c.Response().StatusCode()
	if status >= 400 {
		if c.GetRespHeader("Cache-Control") == "" {
			c.Set("Cache-Control", cacheError)
		}
		return
	}
	if c.GetRespHeader("Cache-Control") == "" {
		if isPublicPath(routepath.Normalize(c.Path())) {
			c.Set("Cache-Control", cacheVocab)
		} else {
			c.Set("Cache-Control", cachePrivate)
		}
	}
	if c.GetRespHeader("Vary") == "" {
		c.Set("Vary", varyPublic)
	}
	if link := c.GetRespHeader("Link"); link == "" {
		c.Set("Link", serviceDesc)
	} else if !strings.Contains(link, `rel="service-desc"`) {
		c.Set("Link", link+", "+serviceDesc)
	}
}

// Cache intent is default-deny: a v2 response that names no public lane is
// private, no-store. The old rule stamped that on /v2/me/ and /v2/moderation/
// only, which is narrower than the credentialed surface — all of /v2/catalog/*
// is key-gated and its bodies vary by site fence, nsfw capability and
// claim_state scope, yet declared nothing, so an intermediary was free to
// invent an intent. One did: during the 2026-08-28 path-case window Cloudflare
// cached /v2/Catalog/claim-events and /v2/Catalog/works 200s for max-age=14400
// (a value this origin never sets), one body carrying actor_uid. Vary:
// Authorization was already set and did not save us. A third prefix would only
// be a fourth one waiting to happen; the public set is the opt-in, and it holds
// nothing whose body a credential can change. /v2/news is public but is
// deliberately not in it — no measurement says its body is credential-invariant
// and a wrong "public" is the failure being fixed.
func isPublicPath(path string) bool {
	return strings.HasPrefix(path, "/v2/problems") ||
		strings.HasPrefix(path, "/v2/vocabularies") ||
		strings.HasPrefix(path, "/v2/catalog/schemas/") ||
		path == "/v2/catalog/openapi.json"
}

func applyCORS(c fiber.Ctx) {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-None-Match, If-Match, Idempotency-Key")
	c.Set("Access-Control-Expose-Headers", "ETag, Link, RateLimit, RateLimit-Policy, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After, Deprecation, Sunset, X-Request-ID")
	c.Set("Access-Control-Max-Age", "86400")
}
