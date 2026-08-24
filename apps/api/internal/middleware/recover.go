package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
)

func Recover() fiber.Handler {
	return fiberrecover.New(fiberrecover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e any) {
			attrs := []any{
				"method", c.Method(),
				"path", c.Path(),
				"panic", e,
				"stack", string(debug.Stack()),
			}
			if requestID := c.Locals("request_id"); requestID != nil {
				attrs = append(attrs, "request_id", requestID)
			}
			slog.Error("panic recovered", attrs...)
		},
	})
}
