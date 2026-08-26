package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api/internal/platform/devapi"
	"api/internal/platform/store/model"
	"api/internal/platform/store/service"
	"api/internal/platform/store/storetest"
	apperr "api/pkg/errors"

	"github.com/gofiber/fiber/v3"
)

type statsEnvelope struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *service.MyStats `json:"data"`
}

func mountStats(t *testing.T, clientID string) *fiber.App {
	t.Helper()
	app := fiber.New()
	h := NewPublicHandler(service.New(testDB, nil, service.Options{}))
	g := app.Group("/v1/store", func(c fiber.Ctx) error {
		devapi.WithCredential(c, &devapi.Credential{
			KeyID: 1, ClientID: clientID, Scopes: []string{devapi.ScopeStoreRead},
		})
		return c.Next()
	}, devapi.RequireScope(devapi.ScopeStoreRead))
	g.Get("/me/stats", h.MyStats)
	return app
}

func getStats(t *testing.T, app *fiber.App, path string) (*http.Response, statsEnvelope) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env statsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return resp, env
}

func seedStats(t *testing.T) (today, yesterday string) {
	t.Helper()
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	now := time.Now()
	today = model.JSTDay(now)
	yesterday = model.JSTDay(now.AddDate(0, 0, -1))

	links := []model.PurchaseLink{
		{ClientID: "site-a", ProductID: "RJ100001", Alias: "pa1", ShortURL: "https://s.test/pa1"},
		{ClientID: "site-b", ProductID: "RJ100001", Alias: "pb1", ShortURL: "https://s.test/pb1"},
	}
	for i := range links {
		if err := testDB.Create(&links[i]).Error; err != nil {
			t.Fatalf("seed purchase link: %v", err)
		}
	}
	coupon := model.CouponLink{ClientID: "site-a", CampaignID: 7, Alias: "ca1", ShortURL: "https://s.test/ca1"}
	if err := testDB.Create(&coupon).Error; err != nil {
		t.Fatalf("seed coupon link: %v", err)
	}

	stats := []model.LinkDailyStat{
		{Alias: "pa1", Day: yesterday, Total: 10, Uniques: 6, SyncedAt: now},
		{Alias: "pa1", Day: today, Total: 4, Uniques: 3, SyncedAt: now},
		{Alias: "ca1", Day: today, Total: 5, Uniques: 2, SyncedAt: now},
		{Alias: "pb1", Day: today, Total: 99, Uniques: 99, SyncedAt: now},
	}
	for i := range stats {
		if err := testDB.Create(&stats[i]).Error; err != nil {
			t.Fatalf("seed stat: %v", err)
		}
	}
	return today, yesterday
}

func TestMyStatsReturnsOnlyTheCallersOwnLinks(t *testing.T) {
	today, yesterday := seedStats(t)
	app := mountStats(t, "site-a")

	resp, env := getStats(t, app, "/v1/store/me/stats")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "private" {
		t.Errorf("Cache-Control = %q, want \"private\"", got)
	}
	if env.Data == nil {
		t.Fatal("data is null")
	}
	if len(env.Data.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 — site-b's 99 clicks must not be visible here (%+v)", len(env.Data.Rows), env.Data.Rows)
	}
	if env.Data.Totals.Total != 19 || env.Data.Totals.Uniques != 11 {
		t.Errorf("totals = %+v, want total=19 uniques=11", env.Data.Totals)
	}

	byKind := map[string]service.StatTotal{}
	for _, k := range env.Data.ByKind {
		byKind[k.Kind] = k
	}
	if p := byKind[model.KindPurchase]; p.Total != 14 || p.Uniques != 9 {
		t.Errorf("purchase totals = %+v, want total=14 uniques=9", p)
	}
	if c := byKind[model.KindCoupon]; c.Total != 5 || c.Uniques != 2 {
		t.Errorf("coupon totals = %+v, want total=5 uniques=2", c)
	}

	first := env.Data.Rows[0]
	if first.Date != yesterday {
		t.Errorf("rows are not day-ordered: first date = %q, want %q", first.Date, yesterday)
	}
	if first.ProductID == nil || *first.ProductID != "RJ100001" || first.CampaignID != nil {
		t.Errorf("purchase row = %+v, want product_id=RJ100001 campaign_id=null", first)
	}

	var coupon *service.StatRow
	for i := range env.Data.Rows {
		if env.Data.Rows[i].Kind == model.KindCoupon {
			coupon = &env.Data.Rows[i]
		}
	}
	if coupon == nil {
		t.Fatal("no coupon row")
	}
	if coupon.CampaignID == nil || *coupon.CampaignID != 7 || coupon.ProductID != nil {
		t.Errorf("coupon row = %+v, want campaign_id=7 product_id=null", coupon)
	}
	if coupon.Date != today {
		t.Errorf("coupon date = %q, want %q", coupon.Date, today)
	}
}

func TestMyStatsHonoursAnExplicitRange(t *testing.T) {
	_, yesterday := seedStats(t)
	app := mountStats(t, "site-a")

	_, env := getStats(t, app, "/v1/store/me/stats?from="+yesterday+"&to="+yesterday)
	if env.Data.From != yesterday || env.Data.To != yesterday {
		t.Errorf("range = %s..%s, want %s..%s", env.Data.From, env.Data.To, yesterday, yesterday)
	}
	if len(env.Data.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 (%+v)", len(env.Data.Rows), env.Data.Rows)
	}
	if env.Data.Totals.Total != 10 {
		t.Errorf("total = %d, want 10", env.Data.Totals.Total)
	}
}

func TestMyStatsRejectsAnImpossibleRange(t *testing.T) {
	seedStats(t)
	app := mountStats(t, "site-a")

	for _, q := range []string{
		"?from=2026-13-01",
		"?to=nonsense",
		"?from=2026-08-10&to=2026-08-09",
		"?from=2026-01-01&to=2026-08-01",
	} {
		resp, env := getStats(t, app, "/v1/store/me/stats"+q)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", q, resp.StatusCode)
		}
		if env.Code != apperr.ErrInvalidParam {
			t.Errorf("%s: code = %d, want %d", q, env.Code, apperr.ErrInvalidParam)
		}
	}
}

func TestMyStatsIsEmptyButWellFormedForANewCaller(t *testing.T) {
	seedStats(t)
	app := mountStats(t, "site-never-called")

	resp, env := getStats(t, app, "/v1/store/me/stats")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if env.Data == nil || env.Data.Rows == nil {
		t.Fatalf("data = %+v, want an empty rows array rather than null", env.Data)
	}
	if len(env.Data.Rows) != 0 || env.Data.Totals.Total != 0 {
		t.Errorf("data = %+v, want zeros", env.Data)
	}
	if len(env.Data.ByKind) != 2 {
		t.Errorf("by_kind = %+v, want both kinds present at zero", env.Data.ByKind)
	}
}

func TestMyStatsWorksWithoutTheShortenerConfigured(t *testing.T) {
	seedStats(t)
	// mountStats deliberately builds the service with a nil minter: reading the
	// cached counts is a local query and must not go dark when the mint leg is
	// unconfigured.
	app := mountStats(t, "site-a")

	if resp, _ := getStats(t, app, "/v1/store/me/stats"); resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
