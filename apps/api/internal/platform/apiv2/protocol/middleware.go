package protocol

import (
	"net/http"
	"strings"

	"api/internal/platform/apiv2/problem"

	"github.com/gofiber/fiber/v3"
)

func Middleware(store Store) fiber.Handler {
	lim := newLimiter(store)
	return func(c fiber.Ctx) error {
		if !strings.HasPrefix(c.Path(), "/v2") {
			return c.Next()
		}
		applyCORS(c)
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		_ = problem.RequestID(c)

		if lim.authFailBlocked(c) {
			return writeErr(c, authFailRefusal(c))
		}

		err := c.Next()
		applyETag(c)
		lim.countAuthFailure(c)
		applyHeaders(c)

		status := c.Response().StatusCode()
		if err != nil && status < 400 {
			return writeErr(c, err)
		}
		if status >= 400 && !strings.Contains(string(c.Response().Header.ContentType()), "application/problem+json") {
			msg := http.StatusText(status)
			c.Response().ResetBody()
			return problem.WriteFiberError(c, fiber.NewError(status, msg))
		}
		return err
	}
}

func writeErr(c fiber.Ctx, err error) error {
	return problem.WriteFiberError(c, err)
}
