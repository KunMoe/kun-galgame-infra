package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/devapi"
	"api/internal/platform/settings"
	"api/internal/platform/settings/keys"
	storemodel "api/internal/platform/store/model"
	storesvc "api/internal/platform/store/service"
	"api/internal/platform/store/storetest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	liveStoreKey     = mustLiveV2Key()
	liveStoreKeyNoSc = mustLiveV2Key()
	liveStoreClient  = "site-a"
	liveStoreMigrate sync.Once
)

const (
	liveStoreManiax = "https://www.dlsite.com/maniax/dlaf/=/link/work/aid/test/id/{product_id}.html"
	liveStorePro    = "https://www.dlsite.com/pro/dlaf/=/link/work/aid/test/id/{product_id}.html"
)

func liveStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := liveCatalog(t).db
	liveStoreMigrate.Do(func() {
		require.NoError(t, db.AutoMigrate(storemodel.AllModels()...))
	})
	require.NoError(t, storetest.Truncate(db))
	return db
}

// liveStoreApp mounts ONLY the v2 face over the given store service, so the
// assertions below are about /v2/store and never about the v1 handler.
func liveStoreApp(t *testing.T, svc *storesvc.Service) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	SetupWith(app, Options{
		Store:   &liveUnlimitedStore{},
		Catalog: &Catalog{Store: svc},
		LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
			switch raw {
			case liveStoreKey:
				return &devapi.Credential{
					KeyID: 11, ClientID: liveStoreClient,
					Scopes: []string{devapi.ScopeCatalogRead, devapi.ScopeStoreRead},
				}, nil
			case liveStoreKeyNoSc:
				return &devapi.Credential{
					KeyID: 12, ClientID: "site-b", Scopes: []string{devapi.ScopeCatalogRead},
				}, nil
			default:
				return nil, nil
			}
		},
	})
	return app
}

func liveStoreFixture(t *testing.T, quota int) (*fiber.App, *storetest.FakeShortener) {
	t.Helper()
	db := liveStoreDB(t)
	settings.Override(t, keys.StoreLinkQuotaPerClient, int64(quota))
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := storesvc.New(db, fake.Client("slk_test"), storesvc.Options{
		AffTemplateManiax: liveStoreManiax, AffTemplatePro: liveStorePro,
	})
	return liveStoreApp(t, svc), fake
}

func liveStoreGet(t *testing.T, app *fiber.App, path, token string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, body
}

type liveStoreLinks struct {
	Object      string  `json:"object"`
	ProductID   string  `json:"product_id"`
	PurchaseURL string  `json:"purchase_url"`
	CouponURL   *string `json:"coupon_url"`
	Campaign    *struct {
		Object string `json:"object"`
		ID     string `json:"id"`
		Name   string `json:"name"`
	} `json:"campaign"`
}

func TestLiveStorePurchaseLinks(t *testing.T) {
	app, fake := liveStoreFixture(t, 5000)

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/RJ123456", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Equal(t, "private", resp.Header.Get("Cache-Control"),
		"every response is keyed to the calling application; a shared cache would misattribute the clicks")

	var links liveStoreLinks
	require.NoError(t, json.Unmarshal(body, &links))
	require.Equal(t, "purchase_links", links.Object)
	require.Equal(t, "RJ123456", links.ProductID)
	require.NotEmpty(t, links.PurchaseURL)
	require.Nil(t, links.CouponURL, "no campaign is running")
	require.Nil(t, links.Campaign)

	mints := fake.Mints()
	require.Len(t, mints, 1)
	require.Equal(t,
		"https://www.dlsite.com/maniax/dlaf/=/link/work/aid/test/id/RJ123456.html",
		mints[0].DestinationURL, "RJ is served by the maniax template")

	// The alias is minted once and then reused; a second call must not mint again.
	resp, body = liveStoreGet(t, app, "/v2/store/purchase-links/RJ123456", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Len(t, fake.Mints(), 1)
}

