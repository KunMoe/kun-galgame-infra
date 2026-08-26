package devapi

import (
	"context"
	"slices"
	"testing"
)

func cleanupPolicies(t *testing.T) {
	t.Helper()
	if err := testDB.Exec(`TRUNCATE devapi_policy_overrides RESTART IDENTITY`).Error; err != nil {
		t.Fatalf("truncate policy overrides: %v", err)
	}
}

func TestPolicyRegistryIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range capabilities {
		if seen[c.Key] {
			t.Errorf("duplicate capability %q", c.Key)
		}
		seen[c.Key] = true
		if c.LabelZH == "" || c.LabelEN == "" || c.DescZH == "" || c.DescEN == "" {
			t.Errorf("%s: every capability needs a zh and en label and description", c.Key)
		}
		if len(c.Modes) < 2 {
			t.Errorf("%s: a capability with fewer than two modes is not a policy", c.Key)
		}
		if !slices.Contains(c.Modes, c.Default) {
			t.Errorf("%s: default %q is not one of its modes %v", c.Key, c.Default, c.Modes)
		}
		if c.Default != PolicySelfService {
			t.Errorf("%s: default is %q — the platform ships open, policy narrows it", c.Key, c.Default)
		}
	}

	want := []string{CapabilityAppCreate, CapabilityAppManage, CapabilityKeyMint}
	for _, key := range want {
		if !seen[key] {
			t.Errorf("capability %q is missing from the matrix", key)
		}
	}
	if len(seen) != len(want) {
		t.Errorf("matrix holds %d capabilities, want exactly %v", len(seen), want)
	}

	// Which scopes a key may carry stays in selfServiceScopes with its own
	// tests. If one ever appears here too, one decision has two sources of truth.
	for _, sc := range selfServiceScopes {
		if seen[sc] {
			t.Errorf("scope %q must not be a policy capability", sc)
		}
	}
}

func TestPolicyValidation(t *testing.T) {
	if err := validatePolicy("app.nonsense", PolicySelfService); err != ErrUnknownCapability {
		t.Errorf("unknown capability = %v, want ErrUnknownCapability", err)
	}
	// approval is meaningful only for app.create: there is nothing to queue for
	// review when an owner edits an app or mints a key.
	for _, capability := range []string{CapabilityAppManage, CapabilityKeyMint} {
		if err := validatePolicy(capability, PolicyApproval); err != ErrInvalidPolicyMode {
			t.Errorf("%s=approval = %v, want ErrInvalidPolicyMode", capability, err)
		}
	}
	if err := validatePolicy(CapabilityAppCreate, "banana"); err != ErrInvalidPolicyMode {
		t.Errorf("unknown mode = %v, want ErrInvalidPolicyMode", err)
	}
	for _, mode := range []string{PolicySelfService, PolicyApproval, PolicyDisabled} {
		if err := validatePolicy(CapabilityAppCreate, mode); err != nil {
			t.Errorf("app.create=%s = %v, want accepted", mode, err)
		}
	}
}

