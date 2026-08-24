package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func recoverApp(t *testing.T, register func(*fiber.App)) (*fiber.App, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
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
		},
	})
	app.Use(Recover())
	register(app)
	return app, &buf
}

func TestRecoverConvertsPanicTo500(t *testing.T) {
	app, buf := recoverApp(t, func(app *fiber.App) {
		app.Use(RequestID())
		app.Get("/boom", func(c fiber.Ctx) error {
			panic("nil map write")
		})
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
		t.Fatalf("body = %v, want the generic 500; panic text must not leak", body)
	}

	var line map[string]any
	if err := json.NewDecoder(buf).Decode(&line); err != nil {
		t.Fatalf("decode log: %v (buf=%s)", err, buf.String())
	}
	if line["msg"] != "panic recovered" {
		t.Fatalf("msg = %v, want panic recovered", line["msg"])
	}
	if line["method"] != http.MethodGet || line["path"] != "/boom" {
		t.Fatalf("method/path = %v %v", line["method"], line["path"])
	}
	if line["panic"] != "nil map write" {
		t.Fatalf("panic = %v, want the panic value", line["panic"])
	}
	stack, _ := line["stack"].(string)
	if stack == "" {
		t.Fatal("stack missing from the log record")
	}
	if line["request_id"] == nil {
		t.Fatal("request_id missing: RequestID ran after Recover so Locals must be set")
	}
}

func TestRecoverOmitsRequestIDWhenUnset(t *testing.T) {
	app, buf := recoverApp(t, func(app *fiber.App) {
		app.Get("/boom", func(c fiber.Ctx) error {
			panic("x")
		})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil))
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}

	var line map[string]any
	if err := json.NewDecoder(buf).Decode(&line); err != nil {
		t.Fatalf("decode log: %v (buf=%s)", err, buf.String())
	}
	if _, ok := line["request_id"]; ok {
		t.Fatalf("request_id = %v, want omitted when Locals is unset", line["request_id"])
	}
}