func TestLiveStorePurchaseLinksWithCampaign(t *testing.T) {
	app, _ := liveStoreFixture(t, 5000)
	db := liveCatalog(t).db
	now := time.Now()
	campaign := storemodel.Campaign{
		Name: "Summer Sale", CouponURL: "https://example.test/coupon",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	}
	require.NoError(t, db.Create(&campaign).Error)

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/VJ012345", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	var links liveStoreLinks
	require.NoError(t, json.Unmarshal(body, &links))
	require.NotNil(t, links.CouponURL)
	require.NotEmpty(t, *links.CouponURL)
	require.NotNil(t, links.Campaign)
	require.Equal(t, "campaign", links.Campaign.Object)
	require.Equal(t, idstr(campaign.ID), links.Campaign.ID, "G7: every id is a JSON string")
	require.Equal(t, "Summer Sale", links.Campaign.Name)
}

func TestLiveStorePurchaseLinksFailures(t *testing.T) {
	app, _ := liveStoreFixture(t, 1)

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/RJ12", liveStoreKey)
	require.Equal(t, 422, resp.StatusCode, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "product_id", p.Errors[0].Parameter)
	require.Equal(t, problem.ReasonInvalidFormat, p.Errors[0].Reason)

	resp, body = liveStoreGet(t, app, "/v2/store/purchase-links/RJ200001", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	resp, body = liveStoreGet(t, app, "/v2/store/purchase-links/RJ200002", liveStoreKey)
	require.Equal(t, 403, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeStoreQuotaExceeded, liveProblem(t, body).Code)

}

// The quota is counted BEFORE the mint, so a quota-1 fixture never reaches the
// shortener and the down-shortener arm needs its own service.
func TestLiveStoreShortenerDown(t *testing.T) {
	app, fake := liveStoreFixture(t, 5000)
	fake.MintFails = true

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/VJ300001", liveStoreKey)
	require.Equal(t, 502, resp.StatusCode, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeStoreLinkUnavailable, p.Code)
	require.Contains(t, p.Detail, "no link was issued")
}

func TestLiveStoreUnconfigured(t *testing.T) {
	db := liveStoreDB(t)

	// A service with no minter, which is what a deployment without the shortener
	// credentials builds. Minting is dead there; the stats read is NOT — it only
	// touches the database, and v1 answered it. The first cut of this face gated
	// both ops on Configured() and lost that read.
	app := liveStoreApp(t, storesvc.New(db, nil, storesvc.Options{}))

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/RJ123456", liveStoreKey)
	require.Equal(t, 503, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeServiceUnavailable, liveProblem(t, body).Code)

	resp, body = liveStoreGet(t, app, "/v2/store/stats", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Equal(t, "private", resp.Header.Get("Cache-Control"))
	var stats liveStoreStats
	require.NoError(t, json.Unmarshal(body, &stats))
	require.Equal(t, "store_stats", stats.Object)
	require.Empty(t, stats.Rows, "nothing was ever minted, so there is nothing to report")
	require.NotNil(t, stats.Rows, "rows is an empty array, never null")
	require.Zero(t, stats.Totals.Total)
	require.Zero(t, stats.Totals.Uniques)
	require.Len(t, stats.ByKind, 2, "both kinds are still published, at zero")

	// An unbound service is the one case both ops refuse.
	app = liveStoreApp(t, nil)
	for _, path := range []string{"/v2/store/purchase-links/RJ123456", "/v2/store/stats"} {
		resp, body = liveStoreGet(t, app, path, liveStoreKey)
		require.Equal(t, 503, resp.StatusCode, string(body))
		require.Equal(t, problem.CodeServiceUnavailable, liveProblem(t, body).Code, path)
	}
}

func TestLiveStoreScopeGate(t *testing.T) {
	app, _ := liveStoreFixture(t, 5000)

	resp, body := liveStoreGet(t, app, "/v2/store/purchase-links/RJ123456", "")
	require.Equal(t, 401, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeMissingCredential, liveProblem(t, body).Code)

	resp, body = liveStoreGet(t, app, "/v2/store/purchase-links/RJ123456", liveStoreKeyNoSc)
	require.Equal(t, 403, resp.StatusCode, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeScopeRequired, p.Code)
	require.Contains(t, p.Detail, "store:read")

	resp, body = liveStoreGet(t, app, "/v2/store/stats", liveStoreKeyNoSc)
	require.Equal(t, 403, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeScopeRequired, liveProblem(t, body).Code)
}

type liveStoreTotal struct {
	LinkKind *string `json:"link_kind"`
	Total    int     `json:"total"`
	Uniques  int     `json:"uniques"`
}

type liveStoreStats struct {
	Object   string `json:"object"`
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
	Rows     []struct {
		Object     string  `json:"object"`
		LinkKind   string  `json:"link_kind"`
		ProductID  *string `json:"product_id"`
		CampaignID *string `json:"campaign_id"`
		Date       string  `json:"date"`
		Total      int     `json:"total"`
		Uniques    int     `json:"uniques"`
	} `json:"rows"`
	Totals liveStoreTotal   `json:"totals"`
	ByKind []liveStoreTotal `json:"by_kind"`
}

func TestLiveStoreStats(t *testing.T) {
	app, _ := liveStoreFixture(t, 5000)
	db := liveCatalog(t).db
	now := time.Now()
	today := storemodel.JSTDay(now)
	yesterday := storemodel.JSTDay(now.AddDate(0, 0, -1))

	for _, link := range []storemodel.PurchaseLink{
		{ClientID: liveStoreClient, ProductID: "RJ100001", Alias: "pa1", ShortURL: "https://s.test/pa1"},
		{ClientID: "site-b", ProductID: "RJ100001", Alias: "pb1", ShortURL: "https://s.test/pb1"},
	} {
		require.NoError(t, db.Create(&link).Error)
	}
	require.NoError(t, db.Create(&storemodel.CouponLink{
		ClientID: liveStoreClient, CampaignID: 7, Alias: "ca1", ShortURL: "https://s.test/ca1",
	}).Error)
	for _, stat := range []storemodel.LinkDailyStat{
		{Alias: "pa1", Day: yesterday, Total: 10, Uniques: 6, SyncedAt: now},
		{Alias: "pa1", Day: today, Total: 4, Uniques: 3, SyncedAt: now},
		{Alias: "ca1", Day: today, Total: 5, Uniques: 2, SyncedAt: now},
		{Alias: "pb1", Day: today, Total: 99, Uniques: 99, SyncedAt: now},
	} {
		require.NoError(t, db.Create(&stat).Error)
	}

	resp, body := liveStoreGet(t, app, "/v2/store/stats?from="+yesterday+"&to="+today, liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Equal(t, "private", resp.Header.Get("Cache-Control"))
	var stats liveStoreStats
	require.NoError(t, json.Unmarshal(body, &stats))
	require.Equal(t, "store_stats", stats.Object)
	require.Equal(t, yesterday, stats.FromDate)
	require.Equal(t, today, stats.ToDate)
	require.Len(t, stats.Rows, 3, "site-b's 99 clicks belong to another application")
	require.Equal(t, 19, stats.Totals.Total)
	require.Equal(t, 11, stats.Totals.Uniques)
	require.Nil(t, stats.Totals.LinkKind, "the grand total names no kind")

	byKind := map[string]liveStoreTotal{}
	for _, k := range stats.ByKind {
		require.NotNil(t, k.LinkKind)
		byKind[*k.LinkKind] = k
	}
	require.Equal(t, 14, byKind[storemodel.KindPurchase].Total)
	require.Equal(t, 9, byKind[storemodel.KindPurchase].Uniques)
	require.Equal(t, 5, byKind[storemodel.KindCoupon].Total)
	require.Equal(t, 2, byKind[storemodel.KindCoupon].Uniques)

	for _, row := range stats.Rows {
		require.Equal(t, "store_stat", row.Object)
		switch row.LinkKind {
		case storemodel.KindPurchase:
			require.NotNil(t, row.ProductID)
			require.Nil(t, row.CampaignID)
		case storemodel.KindCoupon:
			require.Nil(t, row.ProductID)
			require.NotNil(t, row.CampaignID)
			require.Equal(t, "7", *row.CampaignID, "G7: campaign_id is a JSON string")
		default:
			t.Fatalf("unexpected link_kind %q", row.LinkKind)
		}
	}

	resp, body = liveStoreGet(t, app, "/v2/store/stats?from="+today+"&to="+yesterday, liveStoreKey)
	require.Equal(t, 422, resp.StatusCode, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 2)

	resp, body = liveStoreGet(t, app, "/v2/store/stats?from=not-a-day", liveStoreKey)
	require.Equal(t, 422, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeValidationFailed, liveProblem(t, body).Code)

	// Absent from/to defaults to the last 30 JST days ending today.
	resp, body = liveStoreGet(t, app, "/v2/store/stats", liveStoreKey)
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.NoError(t, json.Unmarshal(body, &stats))
	require.Equal(t, today, stats.ToDate)
	require.Len(t, stats.Rows, 3)
}
