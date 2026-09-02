package settings_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"api/internal/platform/settings"

	"gorm.io/datatypes"
)

func serviceHarness(t *testing.T) (*settings.Service, *settings.Key[int64], *settings.Distributor) {
	t.Helper()
	count := settings.Int(settings.Meta{
		Name: "svc.count", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(10),
	}, 1)
	reg := settings.NewRegistry(settings.Domain{
		Name: "svc", TitleZH: "svc", Keys: []settings.Entry{count},
	})
	dist := settings.NewDistributor(testDB, reg, nil)
	svc := settings.NewService(reg, settings.NewStore(testDB), dist)
	return svc, count, dist
}

func TestServiceSetUnknownAndInvalid(t *testing.T) {
	reset(t)
	ctx := context.Background()
	svc, _, _ := serviceHarness(t)

	_, err := svc.Set(ctx, 7, settings.PlatformScope, "nope.key", json.RawMessage(`1`), "", nil)
	if !errors.Is(err, settings.ErrUnknownKey) {
		t.Errorf("unknown key = %v, want ErrUnknownKey", err)
	}

	_, err = svc.Set(ctx, 7, settings.PlatformScope, "svc.count", json.RawMessage(`true`), "", nil)
	if !errors.Is(err, settings.ErrInvalidValue) {
		t.Errorf("wrong kind = %v, want ErrInvalidValue", err)
	}

	_, err = svc.Set(ctx, 7, settings.PlatformScope, "svc.count", json.RawMessage(`99`), "", nil)
	if !errors.Is(err, settings.ErrInvalidValue) {
		t.Errorf("out of bounds = %v, want ErrInvalidValue", err)
	}

	var n int64
	testDB.Model(&settings.SettingOverride{}).Count(&n)
	if n != 0 {
		t.Errorf("rejected writes left %d override row(s)", n)
	}
}

func TestServiceSetResetRefreshAndOverview(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()
	svc, count, _ := serviceHarness(t)

	view, err := svc.Set(ctx, 7, settings.PlatformScope, count.Name(), json.RawMessage(`8`), "up", nil)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if count.Get() != 8 || count.Source() != settings.SourceDB {
		t.Errorf("after set Get/Source = %d %q", count.Get(), count.Source())
	}
	if view.Source != settings.SourceDB || view.Override == nil || view.Effective != int64(8) {
		t.Errorf("set view = %+v", view)
	}

	ov, err := svc.Overview(ctx, true, settings.PlatformScope)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !ov.Writable {
		t.Error("writable must pass through")
	}
	kv := ov.Domains[0].Keys[0]
	if kv.Source != settings.SourceDB || kv.Override == nil || kv.Override.UpdatedByName != "kun" {
		t.Errorf("overview after set = %+v", kv)
	}

	view, err = svc.Reset(ctx, 7, settings.PlatformScope, count.Name(), "undo")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if count.Get() != 1 || count.Source() != settings.SourceDefault {
		t.Errorf("after reset Get/Source = %d %q", count.Get(), count.Source())
	}
	if view.Source != settings.SourceDefault || view.Override != nil {
		t.Errorf("reset view = %+v", view)
	}

	ov, err = svc.Overview(ctx, false, settings.PlatformScope)
	if err != nil {
		t.Fatalf("overview after reset: %v", err)
	}
	kv = ov.Domains[0].Keys[0]
	if kv.Source != settings.SourceDefault || kv.Override != nil {
		t.Errorf("overview after reset = %+v", kv)
	}
}

func TestServiceOverviewKeepsInvalidOverride(t *testing.T) {
	reset(t)
	ctx := context.Background()
	svc, count, _ := serviceHarness(t)

	if err := testDB.Create(&settings.SettingOverride{
		ScopeKind:       settings.ScopePlatform,
		ScopeID:         "",
		Key:             count.Name(),
		Value:           datatypes.JSON(`true`),
		Version:         1,
		UpdatedByUserID: 1,
		Note:            "bad",
	}).Error; err != nil {
		t.Fatalf("seed invalid row: %v", err)
	}

	ov, err := svc.Overview(ctx, false, settings.PlatformScope)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	kv := ov.Domains[0].Keys[0]
	if kv.Source != settings.SourceDefault || kv.Effective != int64(1) {
		t.Errorf("invalid row must fall to default, got source=%q effective=%v", kv.Source, kv.Effective)
	}
	if kv.Override == nil || kv.Override.Version != 1 || kv.Override.Note != "bad" {
		t.Errorf("invalid row must still fill override: %+v", kv.Override)
	}
}
