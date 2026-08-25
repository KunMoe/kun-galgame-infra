package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

func CORS(frontendOrigin string) fiber.Handler {
	origins := []string{
		"https://kungal.com",
		"https://moyu.moe",
	}
	for o := range strings.SplitSeq(frontendOrigin, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return cors.New(cors.Config{
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/v2")
		},
		AllowOrigins: origins,
		AllowMethods: []string{
			fiber.MethodGet,
			fiber.MethodPost,
			fiber.MethodPut,
			fiber.MethodPatch,
			fiber.MethodDelete,
			fiber.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
			"X-Kun-Image-Client-Id",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
		},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
