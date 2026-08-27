package storestats

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"api/internal/platform/store/model"
	"api/internal/platform/store/shortener"
	"api/internal/platform/store/storetest"

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

func fresh(t *testing.T) *storetest.FakeShortener {
	t.Helper()
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	return fake
}

func seedPurchase(t *testing.T, clientID, productID, alias string, created time.Time) {
	t.Helper()
	row := model.PurchaseLink{
		ClientID: clientID, ProductID: productID, Alias: alias,
		ShortURL: "https://s.test/" + alias, CreatedAt: created,
	}
	if err := testDB.Create(&row).Error; err != nil {
		t.Fatalf("seed purchase link: %v", err)
	}
}

func seedCoupon(t *testing.T, clientID string, campaignID int64, alias string, created time.Time) {
	t.Helper()
	row := model.CouponLink{
		ClientID: clientID, CampaignID: campaignID, Alias: alias,
		ShortURL: "https://s.test/" + alias, CreatedAt: created,
	}
	if err := testDB.Create(&row).Error; err != nil {
		t.Fatalf("seed coupon link: %v", err)
	}
}

func statFor(t *testing.T, alias, day string) model.LinkDailyStat {
	t.Helper()
	var row model.LinkDailyStat
	if err := testDB.Where("alias = ? AND day = ?", alias, day).Take(&row).Error; err != nil {
		t.Fatalf("read %s/%s: %v", alias, day, err)
	}
	return row
}

func TestEmptyLinkTablesMakeNoUpstreamCall(t *testing.T) {
	fake := fresh(t)

	res, err := Run(context.Background(), testDB, fake.Client("slk_test"), DefaultOpts())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Aliases != 0 || res.Calls != 0 {
		t.Errorf("result = %+v, want no aliases and no calls", res)
	}
	if got := len(fake.Stats()); got != 0 {
		t.Errorf("upstream calls = %d, want 0", got)
	}
}

func TestFirstRunPullsFromTheEarliestMintNotTheThreeDayWindow(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	old := now.AddDate(0, 0, -40)
	seedPurchase(t, "site-a", "RJ100001", "a1", old)
	seedCoupon(t, "site-a", 7, "c1", now.AddDate(0, 0, -5))

	oldDay := model.JSTDay(old)
	fake.Series["a1"] = []shortener.DayStat{{Date: oldDay, Total: 9, Uniques: 4}}

	res, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !res.FullPull {
		t.Error("FullPull = false; the first run must not start three days ago and lose earlier clicks")
	}
	if res.From != oldDay {
		t.Errorf("from = %q, want %q (earliest created_at)", res.From, oldDay)
	}
	if res.Aliases != 2 {
		t.Errorf("aliases = %d, want 2 (purchase + coupon)", res.Aliases)
	}
	got := statFor(t, "a1", oldDay)
	if got.Total != 9 || got.Uniques != 4 {
		t.Errorf("row = %+v, want total=9 uniques=4", got)
	}
	if got.SyncedAt.IsZero() {
		t.Error("synced_at is zero")
	}
}

