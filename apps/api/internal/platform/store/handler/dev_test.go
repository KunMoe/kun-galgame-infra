package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/platform/store/model"
	"api/internal/platform/store/service"

	"github.com/gofiber/fiber/v3"
)

type devEnvelope struct {
	Code int                        `json:"code"`
	Data *service.OwnerUsageSummary `json:"data"`
}

func mountDev(t *testing.T, ownerID uint, apps []service.OwnerApp) *fiber.App {
	t.Helper()
	app := fiber.New()
	h := NewDevHandler(
		service.New(testDB, nil, service.Options{}),
		func(context.Context, uint) ([]service.OwnerApp, error) { return apps, nil },
	)
	g := app.Group("/dev", func(c fiber.Ctx) error {
		if ownerID != 0 {
			c.Locals("user_id", ownerID)
		}
		return c.Next()
	})
	h.Register(g)
	return app
}

func getDev(t *testing.T, app *fiber.App, path string) (*http.Response, devEnvelope) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env devEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return resp, env
}

func TestOwnerUsageAggregatesTheOwnersAppsOnly(t *testing.T) {
	today, _ := seedStats(t)
	app := mountDev(t, 1, []service.OwnerApp{{ClientID: "site-a", Name: "站 A"}})

	resp, env := getDev(t, app, "/dev/store/usage?days=7")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if env.Data == nil {
		t.Fatal("data is null")
	}
	if env.Data.Total != 19 || env.Data.Uniques != 11 {
		t.Errorf("totals = %d/%d, want 19/11 — site-b must not be counted", env.Data.Total, env.Data.Uniques)
	}
	if env.Data.LinkCount != 2 {
		t.Errorf("link_count = %d, want 2 (one purchase + one coupon)", env.Data.LinkCount)
	}
	if len(env.Data.Daily) != 7 {
		t.Errorf("daily = %d entries, want 7 dense days", len(env.Data.Daily))
	}
	if last := env.Data.Daily[len(env.Data.Daily)-1]; last.Day != today {
		t.Errorf("last dense day = %q, want today %q", last.Day, today)
	}
	if len(env.Data.ByApp) != 1 || env.Data.ByApp[0].ClientID != "site-a" {
		t.Fatalf("by_app = %+v, want one site-a row", env.Data.ByApp)
	}
	if env.Data.ByApp[0].Links != 2 {
		t.Errorf("by_app links = %d, want 2", env.Data.ByApp[0].Links)
	}
	if len(env.Data.ByLink) != 2 {
		t.Fatalf("by_link = %+v, want a purchase row and a coupon row", env.Data.ByLink)
	}
	kinds := map[string]bool{}
	for _, l := range env.Data.ByLink {
		kinds[l.Kind] = true
		if l.AppName != "站 A" {
			t.Errorf("by_link app_name = %q, want 站 A", l.AppName)
		}
	}
	if !kinds[model.KindPurchase] || !kinds[model.KindCoupon] {
		t.Errorf("by_link kinds = %v, want both", kinds)
	}
}

func TestOwnerUsageIsEmptyButRenderableForANewOwner(t *testing.T) {
	seedStats(t)
	app := mountDev(t, 1, nil)

	resp, env := getDev(t, app, "/dev/store/usage")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if env.Data.Days != service.DefaultOwnerUsageDays {
		t.Errorf("days = %d, want the %d-day default", env.Data.Days, service.DefaultOwnerUsageDays)
	}
	if len(env.Data.Daily) != service.DefaultOwnerUsageDays {
		t.Errorf("daily = %d entries, want %d", len(env.Data.Daily), service.DefaultOwnerUsageDays)
	}
	if env.Data.ByApp == nil || env.Data.ByLink == nil {
		t.Errorf("by_app/by_link are null; the panel needs empty arrays: %+v", env.Data)
	}
	if env.Data.LinkCount != 0 || env.Data.Total != 0 {
		t.Errorf("data = %+v, want zeros", env.Data)
	}
}

func TestOwnerUsageAppWithNoLinksIsOmitted(t *testing.T) {
	seedStats(t)
	app := mountDev(t, 1, []service.OwnerApp{
		{ClientID: "site-a", Name: "站 A"},
		{ClientID: "site-idle", Name: "没接入的应用"},
	})

	_, env := getDev(t, app, "/dev/store/usage?days=7")
	for _, row := range env.Data.ByApp {
		if row.ClientID == "site-idle" {
			t.Errorf("by_app includes an app that has minted nothing: %+v", row)
		}
	}
}

func TestOwnerUsageDayCountIsClamped(t *testing.T) {
	seedStats(t)
	app := mountDev(t, 1, []service.OwnerApp{{ClientID: "site-a", Name: "站 A"}})

	_, env := getDev(t, app, "/dev/store/usage?days=9999")
	if env.Data.Days != service.MaxOwnerUsageDays {
		t.Errorf("days = %d, want the %d cap", env.Data.Days, service.MaxOwnerUsageDays)
	}
}

func TestOwnerUsageNeedsASignedInOwner(t *testing.T) {
	seedStats(t)
	app := mountDev(t, 0, []service.OwnerApp{{ClientID: "site-a", Name: "站 A"}})

	if resp, _ := getDev(t, app, "/dev/store/usage"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
