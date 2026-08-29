package devapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func newSelfService(t *testing.T) (*SelfServiceService, *AdminService, *Repository, *memStore) {
	t.Helper()
	repo := NewRepository(testDB)
	store := newMemStore()
	admin := NewAdminService(repo, store)
	return NewSelfServiceService(repo, admin, store), admin, repo, store
}

func cleanupSelf(t *testing.T) {
	t.Helper()
	if err := testDB.Exec(`TRUNCATE developer_api_keys, developer_api_usage RESTART IDENTITY`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := testDB.Exec(`DELETE FROM oauth_clients WHERE owner_user_id IS NOT NULL OR id LIKE 'devapitest_%'`).Error; err != nil {
		t.Fatalf("clean clients: %v", err)
	}
	cleanupPolicies(t)
}

func TestSelfServiceCreateAndOwnerScope(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const userA, userB = uint(1), uint(2)

	app, err := svc.CreateApp(ctx, userA, "my app", "a short description", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if app.OwnerUserID == nil || *app.OwnerUserID != userA {
		t.Fatalf("owner = %v, want %d", app.OwnerUserID, userA)
	}
	if !app.DevEnabled || app.DevTier != TierFree {
		t.Errorf("new app should be dev_enabled free, got enabled=%v tier=%q", app.DevEnabled, app.DevTier)
	}
	if app.Tagline != "a short description" {
		t.Errorf("description not stored in tagline, got %q", app.Tagline)
	}

	aApps, _ := svc.ListApps(ctx, userA)
	if len(aApps) != 1 {
		t.Fatalf("A app count = %d, want 1", len(aApps))
	}
	bApps, _ := svc.ListApps(ctx, userB)
	if len(bApps) != 0 {
		t.Fatalf("B app count = %d, want 0", len(bApps))
	}
	if _, err := svc.GetApp(ctx, userB, app.ID); err == nil {
		t.Errorf("B GET of A's app must be record-not-found, got nil")
	}
}

func TestSelfServiceAppCap(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	for i := 0; i < MaxAppsPerOwner; i++ {
		if _, err := svc.CreateApp(ctx, owner, fmt.Sprintf("app %d", i), "", nil); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := svc.CreateApp(ctx, owner, "over the cap", "", nil); err != ErrAppLimitReached {
		t.Fatalf("6th app err = %v, want ErrAppLimitReached", err)
	}
}

func TestSelfServiceKeyCapAndScope(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	app, err := svc.CreateApp(ctx, owner, "keyed", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "bad", Scopes: []string{ScopeGalgameNSFW}}); err != ErrScopeNotAllowed {
		t.Fatalf("nsfw scope err = %v, want ErrScopeNotAllowed", err)
	}
	if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "bad-write", Scopes: []string{ScopeGalgameWrite}}); err != ErrScopeNotAllowed {
		t.Fatalf("write scope err = %v, want ErrScopeNotAllowed", err)
	}

	for i := 0; i < MaxActiveKeysPerApp; i++ {
		if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: fmt.Sprintf("k%d", i)}); err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
	}
	if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "over"}); err != ErrKeyLimitReached {
		t.Fatalf("6th key err = %v, want ErrKeyLimitReached", err)
	}

	keys, _ := svc.ListKeys(ctx, owner, app.ID)
	if _, err := svc.RevokeKey(ctx, owner, app.ID, keys[0].ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "back under"}); err != nil {
		t.Fatalf("mint after revoke: %v", err)
	}
}

