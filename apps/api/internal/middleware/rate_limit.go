package middleware

import (
	"encoding/json"
	"time"

	"api/internal/infrastructure/cache"
	"api/internal/platform/settings/keys"
	"api/pkg/errors"
	"api/pkg/routepath"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

func RateLimit(redisCache *cache.RedisCache) fiber.Handler {
	config := limiter.Config{
		MaxFunc: func(_ fiber.Ctx) int {
			return int(keys.AuthIPRatePerMinute.Get())
		},
		Expiration: 1 * time.Minute,
		Next: func(c fiber.Ctx) bool {
			return c.Get("Authorization") != ""
		},
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    errors.ErrTooManyRequests,
				"message": errors.GetMessage(errors.ErrTooManyRequests),
			})
		},
	}

	if storage := redisCache.Storage(); storage != nil {
		config.Storage = storage
	}

	return limiter.New(config)
}

func OAuthTokenRateLimit(redisCache *cache.RedisCache) fiber.Handler {
	config := limiter.Config{
		MaxFunc: func(_ fiber.Ctx) int {
			return int(keys.AuthTokenEndpointRatePerMinute.Get())
		},
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			var body struct {
				ClientID string `json:"client_id"`
			}
			if err := json.Unmarshal(c.Body(), &body); err == nil && body.ClientID != "" {
				return "tokc:" + body.ClientID
			}
			return "tokip:" + c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    errors.ErrTooManyRequests,
				"message": errors.GetMessage(errors.ErrTooManyRequests),
			})
		},
	}

	if storage := redisCache.Storage(); storage != nil {
		config.Storage = storage
	}

	return limiter.New(config)
}

func StrictRateLimit(redisCache *cache.RedisCache) fiber.Handler {
	config := limiter.Config{
		MaxFunc: func(_ fiber.Ctx) int {
			return int(keys.AuthStrictRatePerMinute.Get())
		},
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP() + ":" + routepath.Normalize(c.Path())
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    errors.ErrTooManyRequests,
				"message": errors.GetMessage(errors.ErrTooManyRequests),
			})
		},
	}

	if storage := redisCache.Storage(); storage != nil {
		config.Storage = storage
	}

	return limiter.New(config)
}
