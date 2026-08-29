package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"api/internal/platform/store/model"
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

const (
	tmplManiax = "https://www.dlsite.com/maniax/dlaf/=/link/work/aid/test/id/{product_id}.html"
	tmplPro    = "https://www.dlsite.com/pro/dlaf/=/link/work/aid/test/id/{product_id}.html"
)

func newFixture(t *testing.T, quota int) (*Service, *storetest.FakeShortener) {
	t.Helper()
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := New(testDB, fake.Client("slk_test"), Options{
		AffTemplateManiax:  tmplManiax,
		AffTemplatePro:     tmplPro,
		LinkQuotaPerClient: quota,
	})
	return svc, fake
}

func seedCampaign(t *testing.T, name, url string, starts, ends time.Time) int64 {
	t.Helper()
	row := model.Campaign{Name: name, CouponURL: url, StartsAt: starts, EndsAt: ends}
	if err := testDB.Create(&row).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	return row.ID
}

func TestMintSendsReuseFalse(t *testing.T) {
	svc, fake := newFixture(t, 5000)

	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "RJ123456"); err != nil {
		t.Fatalf("purchase links: %v", err)
	}

	mints := fake.Mints()
	if len(mints) != 1 {
		t.Fatalf("mint calls = %d, want 1", len(mints))
	}
	if mints[0].Reuse == nil {
		t.Fatal("mint body omitted `reuse`; the shortener would default to reusing the destination and collapse every site onto one alias")
	}
	if *mints[0].Reuse {
		t.Fatal("mint body sent reuse=true; per-site attribution would be destroyed")
	}
	if want := "https://www.dlsite.com/maniax/dlaf/=/link/work/aid/test/id/RJ123456.html"; mints[0].DestinationURL != want {
		t.Errorf("destination = %q, want %q", mints[0].DestinationURL, want)
	}
	if want := "store:site-a:RJ123456:purchase"; mints[0].Description != want {
		t.Errorf("description = %q, want %q", mints[0].Description, want)
	}
	if want := "Bearer slk_test"; mints[0].Auth != want {
		t.Errorf("authorization = %q, want %q", mints[0].Auth, want)
	}
}

func TestTwoSitesGetDistinctLinksForOneProduct(t *testing.T) {
	svc, fake := newFixture(t, 5000)
	ctx := context.Background()

	a, err := svc.PurchaseLinks(ctx, "site-a", "RJ123456")
	if err != nil {
		t.Fatalf("site-a: %v", err)
	}
	b, err := svc.PurchaseLinks(ctx, "site-b", "RJ123456")
	if err != nil {
		t.Fatalf("site-b: %v", err)
	}
	if a.PurchaseURL == b.PurchaseURL {
		t.Fatalf("both sites got %q — attribution collapsed", a.PurchaseURL)
	}
	if got := len(fake.Mints()); got != 2 {
		t.Errorf("mint calls = %d, want 2", got)
	}
}

// A shortener that stopped honouring reuse:false hands every client the same
// alias. Without the alias unique index both rows accept it and the two sites'
// clicks merge into one bucket — a settlement that is wrong with nothing to
// show for it. The constraint turns that into a refused mint someone notices.
func TestACollapsedAliasIsRefusedNotQuietlyShared(t *testing.T) {
	svc, _ := newFixture(t, 5000)
	ctx := context.Background()

	if _, err := svc.PurchaseLinks(ctx, "site-a", "RJ100001"); err != nil {
		t.Fatalf("site-a: %v", err)
	}

	var first model.PurchaseLink
	if err := testDB.Where("client_id = ?", "site-a").Take(&first).Error; err != nil {
		t.Fatalf("read site-a row: %v", err)
	}

	fakeCollapsed := storetest.NewFakeShortener()
	t.Cleanup(fakeCollapsed.Close)
	fakeCollapsed.StickyAlias = first.Alias
	collapsed := New(testDB, fakeCollapsed.Client("slk_test"), Options{
		AffTemplateManiax: tmplManiax, AffTemplatePro: tmplPro, LinkQuotaPerClient: 5000,
	})

	if _, err := collapsed.PurchaseLinks(ctx, "site-b", "RJ100001"); err == nil {
		t.Fatal("site-b got a link carrying site-a's alias; attribution collapsed silently")
	}
	var rows int64
	if err := testDB.Model(&model.PurchaseLink{}).Where("alias = ?", first.Alias).Count(&rows).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Errorf("rows sharing alias %q = %d, want 1", first.Alias, rows)
	}

	// The same guard on the coupon table.
	now := time.Now()
	campaignID := seedCampaign(t, "券", "https://dlsite.test/coupon", now.Add(-time.Hour), now.Add(time.Hour))
	if _, err := svc.PurchaseLinks(ctx, "site-c", "RJ100002"); err != nil {
		t.Fatalf("site-c: %v", err)
	}
	var couponRow model.CouponLink
	if err := testDB.Where("client_id = ?", "site-c").Take(&couponRow).Error; err != nil {
		t.Fatalf("read site-c coupon row: %v", err)
	}
	if couponRow.CampaignID != campaignID {
		t.Fatalf("coupon row campaign = %d, want %d", couponRow.CampaignID, campaignID)
	}

	fakeCollapsed.StickyAlias = couponRow.Alias
	if _, err := collapsed.PurchaseLinks(ctx, "site-d", "RJ100003"); err == nil {
		t.Fatal("site-d got a coupon link carrying site-c's alias")
	}
	// The two indexes are per table, so site-d's PURCHASE row was allowed to
	// take the same alias string a coupon row holds; only the coupon leg
	// collided. A shared index would have stopped this insert instead, and the
	// assertion above would pass for the wrong reason.
	var purchaseDupes int64
	if err := testDB.Model(&model.PurchaseLink{}).
		Where("client_id = ? AND alias = ?", "site-d", couponRow.Alias).
		Count(&purchaseDupes).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if purchaseDupes != 1 {
		t.Errorf("site-d purchase rows holding %q = %d, want 1", couponRow.Alias, purchaseDupes)
	}
}

