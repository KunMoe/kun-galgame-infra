package handler

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"
	"testing"
	"time"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/model"
	storemodel "api/internal/platform/store/model"
	"api/internal/platform/store/price"
	"api/internal/platform/store/storetest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

var (
	livePricesMigrate  sync.Once
	livePricesSeedOnce sync.Once
)

var amountRe = regexp.MustCompile(`^[0-9]+\.[0-9]{2}$`)

type livePriceFake struct {
	mu      sync.Mutex
	source  string
	regions []string
	calls   int
}

func (f *livePriceFake) Source() string { return f.source }
func (f *livePriceFake) Regions() []string {
	return append([]string{}, f.regions...)
}
func (f *livePriceFake) Batch() int           { return 100 }
func (f *livePriceFake) Gap() time.Duration   { return 0 }
func (f *livePriceFake) Accepts(string) bool  { return true }
func (f *livePriceFake) URL(id string) string { return "https://example.test/" + f.source + "/" + id }
func (f *livePriceFake) Fetch(_ context.Context, _ string, ids []string) (map[string]price.Upstream, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	out := map[string]price.Upstream{}
	for _, id := range ids {
		out[id] = price.Upstream{
			Found: true, URL: f.URL(id), Currency: "JPY",
			ListMinor: 10000, CurrentMinor: 8000, DiscountPercent: 20,
			Converted: map[string]int64{"USD": 1157},
		}
	}
	return out, nil
}
func (f *livePriceFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type liveWorkPrices struct {
	Object string           `json:"object"`
	WorkID string           `json:"work_id"`
	Quotes []livePriceQuote `json:"quotes"`
}

type livePriceQuote struct {
	Object     string      `json:"object"`
	Source     string      `json:"source"`
	ExternalID string      `json:"external_id"`
	Region     string      `json:"region"`
	QuoteState string      `json:"quote_state"`
	Current    *liveMoney  `json:"current"`
	Converted  []liveMoney `json:"converted"`
	Stale      bool        `json:"stale"`
}

type liveMoney struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type livePricesList struct {
	Object  string           `json:"object"`
	Items   []liveWorkPrices `json:"items"`
	Missing []string         `json:"missing"`
}

func livePricesApp(t *testing.T, env *liveEnv, svc *price.Service) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	SetupWith(app, Options{
		Store:   &liveUnlimitedStore{},
		Catalog: &Catalog{Public: env.cat.Public, Resolve: env.cat.Resolve, Prices: svc},
	})
	return app
}

func livePricesSeed(t *testing.T, env *liveEnv) {
	t.Helper()
	var dlsiteID, steamID int16
	require.NoError(t, env.db.Raw(`SELECT id FROM catalog_source WHERE key = ?`, "dlsite").Scan(&dlsiteID).Error)
	require.NoError(t, env.db.Raw(`SELECT id FROM catalog_source WHERE key = ?`, "steam").Scan(&steamID).Error)
	require.NotZero(t, dlsiteID)
	require.NotZero(t, steamID)
	require.NoError(t, env.db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: env.fx.Release, SourceID: dlsiteID,
		ExternalID: "RJ149770", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error)
	require.NoError(t, env.db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeRelease, EntityID: env.fx.Release, SourceID: steamID,
		ExternalID: "1680880", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error)
}

func livePricesFixture(t *testing.T) (*liveEnv, *fiber.App, *livePriceFake, *livePriceFake) {
	t.Helper()
	env := liveCatalog(t)
	livePricesMigrate.Do(func() {
		require.NoError(t, env.db.AutoMigrate(storemodel.AllModels()...))
	})
	require.NoError(t, storetest.Truncate(env.db))
	livePricesSeedOnce.Do(func() { livePricesSeed(t, env) })
	dlsite := &livePriceFake{source: "dlsite", regions: []string{"jp"}}
	steam := &livePriceFake{source: "steam", regions: []string{"jp", "cn"}}
	svc := price.New(env.db, []price.Fetcher{dlsite, steam}, price.Options{
		FlushEvery: 50 * time.Millisecond, RefreshEvery: time.Hour,
	})
	svc.Start()
	t.Cleanup(svc.Stop)
	return env, livePricesApp(t, env, svc), dlsite, steam
}

func TestLiveStorePrices(t *testing.T) {
	env, app, dlsite, steam := livePricesFixture(t)
	path := "/v2/store/prices/" + idstr(env.fx.Work)

	resp, body := liveStoreGet(t, app, path, "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Equal(t, "private, no-store", resp.Header.Get("Cache-Control"))
	var got liveWorkPrices
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "work_prices", got.Object)
	require.Equal(t, idstr(env.fx.Work), got.WorkID)
	require.Len(t, got.Quotes, 3)
	require.Equal(t, "dlsite", got.Quotes[0].Source)
	require.Equal(t, "jp", got.Quotes[0].Region)
	require.Equal(t, "RJ149770", got.Quotes[0].ExternalID)
	require.Equal(t, "steam", got.Quotes[1].Source)
	require.Equal(t, "jp", got.Quotes[1].Region)
	require.Equal(t, "1680880", got.Quotes[1].ExternalID)
	require.Equal(t, "steam", got.Quotes[2].Source)
	require.Equal(t, "cn", got.Quotes[2].Region)
	require.Equal(t, "1680880", got.Quotes[2].ExternalID)
	for _, q := range got.Quotes {
		require.Equal(t, "price_quote", q.Object)
		require.Equal(t, "priced", q.QuoteState)
		require.False(t, q.Stale)
		require.NotNil(t, q.Current)
		require.True(t, amountRe.MatchString(q.Current.Amount), q.Current.Amount)
		require.NotNil(t, q.Converted)
	}
	firstCalls := dlsite.callCount() + steam.callCount()
	require.Equal(t, 3, firstCalls)

	resp, body = liveStoreGet(t, app, path, "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	require.Equal(t, firstCalls, dlsite.callCount()+steam.callCount())

	resp, body = liveStoreGet(t, app, "/v2/store/prices/999999999", "")
	require.Equal(t, 404, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeNotFound, liveProblem(t, body).Code)

	resp, body = liveStoreGet(t, app, "/v2/store/prices?ids="+idstr(env.fx.Work)+",999999999", "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	var list livePricesList
	require.NoError(t, json.Unmarshal(body, &list))
	require.Len(t, list.Items, 1)
	require.Equal(t, []string{"999999999"}, list.Missing)

	resp, body = liveStoreGet(t, app, "/v2/store/prices?ids=", "")
	require.Equal(t, 400, resp.StatusCode, string(body))
	require.Equal(t, problem.CodeInvalidParameter, liveProblem(t, body).Code)
}

func TestLiveStorePricesUnconfigured(t *testing.T) {
	env := liveCatalog(t)
	app := livePricesApp(t, env, nil)
	for _, path := range []string{
		"/v2/store/prices/" + idstr(env.fx.Work),
		"/v2/store/prices?ids=" + idstr(env.fx.Work),
	} {
		resp, body := liveStoreGet(t, app, path, "")
		require.Equal(t, 503, resp.StatusCode, path+" "+string(body))
		require.Equal(t, problem.CodeServiceUnavailable, liveProblem(t, body).Code, path)
	}
}
