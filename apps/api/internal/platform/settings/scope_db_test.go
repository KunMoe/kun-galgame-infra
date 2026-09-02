package settings_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"api/internal/platform/settings"

	"gorm.io/datatypes"
)

func TestStoreSiteScopeIsolated(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()
	store := settings.NewStore(testDB)
	key := "t.siteflag"

	if _, err := store.Set(ctx, settings.SiteScope(3), key, json.RawMessage(`true`), "site3", nil, 7); err != nil {
		t.Fatalf("set site 3: %v", err)
	}

	plat, err := store.Values(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("platform values: %v", err)
	}
	if _, ok := plat[key]; ok {
		t.Errorf("platform values has site key: %s", plat[key])
	}

	s3, err := store.Values(ctx, settings.SiteScope(3))
	if err != nil {
		t.Fatalf("site 3 values: %v", err)
	}
	if string(s3[key]) != "true" {
		t.Errorf("site 3 value = %s, want true", s3[key])
	}

	s4, err := store.Values(ctx, settings.SiteScope(4))
	if err != nil {
		t.Fatalf("site 4 values: %v", err)
	}
	if _, ok := s4[key]; ok {
		t.Errorf("site 4 values has site 3 key: %s", s4[key])
	}

	platO, err := store.Overrides(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("platform overrides: %v", err)
	}
	if len(platO) != 0 {
		t.Errorf("platform overrides = %+v", platO)
	}

	o3, err := store.Overrides(ctx, settings.SiteScope(3))
	if err != nil {
		t.Fatalf("site 3 overrides: %v", err)
	}
	if len(o3) != 1 || o3[0].Key != key {
		t.Errorf("site 3 overrides = %+v", o3)
	}

	o4, err := store.Overrides(ctx, settings.SiteScope(4))
	if err != nil {
		t.Fatalf("site 4 overrides: %v", err)
	}
	if len(o4) != 0 {
		t.Errorf("site 4 overrides = %+v", o4)
	}

	entries, err := store.RecentAudit(ctx, 50)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit has %d rows, want 1: %+v", len(entries), entries)
	}
	if entries[0].ScopeKind != settings.ScopeSite || entries[0].ScopeID != "3" {
		t.Errorf("audit scope = %q %q, want site 3", entries[0].ScopeKind, entries[0].ScopeID)
	}
}

func TestStoreSiteVersionConflict(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()
	store := settings.NewStore(testDB)
	key := "t.sitever"
	scope := settings.SiteScope(3)

	row, err := store.Set(ctx, scope, key, json.RawMessage(`true`), "first", nil, 7)
	if err != nil {
		t.Fatalf("first set: %v", err)
	}
	if row.Version != 1 {
		t.Errorf("first version = %d, want 1", row.Version)
	}

	v1 := int64(1)
	if _, err := store.Set(ctx, scope, key, json.RawMessage(`false`), "second", &v1, 7); err != nil {
		t.Fatalf("expectVersion 1 on version 1 = %v, want nil", err)
	}

	got, err := store.Values(ctx, scope)
	if err != nil {
		t.Fatalf("values after v1 write: %v", err)
	}
	if string(got[key]) != "false" {
		t.Errorf("v1 write = %s, want false", got[key])
	}

	if _, err := store.Set(ctx, scope, key, json.RawMessage(`true`), "stale", &v1, 7); err != settings.ErrVersionConflict {
		t.Errorf("stale version = %v, want ErrVersionConflict", err)
	}
	got, err = store.Values(ctx, scope)
	if err != nil {
		t.Fatalf("values after conflict: %v", err)
	}
	if string(got[key]) != "false" {
		t.Errorf("conflict wrote a value: %s", got[key])
	}

	v99 := int64(99)
	if _, err := store.Set(ctx, scope, "t.missing", json.RawMessage(`true`), "", &v99, 7); err != settings.ErrVersionConflict {
		t.Errorf("non-zero expectVersion on missing = %v, want ErrVersionConflict", err)
	}

	plat, err := store.Values(ctx, settings.PlatformScope)
	if err != nil {
		t.Fatalf("platform values: %v", err)
	}
	if _, ok := plat[key]; ok {
		t.Errorf("site writes leaked to platform: %s", plat[key])
	}
}