func TestSelfServiceKeyOwnerGuard(t *testing.T) {
	cleanupSelf(t)
	svc, _, repo, _ := newSelfService(t)
	ctx := context.Background()
	const userA, userB = uint(1), uint(2)

	app, _ := svc.CreateApp(ctx, userA, "A app", "", nil)
	key, plaintext, err := svc.MintKey(ctx, userA, app.ID, MintKeyInput{Name: "k"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	if _, _, err := svc.RotateKey(ctx, userB, app.ID, key.ID); err == nil {
		t.Errorf("B rotate of A's key must be record-not-found")
	}
	if _, err := svc.RevokeKey(ctx, userB, app.ID, key.ID); err == nil {
		t.Errorf("B revoke of A's key must be record-not-found")
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(plaintext), time.Now()); c == nil {
		t.Errorf("A's key must still resolve after B's failed attempts")
	}
}

func TestSelfServiceArchiveCascade(t *testing.T) {
	cleanupSelf(t)
	svc, admin, repo, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	app, _ := svc.CreateApp(ctx, owner, "doomed", "", nil)
	used, p1, _ := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "k1"})
	unused, p2, _ := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "k2"})
	giveKeyHistory(t, repo, used.ID)
	eligible := true
	if _, err := admin.UpdateAppConfig(ctx, app.ID, AppConfig{StoreSettlementEligible: &eligible}); err != nil {
		t.Fatalf("enrol on the settlement roster: %v", err)
	}

	if err := svc.ArchiveApp(ctx, owner, app.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(p1), time.Now()); c != nil {
		t.Errorf("k1 must not resolve after archive")
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(p2), time.Now()); c != nil {
		t.Errorf("k2 must not resolve after archive")
	}
	if _, err := repo.GetKey(ctx, used.ID); err != nil {
		t.Errorf("the key that served a request = %v, want its row kept", err)
	}
	if _, err := repo.GetKey(ctx, unused.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("the key that never served a request = %v, want its row deleted", err)
	}
	reloaded, err := repo.GetApp(ctx, app.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.DevEnabled {
		t.Errorf("app must be dev_enabled=false after archive")
	}
	if reloaded.DevArchivedAt == nil {
		t.Errorf("app must carry dev_archived_at after archive")
	}
	if reloaded.StoreSettlementEligible {
		t.Errorf("archive must take the app off the settlement roster")
	}

	mine, err := svc.ListApps(ctx, owner)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(mine) != 0 {
		t.Errorf("owner list = %d apps, want the archived one gone", len(mine))
	}
	if _, err := repo.GetAppByOwner(ctx, app.ID, owner); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("owner-scoped get of an archived app = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestSelfServiceUsageShape(t *testing.T) {
	cleanupSelf(t)
	svc, _, repo, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	app, _ := svc.CreateApp(ctx, owner, "used", "", nil)
	rec := NewUsageRecorder(repo, newMemStore())
	// Deliberately three different route patterns under face=catalog: the
	// day+face rollup must aggregate over the path dimension, not split by it.
	rec.Record(&Credential{KeyID: 11, ClientID: app.ID}, "catalog", "/v1/catalog/works/:id", 200)
	rec.Record(&Credential{KeyID: 11, ClientID: app.ID}, "catalog", "/v1/catalog/search", 404)
	rec.Record(&Credential{KeyID: 22, ClientID: app.ID}, "catalog", "/v1/catalog/lookup", 200)
	rec.Record(&Credential{KeyID: 11, ClientID: app.ID}, "galgame", "/v1/galgame/:id", 500)
	if err := rec.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	rows, err := svc.Usage(ctx, owner, app.ID, 7)
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	byFace := map[string]UsageDayFace{}
	for _, r := range rows {
		byFace[r.Face] = r
	}
	if got := byFace["catalog"]; got.Count != 3 || got.Status4xx != 1 || got.Status5xx != 0 {
		t.Errorf("catalog agg = %+v, want count 3 / 4xx 1 / 5xx 0", got)
	}
	if got := byFace["galgame"]; got.Count != 1 || got.Status5xx != 1 {
		t.Errorf("galgame agg = %+v, want count 1 / 5xx 1", got)
	}
	if _, err := svc.Usage(ctx, uint(2), app.ID, 7); err == nil {
		t.Errorf("non-owner usage must be record-not-found")
	}
}

func TestSelfServiceOwnerUsage(t *testing.T) {
	cleanupSelf(t)
	svc, _, repo, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	appA, _ := svc.CreateApp(ctx, owner, "app-a", "", nil)
	appB, _ := svc.CreateApp(ctx, owner, "app-b", "", nil)

	rec := NewUsageRecorder(repo, newMemStore())
	rec.Record(&Credential{KeyID: 1, ClientID: appA.ID}, "catalog", "/v1/catalog/works/:id", 200)
	rec.Record(&Credential{KeyID: 1, ClientID: appA.ID}, "catalog", "/v1/catalog/search", 200)
	rec.Record(&Credential{KeyID: 1, ClientID: appA.ID}, "catalog", "/v1/catalog/search", 404)
	rec.Record(&Credential{KeyID: 2, ClientID: appA.ID}, "galgame", "/v1/galgame/:id", 500)
	rec.Record(&Credential{KeyID: 3, ClientID: appB.ID}, "catalog", "/v1/catalog/works", 200)
	rec.Record(&Credential{KeyID: 3, ClientID: appB.ID}, "catalog", "/v1/catalog/works/:id", 200)
	if err := rec.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	sum, err := svc.OwnerUsage(ctx, owner, 7)
	if err != nil {
		t.Fatalf("owner usage: %v", err)
	}

	if len(sum.Daily) != 7 {
		t.Fatalf("daily len = %d, want 7 (dense)", len(sum.Daily))
	}
	var dailySum int64
	for _, d := range sum.Daily {
		dailySum += d.Count
	}
	if dailySum != 6 || sum.TotalCount != 6 || sum.Total4xx != 1 || sum.Total5xx != 1 {
		t.Errorf("totals = daily %d / total %d / 4xx %d / 5xx %d, want 6/6/1/1",
			dailySum, sum.TotalCount, sum.Total4xx, sum.Total5xx)
	}

	if len(sum.ByApp) != 2 {
		t.Fatalf("by_app len = %d, want 2", len(sum.ByApp))
	}
	if a := sum.ByApp[0]; a.Name != "app-a" || a.Count != 4 || a.Status4xx != 1 || a.Status5xx != 1 {
		t.Errorf("by_app[0] = %+v, want app-a 4/1/1", a)
	}
	if b := sum.ByApp[1]; b.Name != "app-b" || b.Count != 2 {
		t.Errorf("by_app[1] = %+v, want app-b 2", b)
	}

	byFace := map[string]UsageFaceTotal{}
	for _, f := range sum.ByFace {
		byFace[f.Face] = f
	}
	if len(sum.ByFace) != 2 || sum.ByFace[0].Face != "catalog" {
		t.Fatalf("by_face = %+v, want catalog first of two", sum.ByFace)
	}
	if c := byFace["catalog"]; c.Count != 5 || c.Status4xx != 1 || c.Status5xx != 0 {
		t.Errorf("by_face[catalog] = %+v, want count 5 / 4xx 1 / 5xx 0", c)
	}
	if g := byFace["galgame"]; g.Count != 1 || g.Status5xx != 1 {
		t.Errorf("by_face[galgame] = %+v, want count 1 / 5xx 1", g)
	}

	if len(sum.Live) != 0 || sum.LiveUnavailable {
		t.Errorf("live = %+v (unavailable=%v), want empty + available", sum.Live, sum.LiveUnavailable)
	}

	empty, err := svc.OwnerUsage(ctx, uint(999), 7)
	if err != nil {
		t.Fatalf("empty-owner usage: %v", err)
	}
	if len(empty.Daily) != 7 || empty.TotalCount != 0 || len(empty.ByApp) != 0 {
		t.Errorf("empty-owner = daily %d / total %d / apps %d, want 7/0/0",
			len(empty.Daily), empty.TotalCount, len(empty.ByApp))
	}
	if len(empty.ByFace) != 0 || len(empty.Live) != 0 || empty.LiveUnavailable {
		t.Errorf("empty-owner by_face %d / live %d / unavailable %v, want 0/0/false",
			len(empty.ByFace), len(empty.Live), empty.LiveUnavailable)
	}
}

func TestSelfServiceOwnerUsageLive(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, store := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	app, err := svc.CreateApp(ctx, owner, "live-app", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	key, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "k"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	now := time.Now().UTC()
	store.setCounter(quotaCounterKey(key.ID, now.Format("2006-01-02")), 120)

	sum, err := svc.OwnerUsage(ctx, owner, 7)
	if err != nil {
		t.Fatalf("owner usage: %v", err)
	}
	if sum.LiveUnavailable {
		t.Fatalf("live must be available with a healthy store")
	}
	if len(sum.Live) != 1 {
		t.Fatalf("live len = %d, want 1", len(sum.Live))
	}
	l := sum.Live[0]
	wantRate, wantQuota, _ := TierLimits(TierFree)
	if l.AppName != "live-app" || l.KeyID != key.ID {
		t.Errorf("live[0] identity = %q/%d, want live-app/%d", l.AppName, l.KeyID, key.ID)
	}
	if l.RateLimit != wantRate || l.QuotaLimit != wantQuota {
		t.Errorf("live[0] limits = rate %d / quota %d, want %d/%d", l.RateLimit, l.QuotaLimit, wantRate, wantQuota)
	}
	if l.QuotaUsed != 120 || l.QuotaRemaining != int64(wantQuota)-120 {
		t.Errorf("live[0] usage = used %d / remaining %d, want 120/%d", l.QuotaUsed, l.QuotaRemaining, int64(wantQuota)-120)
	}
	if l.QuotaReset != nextDayStartUnix(now) {
		t.Errorf("live[0] quota_reset = %d, want %d", l.QuotaReset, nextDayStartUnix(now))
	}

	store.failing = true
	deg, err := svc.OwnerUsage(ctx, owner, 7)
	if err != nil {
		t.Fatalf("owner usage under outage: %v", err)
	}
	if !deg.LiveUnavailable || len(deg.Live) != 0 {
		t.Errorf("under outage: live_unavailable=%v live=%d, want true/0", deg.LiveUnavailable, len(deg.Live))
	}
}

func fakeAuth(userID uint) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	}
}

