package devapi

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	siteModel "api/internal/platform/site/model"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration tests against a real Postgres (the community/catalog/trust
// convention): TEST_DATABASE_DSN or a local default; a missing database skips
// the package. The schema is provisioned by the exact production migration
// helpers (AddOAuthClientDevColumns + Models), and provisioned TWICE in TestMain
// as the idempotency probe. `go test -p 1` (pinned in test.yml) serializes
// packages that share the CI database; a dedicated advisory lock adds
// robustness when -p 1 is not set.

var testDB *gorm.DB

// suiteLockKey = "dvap" — distinct from community (0x636f6d6d) and trust keys,
// per the single-keyspace advisory-lock rule.
const suiteLockKey = 0x64766170

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=kun_galgame_infra_test sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: cannot connect to test database: %v\n", err)
		os.Exit(0)
	}
	sqlDB, _ := db.DB()
	release := acquireSuiteLock(sqlDB)

	// First provision creates the schema; the second is the idempotency probe —
	// a re-run of AddOAuthClientDevColumns + AutoMigrate must be a no-op.
	if err := provision(db); err != nil {
		release()
		fmt.Fprintf(os.Stderr, "SKIP: devapi migration failed: %v\n", err)
		os.Exit(0)
	}
	if err := provision(db); err != nil {
		release()
		fmt.Fprintf(os.Stderr, "devapi migration is NOT idempotent: %v\n", err)
		os.Exit(1)
	}

	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

// provision mirrors cmd/migrate's order for the developer-platform schema:
// safely pre-migrate an existing oauth_clients table, then AutoMigrate the
// complete OAuth client and developer-platform models. On a fresh database the
// pre-migration is a no-op and AutoMigrate creates every column.
func provision(db *gorm.DB) error {
	if err := AddOAuthClientDevColumns(db); err != nil {
		return err
	}
	models := []any{&siteModel.Site{}, &siteModel.OAuthClient{}}
	models = append(models, Models()...)
	return db.AutoMigrate(models...)
}

func TestFreshDatabaseMigrationOrder(t *testing.T) {
	tx := testDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin fresh-schema transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	const schema = "devapi_fresh_migration_test"
	if err := tx.Exec(`CREATE SCHEMA devapi_fresh_migration_test`).Error; err != nil {
		t.Fatalf("create fresh schema: %v", err)
	}
	if err := tx.Exec(`SET LOCAL search_path TO devapi_fresh_migration_test`).Error; err != nil {
		t.Fatalf("select fresh schema: %v", err)
	}

	if tx.Migrator().HasTable("oauth_clients") {
		t.Fatal("fresh schema unexpectedly contains oauth_clients")
	}
	if err := AddOAuthClientDevColumns(tx); err != nil {
		t.Fatalf("pre-migrate fresh schema: %v", err)
	}
	if tx.Migrator().HasTable("oauth_clients") {
		t.Fatal("pre-migration must leave fresh table creation to AutoMigrate")
	}

	if err := tx.AutoMigrate(&siteModel.Site{}, &siteModel.OAuthClient{}); err != nil {
		t.Fatalf("auto-migrate fresh schema: %v", err)
	}
	if !tx.Migrator().HasColumn(&siteModel.OAuthClient{}, "dev_enabled") {
		t.Fatal("fresh oauth_clients table is missing dev_enabled")
	}
	if err := AddOAuthClientDevColumns(tx); err != nil {
		t.Fatalf("re-run pre-migration on created table: %v", err)
	}
}

func acquireSuiteLock(db *sql.DB) func() {
	if db == nil {
		return func() {}
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return func() {}
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", suiteLockKey); err != nil {
		_ = conn.Close()
		return func() {}
	}
	return func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", suiteLockKey)
		_ = conn.Close()
	}
}

// --- helpers ---

