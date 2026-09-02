package price

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"api/internal/platform/store/model"
	"api/internal/platform/store/storetest"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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

type fakeCall struct {
	Region string
	IDs    []string
}

type fakeFetcher struct {
	mu      sync.Mutex
	source  string
	regions []string
	batch   int
	gap     time.Duration
	answers map[string]Upstream
	fail    bool
	calls   []fakeCall
}

func (f *fakeFetcher) Source() string {
	if f.source == "" {
		return "dlsite"
	}
	return f.source
}
func (f *fakeFetcher) Regions() []string {
	if len(f.regions) == 0 {
		return []string{"jp"}
	}
	return append([]string{}, f.regions...)
}
func (f *fakeFetcher) Batch() int {
	if f.batch <= 0 {
		return 100
	}
	return f.batch
}
func (f *fakeFetcher) Gap() time.Duration   { return f.gap }
func (f *fakeFetcher) Accepts(string) bool  { return true }
func (f *fakeFetcher) URL(id string) string { return "https://example.test/" + id }
func (f *fakeFetcher) Fetch(_ context.Context, region string, ids []string) (map[string]Upstream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := append([]string{}, ids...)
	f.calls = append(f.calls, fakeCall{Region: region, IDs: copied})
	if f.fail {
		return nil, errors.New("upstream down")
	}
	out := map[string]Upstream{}
	for _, id := range ids {
		if f.answers != nil {
			if up, ok := f.answers[id]; ok {
				out[id] = up
			}
			continue
		}
		out[id] = Upstream{
			Found: true, URL: f.URL(id), Currency: "JPY",
			ListMinor: 10000, CurrentMinor: 8000, DiscountPercent: 20,
			Converted: map[string]int64{},
		}
	}
	return out, nil
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFetcher) snapshot() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func testOpts(o Options) Options {
	if o.FlushEvery == 0 {
		o.FlushEvery = 50 * time.Millisecond
	}
	if o.RefreshEvery == 0 {
		o.RefreshEvery = 100 * time.Millisecond
	}
	return o
}

func newTestService(t *testing.T, fetchers []Fetcher, opts Options) *Service {
	t.Helper()
	require.NoError(t, storetest.Truncate(testDB))
	svc := New(testDB, fetchers, testOpts(opts))
	svc.Start()
	t.Cleanup(svc.Stop)
	return svc
}

func TestQuotesColdMissWaitsAndServes(t *testing.T) {
	fake := &fakeFetcher{}
	svc := newTestService(t, []Fetcher{fake}, Options{})
	anchors := []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}
	quotes, err := svc.Quotes(context.Background(), anchors, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.Equal(t, "priced", quotes[0].State)
	require.Equal(t, 1, fake.callCount())
	quotes, err = svc.Quotes(context.Background(), anchors, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.Equal(t, "priced", quotes[0].State)
	require.Equal(t, 1, fake.callCount())
}

func TestQuotesCoalescing(t *testing.T) {
	fake := &fakeFetcher{}
	svc := newTestService(t, []Fetcher{fake}, Options{})
	anchors := []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}
	var ready, done sync.WaitGroup
	ready.Add(2)
	done.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			ready.Wait()
			_, err := svc.Quotes(context.Background(), anchors, 2*time.Second)
			require.NoError(t, err)
		}()
	}
	done.Wait()
	require.Equal(t, 1, fake.callCount())
}

func TestQuotesBatching(t *testing.T) {
	fake := &fakeFetcher{batch: 3}
	svc := newTestService(t, []Fetcher{fake}, Options{})
	var anchors []Anchor
	for _, id := range []string{"RJ000001", "RJ000002", "RJ000003", "RJ000004", "RJ000005"} {
		anchors = append(anchors, Anchor{Source: "dlsite", ExternalID: id})
	}
	quotes, err := svc.Quotes(context.Background(), anchors, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 5)
	calls := fake.snapshot()
	require.Len(t, calls, 2)
	seen := map[string]struct{}{}
	for _, c := range calls {
		require.LessOrEqual(t, len(c.IDs), 3)
		for _, id := range c.IDs {
			seen[id] = struct{}{}
		}
	}
	require.Len(t, seen, 5)
}

func TestQuotesStaleServesAndRefreshes(t *testing.T) {
	fake := &fakeFetcher{}
	svc := newTestService(t, []Fetcher{fake}, Options{})
	past := time.Now().Add(-time.Hour)
	require.NoError(t, testDB.Create(&model.PriceQuote{
		Source: "dlsite", ExternalID: "RJ149770", Region: "jp",
		QuoteState: "priced", URL: "https://example.test/RJ149770", Currency: "JPY",
		ListMinor: 10000, CurrentMinor: 8000, Converted: datatypes.JSON([]byte("{}")),
		FetchedAt: past, ExpiresAt: past, LastRequestedAt: past,
	}).Error)
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}, 0)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.True(t, quotes[0].Stale)
	require.Equal(t, "priced", quotes[0].State)
	require.Eventually(t, func() bool {
		if fake.callCount() < 1 {
			return false
		}
		var row model.PriceQuote
		if err := testDB.Where("source = ? AND external_id = ? AND region = ?", "dlsite", "RJ149770", "jp").Take(&row).Error; err != nil {
			return false
		}
		return row.FetchedAt.After(past.Add(time.Minute))
	}, 3*time.Second, 20*time.Millisecond)
}

func TestQuotesUpstreamError(t *testing.T) {
	fake := &fakeFetcher{fail: true}
	svc := newTestService(t, []Fetcher{fake}, Options{ErrorFor: 15 * time.Minute})
	quotes, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ149770"}}, 2*time.Second)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.Equal(t, "unavailable", quotes[0].State)
	var row model.PriceQuote
	require.NoError(t, testDB.Where("source = ? AND external_id = ?", "dlsite", "RJ149770").Take(&row).Error)
	require.InDelta(t, (15 * time.Minute).Seconds(), row.ExpiresAt.Sub(row.FetchedAt).Seconds(), 2)

	require.NoError(t, storetest.Truncate(testDB))
	past := time.Now().Add(-time.Hour)
	require.NoError(t, testDB.Create(&model.PriceQuote{
		Source: "dlsite", ExternalID: "RJ200000", Region: "jp",
		QuoteState: "priced", URL: "https://example.test/RJ200000", Currency: "JPY",
		ListMinor: 1, CurrentMinor: 1, Converted: datatypes.JSON([]byte("{}")),
		FetchedAt: past, ExpiresAt: past, LastRequestedAt: past,
	}).Error)
	quotes, err = svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ200000"}}, 0)
	require.NoError(t, err)
	require.Len(t, quotes, 1)
	require.True(t, quotes[0].Stale)
	require.Eventually(t, func() bool { return fake.callCount() >= 2 }, 3*time.Second, 20*time.Millisecond)
	var kept model.PriceQuote
	require.NoError(t, testDB.Where("source = ? AND external_id = ?", "dlsite", "RJ200000").Take(&kept).Error)
	require.WithinDuration(t, past, kept.FetchedAt, 2*time.Second)
}