func TestServiceSiteScope(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()

	scoped := settings.Bool(settings.Meta{
		Name: "sc.flag", DescEN: "e", DescZH: "z",
		SiteScoped: true,
	}, false)
	plain := settings.Int(settings.Meta{
		Name: "pl.count", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(10),
	}, 1)
	reg := settings.NewRegistry(
		settings.Domain{Name: "sc", TitleZH: "sc", Keys: []settings.Entry{scoped}},
		settings.Domain{Name: "pl", TitleZH: "pl", Keys: []settings.Entry{plain}},
	)
	dist := settings.NewDistributor(testDB, reg, nil)
	svc := settings.NewService(reg, settings.NewStore(testDB), dist)
	site := settings.SiteScope(3)

	_, err := svc.Set(ctx, 7, site, plain.Name(), json.RawMessage(`2`), "", nil)
	if !errors.Is(err, settings.ErrNotSiteScoped) {
		t.Errorf("plain key site set = %v, want ErrNotSiteScoped", err)
	}
	var n int64
	testDB.Model(&settings.SettingOverride{}).Count(&n)
	if n != 0 {
		t.Errorf("rejected site write left %d override row(s)", n)
	}

	view, err := svc.Set(ctx, 7, site, scoped.Name(), json.RawMessage(`true`), "site", nil)
	if err != nil {
		t.Fatalf("set site scoped: %v", err)
	}
	if view.Source != settings.SourceSite || view.Effective != true {
		t.Errorf("set view = %+v", view)
	}
	if view.Override == nil || view.Override.Version != 1 {
		t.Errorf("set override = %+v", view.Override)
	}
	if view.Inherited != false {
		t.Errorf("set view inherited = %v, want the platform default false", view.Inherited)
	}
	if scoped.Get() != false {
		t.Errorf("in-process Get after site set = %v, want default false", scoped.Get())
	}
	if plain.Get() != 1 {
		t.Errorf("plain Get = %d, want 1", plain.Get())
	}

	ov, err := svc.Overview(ctx, true, site)
	if err != nil {
		t.Fatalf("site overview: %v", err)
	}
	if len(ov.Domains) != 1 || ov.Domains[0].Name != "sc" || len(ov.Domains[0].Keys) != 1 {
		t.Fatalf("site overview domains = %+v", ov.Domains)
	}
	kv := ov.Domains[0].Keys[0]
	if kv.Source != settings.SourceSite || kv.Effective != true || kv.Override == nil || kv.Override.Version != 1 {
		t.Errorf("site overview key = %+v", kv)
	}
	if kv.Inherited != false {
		t.Errorf("site overview inherited = %v, want the platform default false", kv.Inherited)
	}

	plat, err := svc.Overview(ctx, true, settings.PlatformScope)
	if err != nil {
		t.Fatalf("platform overview: %v", err)
	}
	if len(plat.Domains) != 2 {
		t.Fatalf("platform overview domains = %+v", plat.Domains)
	}
	found := false
	for _, d := range plat.Domains {
		for _, k := range d.Keys {
			if k.Key == scoped.Name() {
				found = true
				if k.Source != settings.SourceDefault {
					t.Errorf("platform view of scoped key source = %q, want default", k.Source)
				}
			}
		}
	}
	if !found {
		t.Fatal("platform overview missing scoped key")
	}

	view, err = svc.Reset(ctx, 7, site, scoped.Name(), "undo")
	if err != nil {
		t.Fatalf("reset site: %v", err)
	}
	if view.Source != settings.SourceDefault || view.Override != nil {
		t.Errorf("reset view = %+v", view)
	}

	ov, err = svc.Overview(ctx, false, site)
	if err != nil {
		t.Fatalf("overview after site reset: %v", err)
	}
	kv = ov.Domains[0].Keys[0]
	if kv.Source != settings.SourceDefault || kv.Override != nil {
		t.Errorf("overview after site reset = %+v", kv)
	}

	_, err = svc.Reset(ctx, 7, site, scoped.Name(), "")
	if !errors.Is(err, settings.ErrNoOverride) {
		t.Errorf("second reset = %v, want ErrNoOverride", err)
	}
}

func TestServiceSiteWriteRefreshesForSite(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()

	scoped := settings.Bool(settings.Meta{
		Name: "sc.flag", DescEN: "e", DescZH: "z",
		SiteScoped: true,
	}, false)
	reg := settings.NewRegistry(
		settings.Domain{Name: "sc", TitleZH: "sc", Keys: []settings.Entry{scoped}},
	)
	dist := settings.NewDistributor(testDB, reg, nil)
	svc := settings.NewService(reg, settings.NewStore(testDB), dist)
	site := settings.SiteScope(3)

	if _, err := svc.Set(ctx, 7, site, scoped.Name(), json.RawMessage(`true`), "site", nil); err != nil {
		t.Fatalf("set site scoped: %v", err)
	}
	if scoped.ForSite(3) != true {
		t.Errorf("ForSite(3) after Set = %v, want true without a manual Refresh", scoped.ForSite(3))
	}
	if scoped.Get() != false {
		t.Errorf("Get() after site Set = %v, want platform false", scoped.Get())
	}
	if scoped.ForSite(4) != false {
		t.Errorf("ForSite(4) after site 3 Set = %v, want platform false", scoped.ForSite(4))
	}

	if _, err := svc.Reset(ctx, 7, site, scoped.Name(), "undo"); err != nil {
		t.Fatalf("reset site: %v", err)
	}
	if scoped.ForSite(3) != false {
		t.Errorf("ForSite(3) after Reset = %v, want platform fallback false", scoped.ForSite(3))
	}
}