func cleanup(t *testing.T) {
	t.Helper()
	if err := testDB.Exec(`TRUNCATE developer_api_keys, developer_api_usage RESTART IDENTITY`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := testDB.Exec(`DELETE FROM oauth_clients WHERE id LIKE 'devapitest_%'`).Error; err != nil {
		t.Fatalf("clean clients: %v", err)
	}
}

func makeApp(t *testing.T, clientID string, devEnabled bool) {
	t.Helper()
	app := &siteModel.OAuthClient{
		ID:            clientID,
		Name:          "devapi test app",
		Secret:        "x",
		RedirectURIs:  datatypes.JSON([]byte("[]")),
		Grants:        datatypes.JSON([]byte(`["authorization_code"]`)),
		AllowedScopes: datatypes.JSON([]byte(`["catalog:read","galgame:read"]`)),
		DevEnabled:    devEnabled,
		DevTier:       TierFree,
	}
	if err := testDB.Create(app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
}

func newService(t *testing.T) (*AdminService, *Repository) {
	t.Helper()
	repo := NewRepository(testDB)
	return NewAdminService(repo, newMemStore()), repo
}

// --- migration discipline ---

// TestDevColumnDiscipline asserts the dev_* intent columns carry NO DB default
// (the GORM zero-value trap, 裁定 8).
func TestDevColumnDiscipline(t *testing.T) {
	for _, col := range []string{"dev_enabled", "dev_tier", "dev_nsfw_allowed", "dev_rate_per_min", "dev_quota_daily"} {
		var def *string
		if err := testDB.Raw(
			`SELECT column_default FROM information_schema.columns
			   WHERE table_schema='public' AND table_name='oauth_clients' AND column_name=?`, col,
		).Scan(&def).Error; err != nil {
			t.Fatalf("read default %s: %v", col, err)
		}
		if def != nil {
			t.Errorf("oauth_clients.%s must have NO default (intent column), got %q", col, *def)
		}
	}
}

// TestUsageUniqueIndex asserts the composite unique index carries all four
// columns in declared order (the shared-index priority trap).
func TestUsageUniqueIndex(t *testing.T) {
	var def string
	if err := testDB.Raw(
		`SELECT indexdef FROM pg_indexes WHERE schemaname='public' AND indexname='idx_usage_day'`,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read indexdef: %v", err)
	}
	if def == "" {
		t.Fatal("idx_usage_day does not exist")
	}
	if !strings.Contains(def, "UNIQUE") {
		t.Errorf("idx_usage_day is not unique: %s", def)
	}
	if !strings.Contains(def, "(client_id, key_id, face, day)") {
		t.Errorf("idx_usage_day columns/order wrong: %s", def)
	}
}

// --- key lifecycle ---

func TestKeyLifecycle(t *testing.T) {
	cleanup(t)
	const clientID = "devapitest_lifecycle"
	makeApp(t, clientID, true)
	svc, repo := newService(t)
	ctx := context.Background()
	now := time.Now()

	// Mint → resolves active with the right scopes.
	key, plaintext, err := svc.MintKey(ctx, clientID, MintKeyInput{Name: "k1"}, 42)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if plaintext == "" || !HasKeyPrefix(plaintext) {
		t.Fatalf("mint returned bad plaintext %q", plaintext)
	}
	cred, err := repo.ResolveByHash(ctx, HashKey(plaintext), now)
	if err != nil || cred == nil {
		t.Fatalf("resolve after mint = (%+v, %v), want active", cred, err)
	}
	if cred.KeyID != key.ID || cred.ClientID != clientID || cred.Tier != TierFree {
		t.Errorf("resolved cred mismatch: %+v", cred)
	}
	if !cred.HasScope(ScopeCatalogRead) || !cred.HasScope(ScopeGalgameRead) {
		t.Errorf("resolved scopes = %v, want the two default reads", cred.Scopes)
	}

	// Rotate → both old and new resolve during the grace window.
	newKey, newPlain, err := svc.RotateKey(ctx, key.ID, 42)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newPlain == plaintext {
		t.Fatalf("rotate returned the same plaintext")
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(plaintext), now); c == nil {
		t.Errorf("old key must still resolve during grace window")
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(newPlain), now); c == nil {
		t.Errorf("new key must resolve after rotate")
	}
	// Old key past its grace expiry → no longer resolves.
	old, _ := repo.GetKey(ctx, key.ID)
	if old.ExpiresAt == nil {
		t.Fatalf("rotated-out key should have an expiry")
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(plaintext), old.ExpiresAt.Add(time.Second)); c != nil {
		t.Errorf("old key must not resolve after its expiry")
	}

	// Revoke new key → immediately rejected.
	if err := svc.RevokeKey(ctx, newKey.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(newPlain), now); c != nil {
		t.Errorf("revoked key must not resolve")
	}
}

// TestRevokeBustsCache: revoking a key actively deletes its resolve-cache entry
// (not waiting out the 60s TTL).
func TestRevokeBustsCache(t *testing.T) {
	cleanup(t)
	const clientID = "devapitest_bust"
	makeApp(t, clientID, true)
	store := newMemStore()
	svc := NewAdminService(NewRepository(testDB), store)
	ctx := context.Background()

	key, plaintext, err := svc.MintKey(ctx, clientID, MintKeyInput{Name: "k"}, 1)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// Seed the resolve cache as if the key had been resolved once.
	cacheKey := "devkey:" + hashHex(plaintext)
	store.kv[cacheKey] = []byte("cached")

	if err := svc.RevokeKey(ctx, key.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, present := store.kv[cacheKey]; present {
		t.Errorf("revoke must actively delete the resolve-cache entry %q", cacheKey)
	}
}

// TestDevEnabledGate: a valid key on a disabled app resolves to nothing (401).
func TestDevEnabledGate(t *testing.T) {
	cleanup(t)
	const clientID = "devapitest_disabled"
	makeApp(t, clientID, false) // dev_enabled = false
	svc, repo := newService(t)
	ctx := context.Background()

	_, plaintext, err := svc.MintKey(ctx, clientID, MintKeyInput{Name: "k"}, 1)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(plaintext), time.Now()); c != nil {
		t.Errorf("key on a dev_enabled=false app must not resolve, got %+v", c)
	}

	// Enabling the app makes the same key resolve.
	if _, err := svc.UpdateAppConfig(ctx, clientID, AppConfig{DevEnabled: boolPtr(true)}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if c, _ := repo.ResolveByHash(ctx, HashKey(plaintext), time.Now()); c == nil {
		t.Errorf("key must resolve once the app is enabled")
	}
}

// TestUsageFlushIdempotent: record N → flush → correct row; re-flush is a no-op;
// further records accumulate.
func TestUsageFlushIdempotent(t *testing.T) {
	cleanup(t)
	repo := NewRepository(testDB)
	rec := NewUsageRecorder(repo, newMemStore())
	ctx := context.Background()
	cred := &Credential{KeyID: 100, ClientID: "devapitest_usage"}

	rec.Record(cred, "catalog", 200)
	rec.Record(cred, "catalog", 200)
	rec.Record(cred, "catalog", 404) // 4xx
	if err := rec.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	assertUsage(t, cred.ClientID, cred.KeyID, "catalog", 3, 1, 0)

	// Re-flush with nothing pending must not double-count.
	if err := rec.Flush(ctx); err != nil {
		t.Fatalf("re-flush: %v", err)
	}
	assertUsage(t, cred.ClientID, cred.KeyID, "catalog", 3, 1, 0)

	// Further records accumulate via the ON CONFLICT upsert.
	rec.Record(cred, "catalog", 500) // 5xx
	rec.Record(cred, "catalog", 200)
	if err := rec.Flush(ctx); err != nil {
		t.Fatalf("flush 2: %v", err)
	}
	assertUsage(t, cred.ClientID, cred.KeyID, "catalog", 5, 1, 1)
}

func assertUsage(t *testing.T, clientID string, keyID uint, face string, count, s4xx, s5xx int64) {
	t.Helper()
	var row DeveloperAPIUsage
	err := testDB.Where("client_id = ? AND key_id = ? AND face = ?", clientID, keyID, face).
		Take(&row).Error
	if err != nil {
		t.Fatalf("read usage: %v", err)
	}
	if row.Count != count || row.Status4xx != s4xx || row.Status5xx != s5xx {
		t.Errorf("usage = (count %d, 4xx %d, 5xx %d), want (%d, %d, %d)",
			row.Count, row.Status4xx, row.Status5xx, count, s4xx, s5xx)
	}
}

// TestAddDevColumnsBackfillsExisting exercises the real production migration
// path (裁定 8): the dev_* columns are added to an oauth_clients table that
// ALREADY has rows. The pre-existing row must backfill to the NOT NULL zero
// values, the temporary DEFAULT must be dropped, and a re-run must be a no-op.
// Runs last (it drops + re-adds columns on the shared table, restoring them).
func TestAddDevColumnsBackfillsExisting(t *testing.T) {
	cleanup(t)
	// This test runs DDL (drop + re-add columns) on the shared oauth_clients
	// table. Pooled connections hold prepared-statement plans against the OLD
	// row type, and the next `SELECT *` on any of them fails with "cached plan
	// must not change result type" (SQLSTATE 0A000) — in THIS test and in every
	// test that runs after it (file order puts integration_test before
	// selfservice_test, so "runs last" never held). Flush the pool's idle
	// connections around each DDL phase so no stale plan survives.
	flushPool := func() {
		sqlDB, err := testDB.DB()
		if err != nil {
			t.Fatalf("flush pool: %v", err)
		}
		sqlDB.SetMaxIdleConns(0) // closes idle conns and their statement caches
		sqlDB.SetMaxIdleConns(2)
	}
	defer flushPool()
	// Simulate a pre-migration table: strip the dev columns, then insert a
	// legacy client that predates them.
	for _, col := range []string{"dev_enabled", "dev_tier", "dev_nsfw_allowed", "dev_rate_per_min", "dev_quota_daily", "owner_user_id"} {
		if err := testDB.Exec("ALTER TABLE oauth_clients DROP COLUMN IF EXISTS " + col).Error; err != nil {
			t.Fatalf("drop %s: %v", col, err)
		}
	}
	if err := testDB.Exec(`INSERT INTO oauth_clients (id, name, secret, redirect_uris, grants)
		VALUES ('devapitest_legacy', 'legacy', 'x', '[]'::jsonb, '[]'::jsonb)`).Error; err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	// Add columns (populated table) — then again for idempotency.
	if err := AddOAuthClientDevColumns(testDB); err != nil {
		t.Fatalf("add columns: %v", err)
	}
	if err := AddOAuthClientDevColumns(testDB); err != nil {
		t.Fatalf("add columns not idempotent: %v", err)
	}
	flushPool() // row type just changed twice — drop stale cached plans

	// The legacy row backfilled to the zero values.
	var app siteModel.OAuthClient
	if err := testDB.Where("id = ?", "devapitest_legacy").Take(&app).Error; err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if app.DevEnabled != false || app.DevTier != "free" || app.DevRatePerMin != 0 || app.DevQuotaDaily != 0 {
		t.Errorf("legacy backfill wrong: enabled=%v tier=%q rate=%d quota=%d",
			app.DevEnabled, app.DevTier, app.DevRatePerMin, app.DevQuotaDaily)
	}

	// The temporary DEFAULT was dropped (intent-column discipline).
	var def *string
	if err := testDB.Raw(
		`SELECT column_default FROM information_schema.columns
		   WHERE table_schema='public' AND table_name='oauth_clients' AND column_name='dev_enabled'`,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read default: %v", err)
	}
	if def != nil {
		t.Errorf("dev_enabled default must be dropped, got %q", *def)
	}
}

func boolPtr(b bool) *bool { return &b }
