package galgameapp

import (
	"api/internal/app"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// The successor named /v1/catalog until wave R3 retired the whole v1 surface on
// 2026-08-27 — a tombstone that points at another tombstone.
const (
	retiredSuccessorLink = `<https://api.nextmoe.dev/v2>; rel="successor-version"`
	retiredPublicMessage = "the /v1/galgame face was retired on 2026-07-30; " +
		"use the v2 API instead — https://developer.nextmoe.dev/docs/v2"
)

func MountRetiredPublic(a *app.App) {
	gone := func(c fiber.Ctx) error {
		c.Set("Link", retiredSuccessorLink)
		return response.Error(c, fiber.StatusGone, errors.ErrGone, retiredPublicMessage)
	}
	a.Fiber.All("/v1/galgame", gone)
	a.Fiber.All("/v1/galgame/*", gone)
}