func TestVJUsesProTemplate(t *testing.T) {
	svc, fake := newFixture(t, 5000)

	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "VJ01000123"); err != nil {
		t.Fatalf("purchase links: %v", err)
	}
	want := "https://www.dlsite.com/pro/dlaf/=/link/work/aid/test/id/VJ01000123.html"
	if got := fake.Mints()[0].DestinationURL; got != want {
		t.Errorf("destination = %q, want %q", got, want)
	}
}

func TestInvalidProductIDIsRejectedBeforeMinting(t *testing.T) {
	svc, fake := newFixture(t, 5000)

	for _, bad := range []string{"", "RJ12345", "RJ123456789", "RE123456", "BJ123456", "rj123456", "RJ12345a"} {
		if _, err := svc.PurchaseLinks(context.Background(), "site-a", bad); !errors.Is(err, ErrInvalidProductID) {
			t.Errorf("product %q: err = %v, want ErrInvalidProductID", bad, err)
		}
	}
	if got := len(fake.Mints()); got != 0 {
		t.Errorf("mint calls = %d, want 0 — a malformed id must never reach the shortener", got)
	}
}

func TestSecondRequestReusesTheStoredAlias(t *testing.T) {
	svc, fake := newFixture(t, 5000)
	ctx := context.Background()

	first, err := svc.PurchaseLinks(ctx, "site-a", "RJ123456")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.PurchaseLinks(ctx, "site-a", "RJ123456")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.PurchaseURL != second.PurchaseURL {
		t.Errorf("url moved: %q → %q", first.PurchaseURL, second.PurchaseURL)
	}
	if got := len(fake.Mints()); got != 1 {
		t.Errorf("mint calls = %d, want 1", got)
	}
}

func TestConcurrentFirstRequestsSettleOnOneRow(t *testing.T) {
	svc, _ := newFixture(t, 5000)
	ctx := context.Background()

	const n = 8
	urls := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := svc.PurchaseLinks(ctx, "site-a", "RJ123456")
			errs[i] = err
			if err == nil {
				urls[i] = out.PurchaseURL
			}
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if urls[i] != urls[0] {
			t.Fatalf("goroutine %d returned %q, goroutine 0 returned %q", i, urls[i], urls[0])
		}
	}
	var rows int64
	if err := testDB.Model(&model.PurchaseLink{}).Count(&rows).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Errorf("store_purchase_links rows = %d, want 1", rows)
	}
}

func TestQuotaRefusesTheNextNewProduct(t *testing.T) {
	svc, fake := newFixture(t, 2)
	ctx := context.Background()

	for _, id := range []string{"RJ100001", "RJ100002"} {
		if _, err := svc.PurchaseLinks(ctx, "site-a", id); err != nil {
			t.Fatalf("product %s: %v", id, err)
		}
	}
	if _, err := svc.PurchaseLinks(ctx, "site-a", "RJ100003"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	if got := len(fake.Mints()); got != 2 {
		t.Errorf("mint calls = %d, want 2 — the refused product must not reach the shortener", got)
	}
	// The cap is on NEW products; an already-minted one keeps working.
	if _, err := svc.PurchaseLinks(ctx, "site-a", "RJ100001"); err != nil {
		t.Errorf("existing product after quota: %v", err)
	}
	// And it is per client, not global.
	if _, err := svc.PurchaseLinks(ctx, "site-b", "RJ100003"); err != nil {
		t.Errorf("other client after quota: %v", err)
	}
}

func TestShortenerDownSurfacesAsAnErrorNotABareAffURL(t *testing.T) {
	svc, fake := newFixture(t, 5000)
	fake.MintFails = true

	out, err := svc.PurchaseLinks(context.Background(), "site-a", "RJ123456")
	if !errors.Is(err, ErrShortenerDown) {
		t.Fatalf("err = %v, want ErrShortenerDown", err)
	}
	if out != nil {
		t.Fatalf("got a payload %+v — a bare aff URL bypasses the click counter", out)
	}
	var rows int64
	if err := testDB.Model(&model.PurchaseLink{}).Count(&rows).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Errorf("store_purchase_links rows = %d, want 0", rows)
	}
}

