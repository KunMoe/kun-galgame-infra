package permissions_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"api/internal/platform/authz"
	"api/internal/platform/permissions"
	trustPerm "api/internal/platform/trust/perm"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("permissions overlay suite")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: cannot open the assigned test database: %v\n", err)
		os.Exit(1)
	}
	if err := permissions.AddOverrideEffectColumn(db); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: add effect column: %v\n", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		&permissions.RolePermissionOverride{}, &permissions.PermissionAuditLog{},
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
	testDB = db
	os.Exit(m.Run())
}

func reset(t *testing.T) {
	t.Helper()
	for _, table := range []string{"role_permission_overrides", "permission_audit_logs", "users"} {
		if err := testDB.Exec("TRUNCATE " + table + " RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

func TestStoreGrantRevokeAndAudit(t *testing.T) {
	reset(t)
	ctx := context.Background()
	store := permissions.NewStore(testDB)

	if err := testDB.Exec(
		`INSERT INTO users (id, name, email) VALUES (7, 'kun', 'kun@example.invalid')`).Error; err != nil {
		t.Fatalf("seed actor: %v", err)
	}

	key := string(trustPerm.TermManage)
	if err := store.Add(ctx, permissions.RoleModerator, key, permissions.EffectGrant, 7); err != nil {
		t.Fatalf("grant: %v", err)
	}

	effect, err := store.OverrideEffect(ctx, permissions.RoleModerator, key)
	if err != nil || effect != permissions.EffectGrant {
		t.Fatalf("OverrideEffect after grant = %q, %v; want grant, nil", effect, err)
	}

	if err := store.Add(ctx, permissions.RoleModerator, key, permissions.EffectGrant, 7); err != permissions.ErrAlreadyGranted {
		t.Errorf("duplicate grant = %v, want ErrAlreadyGranted", err)
	}
	if err := store.Add(ctx, permissions.RoleModerator, key, permissions.EffectDeny, 7); err != permissions.ErrAlreadyGranted {
		t.Errorf("deny over an existing grant row = %v, want ErrAlreadyGranted", err)
	}

	if err := store.Remove(ctx, permissions.RoleModerator, key, 7); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if effect, _ := store.OverrideEffect(ctx, permissions.RoleModerator, key); effect != "" {
		t.Errorf("override row survived the revoke (effect %q)", effect)
	}
	if err := store.Remove(ctx, permissions.RoleModerator, key, 7); err != permissions.ErrNotGranted {
		t.Errorf("revoke with nothing to revoke = %v, want ErrNotGranted", err)
	}

	entries, err := store.RecentAudit(ctx, 50)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit has %d rows, want 2 (grant + revoke): %+v", len(entries), entries)
	}
	if entries[0].Action != permissions.ActionRevoke || entries[1].Action != permissions.ActionGrant {
		t.Errorf("audit order = %q, %q; want revoke, grant", entries[0].Action, entries[1].Action)
	}
	if entries[0].ActorName != "kun" {
		t.Errorf("actor name = %q, want kun", entries[0].ActorName)
	}
	if entries[0].Permission != key || entries[0].Role != permissions.RoleModerator {
		t.Errorf("audit row lost its subject: %+v", entries[0])
	}
}

func TestDistributorRefreshSwapsLiveResolvers(t *testing.T) {
	reset(t)
	ctx := context.Background()
	reg := permissions.Live()
	dist := permissions.NewDistributor(testDB, reg, nil)
	t.Cleanup(func() {
		reset(t)
		if err := dist.Refresh(ctx); err != nil {
			t.Errorf("cleanup refresh: %v", err)
		}
	})

	if trustPerm.Resolver.Can([]string{"moderator"}, trustPerm.TermManage) {
		t.Fatal("moderator must start without trust.term_manage")
	}

	store := permissions.NewStore(testDB)
	if err := store.Add(ctx, permissions.RoleModerator, string(trustPerm.TermManage), permissions.EffectGrant, 1); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := dist.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !trustPerm.Resolver.Can([]string{"moderator"}, trustPerm.TermManage) {
		t.Error("the overlay grant did not reach the live trust resolver")
	}
	if !trustPerm.Resolver.Can([]string{"admin"}, trustPerm.TermManage) {
		t.Error("the refresh dropped admin's code-floor grant")
	}

	if err := store.Remove(ctx, permissions.RoleModerator, string(trustPerm.TermManage), 1); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := dist.Refresh(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if trustPerm.Resolver.Can([]string{"moderator"}, trustPerm.TermManage) {
		t.Error("the revoke did not return moderator to the code floor")
	}
}

func TestLoadSnapshotDropsRetiredKeys(t *testing.T) {
	reset(t)
	ctx := context.Background()

	rows := []permissions.RolePermissionOverride{
		{Role: permissions.RoleModerator, Permission: string(trustPerm.TermManage),
			Effect: permissions.EffectGrant, GrantedByUserID: 1},
		{Role: permissions.RoleModerator, Permission: "galgame.publish_direct",
			Effect: permissions.EffectGrant, GrantedByUserID: 1},
	}
	if err := testDB.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	snap, err := permissions.LoadSnapshot(ctx, testDB, permissions.Live())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := snap.Grants[permissions.RoleModerator]
	if len(got) != 1 || got[0] != trustPerm.TermManage {
		t.Errorf("snapshot = %v, want only %q", got, trustPerm.TermManage)
	}
}

func TestServiceApplyEnforcesValidationBeforeWriting(t *testing.T) {
	reset(t)
	ctx := context.Background()
	reg := permissions.Live()
	dist := permissions.NewDistributor(testDB, reg, nil)
	t.Cleanup(func() {
		reset(t)
		if err := dist.Refresh(ctx); err != nil {
			t.Errorf("cleanup refresh: %v", err)
		}
	})
	svc := permissions.NewService(reg, permissions.NewStore(testDB), dist)

	ren := permissions.Caller{UserID: 1, Roles: []string{"admin", "ren"}}

	err := svc.Apply(ctx, ren, permissions.WriteRequest{
		Add: true, Effect: permissions.EffectGrant,
		Role: permissions.RoleAdmin, Permission: authz.Permission("oauth.sites.manage_all"),
	})
	assertRejected(t, err, "不可委派")

	err = svc.Apply(ctx, ren, permissions.WriteRequest{
		Add: true, Effect: permissions.EffectGrant,
		Role: permissions.RoleModerator, Permission: authz.Permission("oauth.users.pii_view"),
	})
	assertRejected(t, err, "授予 admin")

	var overrides, audits int64
	testDB.Model(&permissions.RolePermissionOverride{}).Count(&overrides)
	testDB.Model(&permissions.PermissionAuditLog{}).Count(&audits)
	if overrides != 0 || audits != 0 {
		t.Errorf("a rejected write left %d override(s) and %d audit row(s)", overrides, audits)
	}

	if err := svc.Apply(ctx, ren, permissions.WriteRequest{
		Add: true, Effect: permissions.EffectGrant,
		Role: permissions.RoleAdmin, Permission: authz.Permission("oauth.users.pii_view"),
	}); err != nil {
		t.Fatalf("granting admin first must pass: %v", err)
	}
	if err := svc.Apply(ctx, ren, permissions.WriteRequest{
		Add: true, Effect: permissions.EffectGrant,
		Role: permissions.RoleModerator, Permission: authz.Permission("oauth.users.pii_view"),
	}); err != nil {
		t.Fatalf("granting moderator after admin must pass: %v", err)
	}
	testDB.Model(&permissions.PermissionAuditLog{}).Count(&audits)
	if audits != 2 {
		t.Errorf("audit rows = %d, want 2", audits)
	}
}

func TestMatrixReportsSourcesAndEditability(t *testing.T) {
	reset(t)
	ctx := context.Background()
	reg := permissions.Live()
	dist := permissions.NewDistributor(testDB, reg, nil)
	t.Cleanup(func() {
		reset(t)
		if err := dist.Refresh(ctx); err != nil {
			t.Errorf("cleanup refresh: %v", err)
		}
	})
	svc := permissions.NewService(reg, permissions.NewStore(testDB), dist)
	ren := permissions.Caller{UserID: 1, Roles: []string{"admin", "ren"}}

	if err := svc.Apply(ctx, ren, permissions.WriteRequest{
		Add: true, Effect: permissions.EffectGrant,
		Role: permissions.RoleAdmin, Permission: authz.Permission("oauth.users.pii_view"),
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	m, err := svc.Matrix(ctx, ren)
	if err != nil {
		t.Fatalf("matrix: %v", err)
	}
	if !m.ManagesPermissions {
		t.Error("ren must be reported as holding oauth.permissions.manage")
	}

	row := findKey(t, m, "oauth.users.pii_view")
	if c := row.Grants[permissions.RoleAdmin]; !c.Granted || c.Source != permissions.SourceGrant || !c.Editable {
		t.Errorf("admin cell = %+v; want granted from the overlay and editable", c)
	}
	if c := row.Grants[permissions.RoleAdmin]; c.CanDeny || c.CanRestore {
		t.Errorf("admin cell = %+v; a grant-row cell must not offer deny/restore", c)
	}
	if c := row.Grants[permissions.RoleRen]; !c.Granted || c.Source != permissions.SourceCode ||
		c.Editable || c.CanDeny || c.CanRestore {
		t.Errorf("ren cell = %+v; want a granted code cell with no operation at all", c)
	}
	if c := row.Grants[permissions.RoleUser]; c.Granted || c.Editable || c.CanDeny || c.CanRestore {
		t.Errorf("user cell = %+v; want empty and inert", c)
	}

	locked := findKey(t, m, "oauth.sites.manage_all")
	if !locked.NonDelegable {
		t.Error("oauth.sites.manage_all must be flagged non-delegable")
	}
	for _, role := range permissions.EditableRoles {
		if c := locked.Grants[role]; c.Editable || c.CanDeny || c.CanRestore {
			t.Errorf("a non-delegable key must offer nothing on %s: %+v", role, c)
		}
	}

	admin := permissions.Caller{UserID: 2, Roles: []string{"admin"}}
	am, err := svc.Matrix(ctx, admin)
	if err != nil {
		t.Fatalf("matrix (admin): %v", err)
	}
	if am.ManagesPermissions {
		t.Error("a plain admin must not report permissions.manage")
	}
	arow := findKey(t, am, "oauth.users.pii_view")
	if arow.Grants[permissions.RoleCreator].Editable {
		t.Error("the creator column must be ren-only")
	}
	if arow.Grants[permissions.RoleAdmin].Editable {
		t.Error("an admin must not edit its own tier")
	}
}

func TestStoreNeverWritesAnEmptyEffect(t *testing.T) {
	reset(t)
	ctx := context.Background()
	store := permissions.NewStore(testDB)

	key := string(trustPerm.TermManage)
	if err := store.Add(ctx, permissions.RoleModerator, key, permissions.EffectGrant, 1); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := store.Add(ctx, permissions.RoleAdmin, key, permissions.EffectDeny, 1); err != nil {
		t.Fatalf("deny: %v", err)
	}

	var blank int64
	testDB.Model(&permissions.RolePermissionOverride{}).
		Where("effect IS NULL OR effect = ''").Count(&blank)
	if blank != 0 {
		t.Errorf("%d override row(s) carry no effect", blank)
	}

	var rows []permissions.RolePermissionOverride
	if err := testDB.Order("role").Find(&rows).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(rows) != 2 || rows[0].Effect != permissions.EffectDeny || rows[1].Effect != permissions.EffectGrant {
		t.Errorf("rows did not round-trip their effects: %+v", rows)
	}

	if err := store.Add(ctx, permissions.RoleCreator, key, "", 1); err != permissions.ErrBadEffect {
		t.Errorf("empty effect = %v, want ErrBadEffect", err)
	}
	if err := store.Add(ctx, permissions.RoleCreator, key, "revoke", 1); err != permissions.ErrBadEffect {
		t.Errorf("bogus effect = %v, want ErrBadEffect", err)
	}
}

func TestDenyReachesTheLiveResolverAndIsAudited(t *testing.T) {
	reset(t)
	ctx := context.Background()
	reg := permissions.Live()
	dist := permissions.NewDistributor(testDB, reg, nil)
	t.Cleanup(func() {
		reset(t)
		if err := dist.Refresh(ctx); err != nil {
			t.Errorf("cleanup refresh: %v", err)
		}
	})
	svc := permissions.NewService(reg, permissions.NewStore(testDB), dist)
	ren := permissions.Caller{UserID: 1, Roles: []string{"admin", "ren"}}

	if !trustPerm.Resolver.Can([]string{"admin"}, trustPerm.TermManage) {
		t.Fatal("admin must start WITH trust.term_manage — this test denies it")
	}

	denyReq := permissions.WriteRequest{
		Add: true, Effect: permissions.EffectDeny,
		Role: permissions.RoleAdmin, Permission: trustPerm.TermManage,
	}
	if err := svc.Apply(ctx, ren, denyReq); err != nil {
		t.Fatalf("deny: %v", err)
	}
	if trustPerm.Resolver.Can([]string{"admin"}, trustPerm.TermManage) {
		t.Error("the deny did not reach the live trust resolver")
	}
	if !trustPerm.Resolver.Can([]string{"ren"}, trustPerm.TermManage) {
		t.Error("denying admin took the key off ren too")
	}

	m, err := svc.Matrix(ctx, ren)
	if err != nil {
		t.Fatalf("matrix: %v", err)
	}
	cell := findKey(t, m, string(trustPerm.TermManage)).Grants[permissions.RoleAdmin]
	if cell.Granted || cell.Source != permissions.SourceDeny || !cell.CanRestore || cell.Editable {
		t.Errorf("denied cell = %+v; want an ungranted deny cell offering restore only", cell)
	}

	restore := permissions.WriteRequest{
		Role: permissions.RoleAdmin, Permission: trustPerm.TermManage,
	}
	if err := svc.Apply(ctx, ren, restore); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !trustPerm.Resolver.Can([]string{"admin"}, trustPerm.TermManage) {
		t.Error("the restore did not return admin to its code floor")
	}

	entries, err := svc.Audit(ctx, 50)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit has %d rows, want 2: %+v", len(entries), entries)
	}
	if entries[0].Action != permissions.ActionRevokeDeny || entries[1].Action != permissions.ActionDeny {
		t.Errorf("audit actions = %q, %q; want revoke_deny, deny",
			entries[0].Action, entries[1].Action)
	}
}

func TestMatrixOffersDenyOnCodeFloorCells(t *testing.T) {
	reset(t)
	ctx := context.Background()
	reg := permissions.Live()
	dist := permissions.NewDistributor(testDB, reg, nil)
	t.Cleanup(func() {
		reset(t)
		if err := dist.Refresh(ctx); err != nil {
			t.Errorf("cleanup refresh: %v", err)
		}
	})
	svc := permissions.NewService(reg, permissions.NewStore(testDB), dist)

	m, err := svc.Matrix(ctx, permissions.Caller{UserID: 1, Roles: []string{"admin", "ren"}})
	if err != nil {
		t.Fatalf("matrix: %v", err)
	}
	cell := findKey(t, m, string(trustPerm.TermManage)).Grants[permissions.RoleAdmin]
	if !cell.Granted || cell.Source != permissions.SourceCode || !cell.CanDeny {
		t.Errorf("admin code cell = %+v; want a granted code cell ren may deny", cell)
	}
	if cell.Editable {
		t.Error("a code-floor cell has no grant row to add or remove")
	}

	am, err := svc.Matrix(ctx, permissions.Caller{UserID: 2, Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("matrix (admin): %v", err)
	}
	if c := findKey(t, am, string(trustPerm.TermManage)).Grants[permissions.RoleAdmin]; c.CanDeny {
		t.Errorf("an admin must not be able to deny its own tier: %+v", c)
	}
}

func findKey(t *testing.T, m *permissions.Matrix, key string) permissions.KeyRow {
	t.Helper()
	for _, d := range m.Domains {
		for _, k := range d.Keys {
			if k.Key == key {
				return k
			}
		}
	}
	t.Fatalf("key %q not in the matrix", key)
	return permissions.KeyRow{}
}