func TestSelfServiceHandlerOwnerGuard404(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const userA, userB = uint(1), uint(2)

	app, _ := svc.CreateApp(ctx, userA, "A app", "", nil)
	key, _, err := svc.MintKey(ctx, userA, app.ID, MintKeyInput{Name: "k"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	fiberApp := fiber.New()
	h := NewSelfServiceHandler(svc)
	g := fiberApp.Group("/dev", fakeAuth(userB))
	h.Register(g)

	base := "/dev/apps/" + app.ID
	kbase := base + "/keys/" + fmt.Sprint(key.ID)
	cases := []struct {
		method, path string
		body         string
	}{
		{"GET", base, ""},
		{"PATCH", base, `{"name":"hacked"}`},
		{"DELETE", base, ""},
		{"POST", base + "/keys", `{"name":"x"}`},
		{"GET", base + "/keys", ""},
		{"POST", kbase + "/rotate", ""},
		{"DELETE", kbase, ""},
		{"GET", base + "/usage", ""},
	}
	for _, tc := range cases {
		var body io.Reader
		if tc.body != "" {
			body = bytes.NewBufferString(tc.body)
		}
		req := httptest.NewRequest(tc.method, tc.path, body)
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := fiberApp.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode != fiber.StatusNotFound {
			t.Errorf("%s %s as non-owner = %d, want 404", tc.method, tc.path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	ownerApp := fiber.New()
	og := ownerApp.Group("/dev", fakeAuth(userA))
	NewSelfServiceHandler(svc).Register(og)
	resp, _ := ownerApp.Test(httptest.NewRequest("GET", base, nil))
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("owner GET = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestSelfServiceMintShowOnce(t *testing.T) {
	cleanupSelf(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)
	app, _ := svc.CreateApp(ctx, owner, "showonce", "", nil)

	fiberApp := fiber.New()
	NewSelfServiceHandler(svc).Register(fiberApp.Group("/dev", fakeAuth(owner)))

	mintReq := httptest.NewRequest("POST", "/dev/apps/"+app.ID+"/keys", bytes.NewBufferString(`{"name":"k"}`))
	mintReq.Header.Set("Content-Type", "application/json")
	resp, err := fiberApp.Test(mintReq)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var minted struct {
		Data struct {
			Key       string `json:"key"`
			KeyPrefix string `json:"key_prefix"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &minted); err != nil {
		t.Fatalf("decode mint: %v", err)
	}
	if !HasKeyPrefix(minted.Data.Key) {
		t.Fatalf("mint response missing plaintext key, got %q", minted.Data.Key)
	}
	plaintext := minted.Data.Key

	lresp, err := fiberApp.Test(httptest.NewRequest("GET", "/dev/apps/"+app.ID+"/keys", nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	raw, _ := io.ReadAll(lresp.Body)
	_ = lresp.Body.Close()
	if bytes.Contains(raw, []byte(plaintext)) {
		t.Errorf("list response leaks the plaintext key")
	}
	if !bytes.Contains(raw, []byte(minted.Data.KeyPrefix)) {
		t.Errorf("list response should carry the key prefix %q", minted.Data.KeyPrefix)
	}
}