func TestNoActiveCampaignLeavesTheCouponSlotEmpty(t *testing.T) {
	svc, fake := newFixture(t, 5000)
	now := time.Now()
	seedCampaign(t, "already over", "https://dlsite.test/coupon/past", now.Add(-72*time.Hour), now.Add(-48*time.Hour))
	seedCampaign(t, "not yet", "https://dlsite.test/coupon/future", now.Add(48*time.Hour), now.Add(72*time.Hour))

	out, err := svc.PurchaseLinks(context.Background(), "site-a", "RJ123456")
	if err != nil {
		t.Fatalf("purchase links: %v", err)
	}
	if out.CouponURL != nil {
		t.Errorf("coupon_url = %q, want null", *out.CouponURL)
	}
	if out.Campaign != nil {
		t.Errorf("campaign = %+v, want null", out.Campaign)
	}
	if got := len(fake.Mints()); got != 1 {
		t.Errorf("mint calls = %d, want 1 (purchase only)", got)
	}
}

func TestActiveCampaignMintsAPerClientCouponLink(t *testing.T) {
	svc, fake := newFixture(t, 5000)
	now := time.Now()
	id := seedCampaign(t, "9 月券", "https://dlsite.test/coupon/september", now.Add(-time.Hour), now.Add(time.Hour))
	ctx := context.Background()

	a, err := svc.PurchaseLinks(ctx, "site-a", "RJ123456")
	if err != nil {
		t.Fatalf("site-a: %v", err)
	}
	if a.CouponURL == nil || a.Campaign == nil {
		t.Fatalf("coupon slot empty: %+v", a)
	}
	if a.Campaign.ID != id || a.Campaign.Name != "9 月券" {
		t.Errorf("campaign = %+v, want id=%d name=9 月券", a.Campaign, id)
	}
	if *a.CouponURL == a.PurchaseURL {
		t.Error("coupon and purchase share one alias")
	}

	b, err := svc.PurchaseLinks(ctx, "site-b", "RJ123456")
	if err != nil {
		t.Fatalf("site-b: %v", err)
	}
	if *a.CouponURL == *b.CouponURL {
		t.Error("both sites got the same coupon alias — coupon interest is per site too")
	}

	// A second call for a different product reuses the campaign's coupon link.
	again, err := svc.PurchaseLinks(ctx, "site-a", "RJ999999")
	if err != nil {
		t.Fatalf("second product: %v", err)
	}
	if *again.CouponURL != *a.CouponURL {
		t.Errorf("coupon url moved: %q → %q", *a.CouponURL, *again.CouponURL)
	}

	var couponDescriptions int
	for _, m := range fake.Mints() {
		if m.Description == "store:site-a:"+strconv.FormatInt(id, 10)+":coupon" {
			couponDescriptions++
		}
		if m.Reuse == nil || *m.Reuse {
			t.Fatalf("coupon mint sent reuse=%v", m.Reuse)
		}
	}
	if couponDescriptions != 1 {
		t.Errorf("site-a coupon mints = %d, want 1", couponDescriptions)
	}
}

func TestUnconfiguredShortenerRefusesRatherThanLeaking(t *testing.T) {
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	svc := New(testDB, nil, Options{AffTemplateManiax: tmplManiax, AffTemplatePro: tmplPro})
	if svc.Configured() {
		t.Fatal("Configured() = true with no minter")
	}
	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "RJ123456"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestMissingAffTemplateRefusesRatherThanMintingAnEmptyDestination(t *testing.T) {
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := New(testDB, fake.Client("slk_test"), Options{AffTemplateManiax: tmplManiax})

	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "VJ01000123"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if got := len(fake.Mints()); got != 0 {
		t.Errorf("mint calls = %d, want 0", got)
	}
}

func TestAffTemplateWithoutTheProductIDSlotRefusesRatherThanMintingOneDeadURL(t *testing.T) {
	if err := storetest.Truncate(testDB); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	fake := storetest.NewFakeShortener()
	t.Cleanup(fake.Close)
	svc := New(testDB, fake.Client("slk_test"), Options{
		AffTemplateManiax: "https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/id/{workno}.html",
		AffTemplatePro:    tmplPro,
	})

	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "RJ123456"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if got := len(fake.Mints()); got != 0 {
		t.Errorf("mint calls = %d, want 0", got)
	}
	if _, err := svc.PurchaseLinks(context.Background(), "site-a", "VJ01000123"); err != nil {
		t.Fatalf("the well-formed pro template must still mint: %v", err)
	}
}

func TestValidAffTemplate(t *testing.T) {
	for _, c := range []struct {
		tmpl string
		want bool
	}{
		{"", false},
		{"https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/id/{workno}.html", false},
		{"https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/locale/zh_CN/id/{product_id}.html/?locale=zh_CN", true},
	} {
		if got := ValidAffTemplate(c.tmpl); got != c.want {
			t.Errorf("ValidAffTemplate(%q) = %v, want %v", c.tmpl, got, c.want)
		}
	}
}