func TestServiceEffective(t *testing.T) {
	reset(t)
	seedActor(t)
	ctx := context.Background()

	a := settings.Bool(settings.Meta{
		Name: "eff.a", DescEN: "e", DescZH: "z",
		Public: true, SiteScoped: true,
	}, false)
	b := settings.Bool(settings.Meta{
		Name: "eff.b", DescEN: "e", DescZH: "z",
		Public: true,
	}, false)
	c := settings.Bool(settings.Meta{
		Name: "eff.c", DescEN: "e", DescZH: "z",
	}, false)
	reg := settings.NewRegistry(settings.Domain{
		Name: "eff", TitleZH: "eff", Keys: []settings.Entry{a, b, c},
	})
	dist := settings.NewDistributor(testDB, reg, nil)
	svc := settings.NewService(reg, settings.NewStore(testDB), dist)

	view, err := svc.Effective(ctx, nil)
	if err != nil {
		t.Fatalf("effective nil: %v", err)
	}
	if view.SiteID != nil {
		t.Errorf("site_id = %v, want nil", view.SiteID)
	}
	if len(view.Settings) != 2 {
		t.Errorf("settings = %+v, want a and b", view.Settings)
	}
	if _, ok := view.Settings[c.Name()]; ok {
		t.Errorf("non-public key leaked: %+v", view.Settings)
	}
	if view.Settings[a.Name()] != false || view.Settings[b.Name()] != false {
		t.Errorf("defaults = %+v", view.Settings)
	}
	etagNil := view.ETag

	again, err := svc.Effective(ctx, nil)
	if err != nil {
		t.Fatalf("effective nil again: %v", err)
	}
	if again.ETag != etagNil {
		t.Errorf("same settings etag %q vs %q", etagNil, again.ETag)
	}

	if _, err := svc.Set(ctx, 7, settings.PlatformScope, a.Name(), json.RawMessage(`true`), "plat", nil); err != nil {
		t.Fatalf("platform set: %v", err)
	}
	view, err = svc.Effective(ctx, nil)
	if err != nil {
		t.Fatalf("effective after platform set: %v", err)
	}
	if view.Settings[a.Name()] != true {
		t.Errorf("platform current not reflected: %+v", view.Settings)
	}
	if view.ETag == etagNil {
		t.Error("etag did not change after platform set")
	}
	etagPlat := view.ETag

	siteID := uint(5)
	if _, err := svc.Set(ctx, 7, settings.SiteScope(siteID), a.Name(), json.RawMessage(`false`), "site", nil); err != nil {
		t.Fatalf("site set: %v", err)
	}
	siteView, err := svc.Effective(ctx, &siteID)
	if err != nil {
		t.Fatalf("effective site 5: %v", err)
	}
	if siteView.SiteID == nil || *siteView.SiteID != 5 {
		t.Errorf("site_id = %v, want 5", siteView.SiteID)
	}
	if siteView.Settings[a.Name()] != false {
		t.Errorf("site 5 a = %v, want false", siteView.Settings[a.Name()])
	}
	if siteView.Settings[b.Name()] != false {
		t.Errorf("site 5 b = %v, want false", siteView.Settings[b.Name()])
	}

	platView, err := svc.Effective(ctx, nil)
	if err != nil {
		t.Fatalf("effective nil after site set: %v", err)
	}
	if platView.Settings[a.Name()] != true {
		t.Errorf("nil site still platform a = %v, want true", platView.Settings[a.Name()])
	}
	if platView.ETag != etagPlat {
		t.Errorf("platform etag changed after site set: %q vs %q", etagPlat, platView.ETag)
	}
	if siteView.ETag == platView.ETag {
		t.Error("site and platform etags must differ")
	}

	againSite, err := svc.Effective(ctx, &siteID)
	if err != nil {
		t.Fatalf("effective site 5 again: %v", err)
	}
	if againSite.ETag != siteView.ETag {
		t.Errorf("same site settings etag %q vs %q", siteView.ETag, againSite.ETag)
	}

	if err := testDB.Where("scope_kind = ? AND scope_id = ? AND key = ?", settings.ScopeSite, "5", a.Name()).
		Delete(&settings.SettingOverride{}).Error; err != nil {
		t.Fatalf("delete site row: %v", err)
	}
	if err := testDB.Create(&settings.SettingOverride{
		ScopeKind:       settings.ScopeSite,
		ScopeID:         "5",
		Key:             a.Name(),
		Value:           datatypes.JSON(`1`),
		Version:         1,
		UpdatedByUserID: 7,
		Note:            "bad",
	}).Error; err != nil {
		t.Fatalf("seed invalid site row: %v", err)
	}
	bad, err := svc.Effective(ctx, &siteID)
	if err != nil {
		t.Fatalf("effective invalid site row: %v", err)
	}
	if bad.Settings[a.Name()] != true {
		t.Errorf("invalid site row must be ignored, a = %v, want platform true", bad.Settings[a.Name()])
	}
}
