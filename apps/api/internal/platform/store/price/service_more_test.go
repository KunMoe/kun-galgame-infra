package price

import (
	"context"
	"testing"
	"time"

	"api/internal/platform/store/model"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestQuotesNotFoundUpstream(t *testing.T) {
	fake := &fakeFetcher{answers: map[string]Upstream{}}
	svc := newTestService(t, []Fetcher{fake}, Options{NegativeFor: 24 * time.Hour})
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.Equal(t, "unavailable", quotes[0].State)
	var row model.PriceQuote
	require.NoError(t, testDB.Where("source = ? AND external_id = ?", "dlsite", "RJ149770").Take(&row).Error)
	require.InDelta(t, (24 * time.Hour).Seconds(), row.ExpiresAt.Sub(row.FetchedAt).Seconds(), 2)
}

func TestQuotesSaleEndCapsExpiry(t *testing.T) {
	saleEnd := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	fake := &fakeFetcher{answers: map[string]Upstream{
		"RJ149770": {
			Found: true, URL: "https://example.test/RJ149770", Currency: "JPY",
			ListMinor: 10000, CurrentMinor: 8000, SaleEndsAt: &saleEnd,
			Converted: map[string]int64{},
		},
	}}
	svc := newTestService(t, []Fetcher{fake}, Options{FreshFor: 6 * time.Hour})
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.Equal(t, "priced", quotes[0].State)
	require.NotNil(t, quotes[0].ExpiresAt)
	require.WithinDuration(t, saleEnd, *quotes[0].ExpiresAt, 2*time.Second)
}

func TestRefreshPicksHotRowsOnly(t *testing.T) {
	fake := &fakeFetcher{}
	svc := newTestService(t, []Fetcher{fake}, Options{HotWindow: 7 * 24 * time.Hour, FreshFor: 6 * time.Hour})
	require.NotNil(t, svc)
	now := time.Now()
	expired := now.Add(-time.Hour)
	require.NoError(t, testDB.Create(&model.PriceQuote{
		Source: "dlsite", ExternalID: "RJCOLD000", Region: "jp",
		QuoteState: "priced", URL: "https://example.test/RJCOLD000", Currency: "JPY",
		Converted: datatypes.JSON([]byte("{}")),
		FetchedAt: expired, ExpiresAt: expired, LastRequestedAt: now.Add(-8 * 24 * time.Hour),
	}).Error)
	require.NoError(t, testDB.Create(&model.PriceQuote{
		Source: "dlsite", ExternalID: "RJHOT0000", Region: "jp",
		QuoteState: "priced", URL: "https://example.test/RJHOT0000", Currency: "JPY",
		Converted: datatypes.JSON([]byte("{}")),
		FetchedAt: expired, ExpiresAt: expired, LastRequestedAt: now.Add(-time.Hour),
	}).Error)
	require.Eventually(t, func() bool {
		for _, c := range fake.snapshot() {
			for _, id := range c.IDs {
				if id == "RJHOT0000" {
					return true
				}
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)
	for _, c := range fake.snapshot() {
		for _, id := range c.IDs {
			require.NotEqual(t, "RJCOLD000", id)
		}
	}
}

func TestQuotesRegions(t *testing.T) {
	fake := &fakeFetcher{source: "steam", regions: []string{"jp", "cn"}}
	svc := newTestService(t, []Fetcher{fake}, Options{})
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "steam", ExternalID: "1680880"}}, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 2)
	require.Equal(t, "jp", quotes[0].Key.Region)
	require.Equal(t, "cn", quotes[1].Key.Region)
}

func TestQuotesUnknownSourceDropped(t *testing.T) {
	svc := newTestService(t, nil, Options{})
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "dmm", ExternalID: "123"}}, 0)
	require.NoError(t, err)
	require.Empty(t, quotes)
}