func TestRoutineRunUsesTheThreeDayWindow(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	seedPurchase(t, "site-a", "RJ100001", "a1", now.AddDate(0, 0, -40))
	if err := testDB.Create(&model.LinkDailyStat{
		Alias: "a1", Day: model.JSTDay(now.AddDate(0, 0, -30)), Total: 1, Uniques: 1, SyncedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed cached row: %v", err)
	}

	res, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.FullPull {
		t.Error("FullPull = true on a run that already has cached rows")
	}
	if want := model.JSTDay(now.AddDate(0, 0, -2)); res.From != want {
		t.Errorf("from = %q, want %q", res.From, want)
	}
	calls := fake.Stats()
	if len(calls) != 1 {
		t.Fatalf("upstream calls = %d, want 1", len(calls))
	}
	if calls[0].From != res.From || calls[0].To != res.To {
		t.Errorf("call range = %s..%s, want %s..%s", calls[0].From, calls[0].To, res.From, res.To)
	}
	if want := "Bearer slk_test"; calls[0].Auth != want {
		t.Errorf("authorization = %q, want %q", calls[0].Auth, want)
	}
}

func TestReSyncOverwritesRatherThanAccumulates(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	today := model.JSTDay(now)
	seedPurchase(t, "site-a", "RJ100001", "a1", now)
	fake.Series["a1"] = []shortener.DayStat{{Date: today, Total: 10, Uniques: 6}}

	for range 3 {
		if _, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now}); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	got := statFor(t, "a1", today)
	if got.Total != 10 || got.Uniques != 6 {
		t.Errorf("row = %+v after three runs, want total=10 uniques=6 — upstream is authoritative, not additive", got)
	}

	fake.Series["a1"] = []shortener.DayStat{{Date: today, Total: 14, Uniques: 8}}
	if _, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got = statFor(t, "a1", today)
	if got.Total != 14 || got.Uniques != 8 {
		t.Errorf("row = %+v, want the revised upstream numbers", got)
	}

	var rows int64
	if err := testDB.Model(&model.LinkDailyStat{}).Count(&rows).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Errorf("store_link_daily_stats rows = %d, want 1", rows)
	}
}

func TestAliasesArePagedWithinTheContractLimit(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	for i := range 1201 {
		seedPurchase(t, "site-a", fmt.Sprintf("RJ%06d", i+1), fmt.Sprintf("a%d", i+1), now)
	}

	res, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Aliases != 1201 {
		t.Fatalf("aliases = %d, want 1201", res.Aliases)
	}
	calls := fake.Stats()
	if len(calls) != 3 {
		t.Fatalf("upstream calls = %d, want 3 (500+500+201)", len(calls))
	}
	seen := 0
	for _, c := range calls {
		if len(c.Aliases) > shortener.MaxAliasesPerStatsCall {
			t.Errorf("a call carried %d aliases, over the %d limit", len(c.Aliases), shortener.MaxAliasesPerStatsCall)
		}
		seen += len(c.Aliases)
	}
	if seen != 1201 {
		t.Errorf("aliases across calls = %d, want 1201", seen)
	}
}

func TestLongFirstPullIsSplitIntoContractSizedWindows(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	seedPurchase(t, "site-a", "RJ100001", "a1", now.AddDate(0, 0, -200))

	if _, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now}); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := fake.Stats()
	if len(calls) != 3 {
		t.Fatalf("upstream calls = %d, want 3 windows over 201 days", len(calls))
	}
	for _, c := range calls {
		from, ok := model.ParseJSTDay(c.From)
		if !ok {
			t.Fatalf("bad from %q", c.From)
		}
		to, ok := model.ParseJSTDay(c.To)
		if !ok {
			t.Fatalf("bad to %q", c.To)
		}
		if span := model.DaySpan(from, to); span > shortener.MaxStatsSpanDays {
			t.Errorf("window %s..%s spans %d days, over the %d limit", c.From, c.To, span, shortener.MaxStatsSpanDays)
		}
	}
	if calls[0].From != model.JSTDay(now.AddDate(0, 0, -200)) {
		t.Errorf("first window starts %q, want the mint day", calls[0].From)
	}
	if last := calls[len(calls)-1].To; last != model.JSTDay(now) {
		t.Errorf("last window ends %q, want today", last)
	}
}

func TestUnknownAliasYieldsNoRowsRatherThanAnError(t *testing.T) {
	fake := fresh(t)
	now := time.Now()
	seedPurchase(t, "site-a", "RJ100001", "never-clicked", now)

	res, err := Run(context.Background(), testDB, fake.Client("slk_test"), Opts{Now: now})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Rows != 0 {
		t.Errorf("rows = %d, want 0", res.Rows)
	}
}
