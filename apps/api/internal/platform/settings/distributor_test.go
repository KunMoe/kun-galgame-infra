package settings_test

import (
	"context"
	"testing"

	"api/internal/platform/settings"

	"gorm.io/datatypes"
)

func TestDistributorStartLoadsExistingRows(t *testing.T) {
	reset(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	n := settings.Int(settings.Meta{
		Name: "dist.n", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(100),
	}, 1)
	m := settings.Int(settings.Meta{
		Name: "dist.m", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(100),
	}, 1)
	reg := settings.NewRegistry(settings.Domain{
		Name: "dist", TitleZH: "dist", Keys: []settings.Entry{n, m},
	})
	if err := testDB.Create(&settings.SettingOverride{
		ScopeKind:       settings.ScopePlatform,
		ScopeID:         "",
		Key:             n.Name(),
		Value:           datatypes.JSON(`9`),
		Version:         1,
		UpdatedByUserID: 1,
		Note:            "",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	dist := settings.NewDistributor(testDB, reg, nil)
	dist.Start(ctx)
	if n.Get() != 9 || n.Source() != settings.SourceDB {
		t.Errorf("Start did not load existing row: Get=%d Source=%q", n.Get(), n.Source())
	}
	if m.Get() != 1 || m.Source() != settings.SourceDefault {
		t.Errorf("unset key must stay default after Start: Get=%d Source=%q", m.Get(), m.Source())
	}

	if err := testDB.Exec(
		`INSERT INTO setting_overrides (scope_kind, scope_id, key, value, version, updated_by_user_id, note, created_at, updated_at)
		 VALUES ('platform', '', ?, '12'::jsonb, 1, 1, '', NOW(), NOW())`, m.Name()).Error; err != nil {
		t.Fatalf("direct insert: %v", err)
	}
	if err := dist.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if m.Get() != 12 || m.Source() != settings.SourceDB {
		t.Errorf("Refresh did not see the SQL insert: Get=%d Source=%q", m.Get(), m.Source())
	}
}

func TestDistributorRefreshFailureKeepsPrevious(t *testing.T) {
	reset(t)
	ctx := context.Background()
	n := settings.Int(settings.Meta{
		Name: "dist.keep", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(100),
	}, 1)
	reg := settings.NewRegistry(settings.Domain{
		Name: "dist", TitleZH: "dist", Keys: []settings.Entry{n},
	})
	if err := testDB.Create(&settings.SettingOverride{
		ScopeKind:       settings.ScopePlatform,
		ScopeID:         "",
		Key:             n.Name(),
		Value:           datatypes.JSON(`7`),
		Version:         1,
		UpdatedByUserID: 1,
		Note:            "",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	dist := settings.NewDistributor(testDB, reg, nil)
	if err := dist.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if n.Get() != 7 {
		t.Fatalf("precondition Get=%d", n.Get())
	}

	if err := testDB.Migrator().DropTable(&settings.SettingOverride{}); err != nil {
		t.Fatalf("drop: %v", err)
	}
	t.Cleanup(func() {
		if err := testDB.AutoMigrate(&settings.SettingOverride{}, &settings.SettingAuditLog{}); err != nil {
			t.Errorf("recreate tables: %v", err)
		}
	})

	if err := dist.Refresh(ctx); err == nil {
		t.Fatal("Refresh against a dropped table must return an error")
	}
	if n.Get() != 7 || n.Source() != settings.SourceDB {
		t.Errorf("previous snapshot must stay, Get=%d Source=%q", n.Get(), n.Source())
	}
}
