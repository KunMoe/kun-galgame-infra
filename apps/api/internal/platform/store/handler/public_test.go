package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"api/internal/platform/devapi"
	"api/internal/platform/store/model"
	"api/internal/platform/store/service"
	"api/internal/platform/store/storetest"
	apperr "api/pkg/errors"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	db, release, ok := storetest.Open()
	if !ok {
		os.Exit(0)
	}
	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

const (
	tmplManiax = "https://www.dlsite.com/maniax/dlaf/=/link/work/aid/test/id/{product_id}.html"
	tmplPro    = "https://www.dlsite.com/pro/dlaf/=/link/work/aid/test/id/{product_id}.html"
)

type envelope struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    *service.PurchaseLinks `json:"data"`
}

// mount reproduces the /v1/store middleware chain the catalog process installs:
// a resolved credential, then the store:read scope gate, then the handler.
func mount(t *testing.T, svc *service.Service, scopes []string) *fiber.App {
	t.Helper()
	app := fiber.New()
	h := NewPublicHandler(svc)
	g := app.Group("/v1/store", func(c fiber.Ctx) error {
		if scopes != nil {
			devapi.WithCredential(c, &devapi.Credential{KeyID: 1, ClientID: "site-a", Scopes: scopes})
		}
		return c.Next()
	}, devapi.RequireScope(devapi.ScopeStoreRead))
	g.Get("/purchase-links/:product_id", h.PurchaseLinks)
	return app
}

func newFixture(t *testing.T, quota int) (*fiber.App, *storetest.FakeShortener, *service.Service) {
	t.Helper()
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := service.New(testDB, fake.Client("slk_test"), service.Options{
		AffTemplateManiax:  tmplManiax,
		AffTemplatePro:     tmplPro,
		LinkQuotaPerClient: quota,
	})
	return mount(t, svc, []string{devapi.ScopeStoreRead}), fake, svc
}

func get(t *testing.T, app *fiber.App, path string) (*http.Response, envelope) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return resp, env
}

func TestPurchaseLinksReturnsBothSlotsAndIsUncacheable(t *testing.T) {
	app, _, _ := newFixture(t, 5000)
	now := time.Now()
	if err := testDB.Create(&model.Campaign{
		Name: "9 月券", CouponURL: "https://dlsite.test/coupon/september",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}

	resp, env := get(t, app, "/v1/store/purchase-links/RJ123456")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %+v)", resp.StatusCode, env)
	}
	if got := resp.Header.Get("Cache-Control"); got != "private" {
		t.Errorf("Cache-Control = %q, want \"private\" — a shared cache would hand one site's links to another", got)
	}
	if env.Code != 0 {
		t.Errorf("code = %d, want 0", env.Code)
	}
	if env.Data == nil || env.Data.PurchaseURL == "" {
		t.Fatalf("data = %+v, want a purchase_url", env.Data)
	}
	if env.Data.CouponURL == nil || env.Data.Campaign == nil {
		t.Fatalf("coupon slot empty while a campaign is running: %+v", env.Data)
	}
	if env.Data.ProductID != "RJ123456" {
		t.Errorf("product_id = %q, want RJ123456", env.Data.ProductID)
	}
}

func TestPurchaseLinksCouponSlotIsNullWithoutACampaign(t *testing.T) {
	app, _, _ := newFixture(t, 5000)

	resp, env := get(t, app, "/v1/store/purchase-links/RJ123456")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if env.Data.CouponURL != nil || env.Data.Campaign != nil {
		t.Errorf("coupon slot = %+v, want nulls", env.Data)
	}
}

func TestMalformedProductIDIs422(t *testing.T) {
	app, fake, _ := newFixture(t, 5000)

	for _, bad := range []string{"RJ12345", "RE123456", "rj123456"} {
		resp, env := get(t, app, "/v1/store/purchase-links/"+bad)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", bad, resp.StatusCode)
		}
		if env.Code != apperr.ErrStoreInvalidProduct {
			t.Errorf("%s: code = %d, want %d", bad, env.Code, apperr.ErrStoreInvalidProduct)
		}
	}
	if got := len(fake.Mints()); got != 0 {
		t.Errorf("mint calls = %d, want 0", got)
	}
}

func TestQuotaExhaustionIs403(t *testing.T) {
	app, _, _ := newFixture(t, 1)

	if resp, _ := get(t, app, "/v1/store/purchase-links/RJ100001"); resp.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp.StatusCode)
	}
	resp, env := get(t, app, "/v1/store/purchase-links/RJ100002")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if env.Code != apperr.ErrStoreQuotaExceeded {
		t.Errorf("code = %d, want %d", env.Code, apperr.ErrStoreQuotaExceeded)
	}
}

func TestShortenerDownIs5xxAndLeaksNoAffURL(t *testing.T) {
	app, fake, _ := newFixture(t, 5000)
	fake.MintFails = true

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/store/purchase-links/RJ123456", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode < 500 {
		t.Fatalf("status = %d, want 5xx", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(body), "dlsite.com") {
		t.Fatalf("body leaked a bare affiliate URL, which bypasses the click counter: %s", body)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if env.Code != apperr.ErrStoreLinkUnavailable {
		t.Errorf("code = %d, want %d", env.Code, apperr.ErrStoreLinkUnavailable)
	}
}

func TestMissingScopeIsRefused(t *testing.T) {
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := service.New(testDB, fake.Client("slk_test"), service.Options{
		AffTemplateManiax: tmplManiax, AffTemplatePro: tmplPro,
	})

	app := mount(t, svc, []string{devapi.ScopeCatalogRead, devapi.ScopeNewsRead})
	if resp, _ := get(t, app, "/v1/store/purchase-links/RJ123456"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("with catalog:read + news:read: status = %d, want 403", resp.StatusCode)
	}

	anon := mount(t, svc, nil)
	if resp, _ := get(t, anon, "/v1/store/purchase-links/RJ123456"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("with no credential: status = %d, want 401", resp.StatusCode)
	}
	if got := len(fake.Mints()); got != 0 {
		t.Errorf("mint calls = %d, want 0 — the gate must run before any minting", got)
	}
}

func TestUnconfiguredFaceIs503(t *testing.T) {
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	app := mount(t, service.New(testDB, nil, service.Options{}), []string{devapi.ScopeStoreRead})

	resp, env := get(t, app, "/v1/store/purchase-links/RJ123456")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	if env.Code != apperr.ErrStoreUnconfigured {
		t.Errorf("code = %d, want %d", env.Code, apperr.ErrStoreUnconfigured)
	}
	if got := resp.Header.Get("Cache-Control"); got != "private" {
		t.Errorf("Cache-Control = %q, want \"private\" even on the failure path", got)
	}
}
