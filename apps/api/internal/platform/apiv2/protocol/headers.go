package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"
)

const (
	cacheVocab  = "public, max-age=300, s-maxage=1800, stale-while-revalidate=3600"
	cacheError  = "no-store"
	varyPublic  = "Authorization, Accept-Encoding"
	serviceDesc = `<https://developer.nextmoe.dev/>; rel="service-desc"`
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
	path := c.Path()
	if isVocabPath(path) && c.GetRespHeader("Cache-Control") == "" {
		c.Set("Cache-Control", cacheVocab)
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

func isVocabPath(path string) bool {
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
