package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/middleware"

	"github.com/gofiber/fiber/v3"
)

func TestRecoverReachesTheRealErrorHandler(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: errorHandler})
	app.Use(middleware.Recover())
	app.Get("/boom", func(c fiber.Ctx) error {
		var m map[string]int
		m["nil map write"] = 1
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil))
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != float64(500) || body["message"] != "Internal Server Error" {
		t.Fatalf("body = %v, want the generic 500 — the runtime panic text must not reach the client", body)
	}
}
