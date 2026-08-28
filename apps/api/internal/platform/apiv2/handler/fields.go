package handler

import (
	"strings"

	"api/internal/platform/apiv2/collect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

// fields= was declared on 40 operations, validated down to 400 UNKNOWN_FIELD
// on an unknown token, and then never applied to anything: the response always
// carried the full projection. It is honored here, once, on the way out —
// before protocol.Middleware computes the body ETag, so the validator moves
// with the token set for free.
//
// The gate is the operation's own declared parameter list, not the presence of
// the query string. Projecting wherever fields= appears would honor it on the
// ~48 operations that never declared it, including /v2/catalog/openapi.json,
// and would trade an ignored parameter for an undeclared one.
func fieldsProjection(declared map[string]bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		err := c.Next()
		raw := c.Query("fields")
		if strings.TrimSpace(raw) == "" {
			return err
		}
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return err
		}
		if c.Response().StatusCode() != fiber.StatusOK {
			return err
		}
		route := c.Route()
		if route == nil || !declared[c.Method()+" "+route.Path] {
			return err
		}
		if !strings.Contains(string(c.Response().Header.ContentType()), "json") {
			return err
		}
		out, perr := collect.ApplyFields(c.Response().Body(), strings.Split(raw, ","))
		if perr != nil {
			return err
		}
		c.Response().SetBody(out)
		return err
	}
}

// humafiber rewrites {id} to :id when it registers, so the document's paths are
// keyed the same way here rather than converting c.Route().Path back.
func declaredFieldsOps(doc *huma.OpenAPI, into map[string]bool) {
	for path, item := range doc.Paths {
		fiberPath := strings.NewReplacer("{", ":", "}", "").Replace(path)
		for _, op := range pathOps(item) {
			for _, p := range op.Parameters {
				if p != nil && p.Name == "fields" && p.In == "query" {
					into[op.Method+" "+fiberPath] = true
					break
				}
			}
		}
	}
}
