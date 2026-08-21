package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ETag tags a 200 GET with a strong validator hashed from the response body and
// answers a matching If-None-Match with a bodyless 304. Mount it on a route
// GROUP so routes added later inherit it.
func ETag() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}
		if err := c.Next(); err != nil {
			return err
		}
		if c.Response().StatusCode() != fiber.StatusOK {
			return nil
		}
		// /v1/catalog/releases and the three /v1/catalog/calendar routes already
		// validate, and with a BETTER tag than this one: theirs is
		// (population key, count, max updated_at), computed from a meta query,
		// so a match skips building the page at all. Recomputing here would
		// overwrite that with a hash the handler cannot recognise on the next
		// request, and the client would pay for the page every time.
		if len(c.Response().Header.Peek(fiber.HeaderETag)) > 0 {
			return nil
		}
		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}
		sum := sha256.Sum256(body)
		tag := `"` + hex.EncodeToString(sum[:16]) + `"`
		c.Set(fiber.HeaderETag, tag)
		if !ifNoneMatch(c.Get(fiber.HeaderIfNoneMatch), tag) {
			return nil
		}
		// Status + ResetBody rather than SendStatus: SendStatus fills an empty
		// body with the status text, and the 304 must keep the Cache-Control and
		// ETag the caller needs to revalidate again.
		c.Response().ResetBody()
		c.Status(fiber.StatusNotModified)
		return nil
	}
}

// If-None-Match uses the weak comparison function (RFC 9110 §13.1.2), so a
// W/-prefixed candidate matches our strong tag.
func ifNoneMatch(header, tag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == tag {
			return true
		}
	}
	return false
}
