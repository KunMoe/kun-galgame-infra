package price

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDLsiteFetch(t *testing.T) {
	const sample = `{
		"RJ01402486": {
			"price": 275, "official_price": 550, "discount_rate": 50, "is_discount": true,
			"campaign_start_date": null, "campaign_end_date": "2026-09-01 12:00:00",
			"timesale_start": null, "timesale_end": null,
			"on_sale": 1, "sales_end_date": null,
			"currency_price": {"JPY": 275, "CNY": 11.566188036776273, "USD": 1.7175391349098885, "TWD": 0, "HKD": 0, "KRW": 0, "EUR": 0},
			"locale_price": {"zh_CN": 0}
		}
	}`
	var gotPath, gotQuery, gotCookie, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("product_id")
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sample))
	}))
	t.Cleanup(srv.Close)

	f := NewDLsite("Test-UA/1.0", []string{"CNY", "USD", "TWD", "HKD", "KRW", "EUR"}, srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"RJ00000001", "RJ01402486"})
	require.NoError(t, err)
	require.Equal(t, "/maniax/product/info/ajax", gotPath)
	require.Equal(t, "RJ00000001,RJ01402486", gotQuery)
	require.Equal(t, "adultchecked=1", gotCookie)
	require.Equal(t, "Test-UA/1.0", gotUA)
	_, omitted := out["RJ00000001"]
	require.False(t, omitted)
	up, ok := out["RJ01402486"]
	require.True(t, ok)
	require.True(t, up.Found)
	require.Equal(t, int64(55000), up.ListMinor)
	require.Equal(t, int64(27500), up.CurrentMinor)
	require.Equal(t, 50, up.DiscountPercent)
	require.Equal(t, "JPY", up.Currency)
	require.Equal(t, int64(1157), up.Converted["CNY"])
	_, hasTWD := up.Converted["TWD"]
	require.False(t, hasTWD)
	require.NotNil(t, up.SaleEndsAt)
	want := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	require.True(t, up.SaleEndsAt.UTC().Equal(want), up.SaleEndsAt.UTC())
}

func TestDLsiteOnSaleZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"RJ149770": map[string]any{"price": 275, "official_price": 550, "discount_rate": 0, "on_sale": 0, "currency_price": map[string]any{}},
		})
	}))
	t.Cleanup(srv.Close)
	f := NewDLsite("Test-UA/1.0", nil, srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"RJ149770"})
	require.NoError(t, err)
	up, ok := out["RJ149770"]
	require.True(t, ok)
	require.False(t, up.Found)
}

func TestDLsiteAccepts(t *testing.T) {
	f := NewDLsite("", nil, "")
	require.False(t, f.Accepts("RE149770"))
	require.True(t, f.Accepts("RJ149770"))
	require.True(t, f.Accepts("VJ012345"))
}

func TestDLsiteTimesaleTLayout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"RJ149770": map[string]any{
				"price": 100, "official_price": 100, "discount_rate": 0, "on_sale": 1,
				"campaign_end_date": nil,
				"timesale_end":      "2026-09-01T21:00:00",
				"currency_price":    map[string]any{},
			},
		})
	}))
	t.Cleanup(srv.Close)
	f := NewDLsite("Test-UA/1.0", nil, srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"RJ149770"})
	require.NoError(t, err)
	up := out["RJ149770"]
	require.True(t, up.Found)
	require.NotNil(t, up.SaleEndsAt)
	want := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.True(t, up.SaleEndsAt.UTC().Equal(want), up.SaleEndsAt.UTC())
}
