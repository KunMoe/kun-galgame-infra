package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"api/internal/infrastructure/cache"
	"api/internal/infrastructure/database"
	"api/internal/middleware"
	"api/pkg/config"

	"github.com/gofiber/fiber/v3"
)

type App struct {
	Config *config.Config
	Fiber  *fiber.App
	DB     *database.PostgresDB
	Cache  *cache.RedisCache
}

type Options struct {
	Name      string
	NeedCache bool
}

func New(cfg *config.Config, opts Options) (*App, error) {
	a := &App{Config: cfg}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	a.DB = db

	if opts.NeedCache {
		redisCache, err := cache.NewRedisCache(cfg.Redis)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to redis: %w", err)
		}
		a.Cache = redisCache
	}

	name := opts.Name
	if name == "" {
		name = "kun-api"
	}
	a.Fiber = fiber.New(fiber.Config{
		AppName:        name,
		ServerHeader:   name,
		ErrorHandler:   errorHandler,
		ReadBufferSize: 32 * 1024,
		// Real client IP behind Cloudflare → Traefik (dokploy). Without this,
		// c.IP() returns the immediate peer (Traefik, ~10.0.1.x) for EVERY user,
		// which silently makes the rate limiters global-per-proxy (the strict
		// 10/min would throttle all logins site-wide) and stores a useless proxy
		// IP on sessions/audit. Cloudflare sets CF-Connecting-IP to the true
		// client; TrustProxy+Private only honors the header when the PEER is a
		// trusted private proxy (Traefik) — it does NOT verify the header value.
		// So this is only spoof-proof when the origin is locked to Cloudflare IPs
		// (infra hardening, see docs/runbook): a request reaching Traefik OFF the
		// CF path could forge CF-Connecting-IP and bypass the IP rate limiters.
		// EnableIPValidation rejects malformed header values. In dev (loopback
		// peer / no CF header) c.IP() falls back to the direct connection IP.
		ProxyHeader:        "CF-Connecting-IP",
		TrustProxy:         true,
		EnableIPValidation: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Private: true,
		},
	})
	// First on purpose: a recover registered later still lets panics drop the connection with no 500.
	a.Fiber.Use(middleware.Recover())

	return a, nil
}

func (a *App) Run(host string, port int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		addr := fmt.Sprintf("%s:%d", host, port)
		slog.Info("starting server", "addr", addr)
		if err := a.Fiber.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server...")

	if err := a.Fiber.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			slog.Error("failed to close database", "error", err)
		}
	}
	if a.Cache != nil {
		if err := a.Cache.Close(); err != nil {
			slog.Error("failed to close cache", "error", err)
		}
	}

	slog.Info("server stopped")
	return nil
}

func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"code":    code,
		"message": message,
	})
}