func TestPolicyOverrideTakesEffectAndResets(t *testing.T) {
	cleanupPolicies(t)
	svc, admin, repo, _ := newSelfService(t)
	ctx := context.Background()
	const actor = uint(7)

	modes, err := svc.EffectivePolicies(ctx)
	if err != nil {
		t.Fatalf("effective policies: %v", err)
	}
	if len(modes) != len(capabilities) {
		t.Fatalf("effective policies = %d entries, want %d", len(modes), len(capabilities))
	}
	for _, c := range capabilities {
		if modes[c.Key] != c.Default {
			t.Errorf("%s = %q with no override, want the code default %q", c.Key, modes[c.Key], c.Default)
		}
	}

	if err := admin.SetPolicy(ctx, CapabilityAppCreate, PolicyApproval, actor); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	if mode, _ := repo.PolicyMode(ctx, CapabilityAppCreate); mode != PolicyApproval {
		t.Errorf("app.create = %q after the override, want approval", mode)
	}

	// Writing the same capability twice must move the one row, not stack a second.
	if err := admin.SetPolicy(ctx, CapabilityAppCreate, PolicyDisabled, actor+1); err != nil {
		t.Fatalf("re-set policy: %v", err)
	}
	rows, err := repo.PolicyOverrides(ctx)
	if err != nil {
		t.Fatalf("list overrides: %v", err)
	}
	if len(rows) != 1 || rows[0].Mode != PolicyDisabled || rows[0].SetByUserID != actor+1 {
		t.Fatalf("overrides = %+v, want one disabled row set by %d", rows, actor+1)
	}

	views, err := admin.Policies(ctx)
	if err != nil {
		t.Fatalf("admin policies: %v", err)
	}
	for _, v := range views {
		if v.Key != CapabilityAppCreate {
			if v.Overridden || v.Mode != v.Default {
				t.Errorf("%s = %+v, want the untouched default", v.Key, v)
			}
			continue
		}
		if !v.Overridden || v.Mode != PolicyDisabled || v.Default != PolicySelfService || v.UpdatedAt == "" {
			t.Errorf("app.create view = %+v, want an overridden disabled row keeping its default", v)
		}
	}

	if err := admin.ResetPolicy(ctx, CapabilityAppCreate, actor); err != nil {
		t.Fatalf("reset policy: %v", err)
	}
	if mode, _ := repo.PolicyMode(ctx, CapabilityAppCreate); mode != PolicySelfService {
		t.Errorf("app.create = %q after the reset, want the code default", mode)
	}
	if rows, _ := repo.PolicyOverrides(ctx); len(rows) != 0 {
		t.Errorf("overrides after the reset = %+v, want none", rows)
	}

	if err := admin.SetPolicy(ctx, "app.nonsense", PolicySelfService, actor); err != ErrUnknownCapability {
		t.Errorf("set unknown capability = %v, want ErrUnknownCapability", err)
	}
	if err := admin.ResetPolicy(ctx, "app.nonsense", actor); err != ErrUnknownCapability {
		t.Errorf("reset unknown capability = %v, want ErrUnknownCapability", err)
	}
}

func TestPolicyDisabledGatesSelfService(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner, actor = uint(1), uint(7)

	app, err := svc.CreateApp(ctx, owner, "gated", "", nil)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	key, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "k"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	if err := admin.SetPolicy(ctx, CapabilityKeyMint, PolicyDisabled, actor); err != nil {
		t.Fatalf("disable key.mint: %v", err)
	}
	if _, _, err := svc.MintKey(ctx, owner, app.ID, MintKeyInput{Name: "blocked"}); err != ErrCapabilityDisabled {
		t.Errorf("mint under key.mint=disabled = %v, want ErrCapabilityDisabled", err)
	}
	if _, _, err := svc.RotateKey(ctx, owner, app.ID, key.ID); err != ErrCapabilityDisabled {
		t.Errorf("rotate under key.mint=disabled = %v, want ErrCapabilityDisabled", err)
	}
	// Revocation is the stop-loss action; no policy may take it away.
	found, err := svc.RevokeKey(ctx, owner, app.ID, key.ID)
	if err != nil || !found {
		t.Errorf("revoke under key.mint=disabled = (%v, %v), want it to go through", found, err)
	}

	if err := admin.SetPolicy(ctx, CapabilityAppManage, PolicyDisabled, actor); err != nil {
		t.Fatalf("disable app.manage: %v", err)
	}
	name := "renamed"
	if _, err := svc.UpdateApp(ctx, owner, app.ID, &name, nil, nil); err != ErrCapabilityDisabled {
		t.Errorf("update under app.manage=disabled = %v, want ErrCapabilityDisabled", err)
	}
	if err := svc.DeactivateApp(ctx, owner, app.ID); err != ErrCapabilityDisabled {
		t.Errorf("deactivate under app.manage=disabled = %v, want ErrCapabilityDisabled", err)
	}

	if err := admin.SetPolicy(ctx, CapabilityAppCreate, PolicyDisabled, actor); err != nil {
		t.Fatalf("disable app.create: %v", err)
	}
	if _, err := svc.CreateApp(ctx, owner, "nope", "", nil); err != ErrCapabilityDisabled {
		t.Errorf("create under app.create=disabled = %v, want ErrCapabilityDisabled", err)
	}

	// A non-owner still gets the ownership 404 rather than the policy error:
	// the policy must not become an existence oracle for someone else's app.
	if _, _, err := svc.MintKey(ctx, uint(2), app.ID, MintKeyInput{Name: "stranger"}); err == ErrCapabilityDisabled {
		t.Error("a non-owner must be refused as not-found, not told about the policy")
	}
}
