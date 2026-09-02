package settings_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"api/internal/platform/settings"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	if dsn, ok := dbtest.DSN(); ok {
		testDB = openTestDB(dsn)
	}
	os.Exit(m.Run())
}

func openTestDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: cannot open the assigned test database: %v\n", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		&settings.SettingOverride{}, &settings.SettingAuditLog{},
	); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: migrate: %v\n", err)
		os.Exit(1)
	}
	// IF NOT EXISTS means this stub loses to the real users table whenever a
	// neighbouring suite in the same database has already AutoMigrated
	// auth/model.User — and that table's email is NOT NULL with no default, so a
	// seed insert naming only (id, name) fails. -p 1 ./... makes that the normal
	// case, not the exception; the columns here and the inserts below have to
	// work against either shape.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '')`).Error; err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: create users stub: %v\n", err)
		os.Exit(1)
	}
	return db
}

func reset(t *testing.T) {
	t.Helper()
	if testDB == nil {
		dbtest.Skip(t)
	}
	for _, table := range []string{"setting_overrides", "setting_audit_logs", "users"} {
		if err := testDB.Exec("TRUNCATE " + table + " RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

func seedActor(t *testing.T) {
	t.Helper()
	if err := testDB.Exec(
		`INSERT INTO users (id, name, email) VALUES (7, 'kun', 'kun@example.invalid')`).Error; err != nil {
		t.Fatalf("seed actor: %v", err)
	}
}

func TestStoreSetResetVersionAndAudit(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()
	store := settings.NewStore(testDB)
	key := "t.count"

	row, err := store.Set(ctx, settings.PlatformScope, key, json.RawMessage(`15`), "first", nil, 7)
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	if row.Version != 1 || row.Key != key || row.UpdatedByUserID != 7 {
		t.Errorf("first row = %+v, want version 1 actor 7", row)
	}

	entries, err := store.RecentAudit(ctx, 50)
	if err != nil {
		t.Fatalf("audit after first set: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit has %d rows, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != settings.ActionSet || entries[0].ActorName != "kun" {
		t.Errorf("first audit = %+v", entries[0])
	}
	if string(entries[0].OldValue) != "null" {
		t.Errorf("first old_value = %q, want JSON null", entries[0].OldValue)
	}
	if string(entries[0].NewValue) != "15" {
		t.Errorf("first new_value = %q, want 15", entries[0].NewValue)
	}

	row, err = store.Set(ctx, settings.PlatformScope, key, json.RawMessage(`16`), "second", nil, 7)
	if err != nil {
		t.Fatalf("second set: %v", err)
	}
	if row.Version != 2 {
		t.Errorf("second version = %d, want 2", row.Version)
	}

	entries, err = store.RecentAudit(ctx, 50)
	if err != nil {
		t.Fatalf("audit after second set: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit has %d rows, want 2: %+v", len(entries), entries)
	}
	if string(entries[0].OldValue) != "15" || string(entries[0].NewValue) != "16" {
		t.Errorf("second audit old/new = %s / %s", entries[0].OldValue, entries[0].NewValue)
	}

	v1 := int64(1)
	if _, err := store.Set(ctx, settings.PlatformScope, key, json.RawMessage(`17`), "stale", &v1, 7); err != settings.ErrVersionConflict {
		t.Errorf("stale version = %v, want ErrVersionConflict", err)
	}
	got, err := store.Values(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("values after conflict: %v", err)
	}
	if string(got[key]) != "16" {
		t.Errorf("conflict wrote a value: %s", got[key])
	}

	v99 := int64(99)
	if _, err := store.Set(ctx, settings.PlatformScope, "t.missing", json.RawMessage(`1`), "", &v99, 7); err != settings.ErrVersionConflict {
		t.Errorf("non-zero expectVersion on missing = %v, want ErrVersionConflict", err)
	}

	if err := store.Reset(ctx, settings.PlatformScope, key, "undo", 7); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := store.Values(ctx, settings.PlatformScope); err != nil {
		t.Fatalf("values after reset: %v", err)
	}
	got, _ = store.Values(ctx, settings.PlatformScope)
	if _, ok := got[key]; ok {
		t.Errorf("reset left a value: %s", got[key])
	}

	entries, err = store.RecentAudit(ctx, 50)
	if err != nil {
		t.Fatalf("audit after reset: %v", err)
	}
	if entries[0].Action != settings.ActionReset {
		t.Errorf("reset action = %q", entries[0].Action)
	}
	if string(entries[0].OldValue) != "16" {
		t.Errorf("reset old_value = %q, want 16", entries[0].OldValue)
	}
	if string(entries[0].NewValue) != "null" {
		t.Errorf("reset new_value = %q, want JSON null", entries[0].NewValue)
	}

	if err := store.Reset(ctx, settings.PlatformScope, key, "", 7); err != settings.ErrNoOverride {
		t.Errorf("reset missing = %v, want ErrNoOverride", err)
	}

	overrides, err := store.Overrides(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("overrides after reset: %v", err)
	}
	if len(overrides) != 0 {
		t.Errorf("overrides after reset = %+v", overrides)
	}

	_, err = store.Set(ctx, settings.PlatformScope, key, json.RawMessage(`1`), "named", nil, 7)
	if err != nil {
		t.Fatalf("set for name join: %v", err)
	}
	overrides, err = store.Overrides(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("overrides: %v", err)
	}
	if len(overrides) != 1 || overrides[0].UpdatedByName != "kun" {
		t.Errorf("updated_by_name = %+v", overrides)
	}
	if overrides[0].Key != key || overrides[0].Version != 1 {
		t.Errorf("override row = %+v", overrides[0])
	}
}
