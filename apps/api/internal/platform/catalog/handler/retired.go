package handler

import (
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const RetiredSuccessorLink = `<https://api.nextmoe.dev/v2>; rel="successor-version"`

const retiredV1Message = "The v1 catalog API was retired on 2026-08-27. " +
	"Use the v2 API (/v2, spec at https://api.nextmoe.dev/v2/catalog/openapi.json, " +
	"docs at https://developer.nextmoe.dev). " +
	"This endpoint returns 410 for every path and method."

// RetiredV1Prefixes is the whole v1 surface the catalog binary used to serve.
// Every prefix here must be mounted AFTER the admin groups and the v2 setup:
// fiber matches in registration order, and /api/v1/catalog mounted early would
// swallow /api/v1/catalog/... paths that no longer exist but also anything a
// later group registers under the same prefix.
var RetiredV1Prefixes = []string{
	"/v1/catalog",
	"/v1/news",
	"/v1/store",
	"/v1/playtime",
	"/api/v1/catalog",
	"/api/v1/user/catalog",
}

func MountRetiredV1(app *fiber.App) {
	gone := func(c fiber.Ctx) error {
		c.Set("Link", RetiredSuccessorLink)
		return response.Error(c, fiber.StatusGone, errors.ErrGone, retiredV1Message)
	}
	for _, prefix := range RetiredV1Prefixes {
		app.All(prefix, gone)
		app.All(prefix+"/*", gone)
	}
}
