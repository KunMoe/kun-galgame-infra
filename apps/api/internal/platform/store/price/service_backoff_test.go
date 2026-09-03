package price

import (
	"context"
	"testing"
	"time"

	"api/internal/platform/store/model"
	"api/internal/platform/store/storetest"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func (f *fakeFetcher) setFail(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = v
}

func seedExpired(t *testing.T, id string, fetched time.Time) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.PriceQuote{
		Source: "dlsite", ExternalID: id, Region: "jp",
		QuoteState: "priced", URL: "https://example.test/" + id, Currency: "JPY",
		ListMinor: 10000, CurrentMinor: 8000, Converted: datatypes.JSON([]byte("{}")),
		FetchedAt: fetched, ExpiresAt: fetched, LastRequestedAt: time.Now(),
	}).Error)
}

func takeQuote(t *testing.T, id string) model.PriceQuote {
	t.Helper()
	var row model.PriceQuote
	require.NoError(t, testDB.Where("source = ? AND external_id = ? AND region = ?", "dlsite", id, "jp").Take(&row).Error)
	return row
}

func TestFailedRefreshSchedulesTheRetryInsteadOfEveryTick(t *testing.T) {
	fake := &fakeFetcher{fail: true}
	svc := newTestService(t, []Fetcher{fake}, Options{ErrorFor: 15 * time.Minute, RefreshEvery: 50 * time.Millisecond})
	past := time.Now().Add(-time.Hour)
	seedExpired(t, "RJ300001", past)

	_, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ300001"}}, 0)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return fake.callCount() >= 1 }, 3*time.Second, 20*time.Millisecond)

	require.Never(t, func() bool { return fake.callCount() > 1 }, 600*time.Millisecond, 25*time.Millisecond)

	row := takeQuote(t, "RJ300001")
	require.Equal(t, "priced", row.QuoteState)
	require.WithinDuration(t, past, row.FetchedAt, 2*time.Second)
	require.NotNil(t, row.NextAttemptAt)
	require.InDelta(t, (15 * time.Minute).Seconds(), time.Until(*row.NextAttemptAt).Seconds(), 5)

	_, err = svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ300001"}}, 0)
	require.NoError(t, err)
	require.Never(t, func() bool { return fake.callCount() > 1 }, 300*time.Millisecond, 25*time.Millisecond)
}

func TestScheduledRetryRunsWhenDueAndSuccessClearsIt(t *testing.T) {
	fake := &fakeFetcher{fail: true}
	svc := newTestService(t, []Fetcher{fake}, Options{ErrorFor: 15 * time.Minute, RefreshEvery: 50 * time.Millisecond})
	past := time.Now().Add(-time.Hour)
	seedExpired(t, "RJ300002", past)

	_, err := svc.Quotes(context.Background(), []Anchor{{Source: "dlsite", ExternalID: "RJ300002"}}, 0)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return fake.callCount() >= 1 }, 3*time.Second, 20*time.Millisecond)

	fake.setFail(false)
	require.NoError(t, testDB.Model(&model.PriceQuote{}).
		Where("external_id = ?", "RJ300002").
		Update("next_attempt_at", time.Now().Add(-time.Minute)).Error)

	require.Eventually(t, func() bool {
		row := takeQuote(t, "RJ300002")
		return row.NextAttemptAt == nil && row.FetchedAt.After(past.Add(time.Minute))
	}, 3*time.Second, 25*time.Millisecond)
}

func TestDueForRefreshSkipsScheduledRows(t *testing.T) {
	require.NoError(t, storetest.Truncate(testDB))
	svc := New(testDB, nil, Options{})
	past := time.Now().Add(-time.Hour)
	seedExpired(t, "RJ300003", past)
	seedExpired(t, "RJ300004", past)
	later := time.Now().Add(10 * time.Minute)
	require.NoError(t, testDB.Model(&model.PriceQuote{}).
		Where("external_id = ?", "RJ300004").
		Update("next_attempt_at", later).Error)

	keys, err := svc.dueForRefresh(context.Background(), time.Now(), 7*24*time.Hour, 500)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "RJ300003", keys[0].ExternalID)
}
