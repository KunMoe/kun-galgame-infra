package devapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	siteModel "api/internal/platform/site/model"

	"github.com/gofiber/fiber/v3"
	"gorm.io/datatypes"
)

func approvalMode(t *testing.T, admin *AdminService) {
	t.Helper()
	if err := admin.SetPolicy(context.Background(), CapabilityAppCreate, PolicyApproval, 7); err != nil {
		t.Fatalf("set app.create=approval: %v", err)
	}
}

func TestAppApprovalFlow(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, repo, _ := newSelfService(t)
	ctx := context.Background()
	const owner, reviewer = uint(1), uint(9)

	selfServed, err := svc.CreateApp(ctx, owner, "before the switch", "", nil)
	if err != nil {
		t.Fatalf("create under self_service: %v", err)
	}
	if !selfServed.DevEnabled || selfServed.DevReviewStatus != AppReviewApproved {
		t.Fatalf("self-service create = enabled %v / %q, want enabled + approved",
			selfServed.DevEnabled, selfServed.DevReviewStatus)
	}

	approvalMode(t, admin)

	filed, err := svc.CreateApp(ctx, owner, "awaiting review", "", nil)
	if err != nil {
		t.Fatalf("create under approval: %v", err)
	}
	if filed.DevEnabled || filed.DevReviewStatus != AppReviewPending {
		t.Fatalf("approval create = enabled %v / %q, want disabled + pending",
			filed.DevEnabled, filed.DevReviewStatus)
	}

	if _, _, err := svc.MintKey(ctx, owner, filed.ID, MintKeyInput{Name: "too early"}); err != ErrAppNotApproved {
		t.Errorf("mint on a pending app = %v, want ErrAppNotApproved", err)
	}
	if _, err := svc.ResubmitApp(ctx, owner, filed.ID); err != ErrAppNotDeclined {
		t.Errorf("resubmit a pending app = %v, want ErrAppNotDeclined", err)
	}

	// A pending row is still the owner's app: it has to show up in the portal
	// list, or the applicant sees nothing at all after filing.
	mine, err := svc.ListApps(ctx, owner)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("owner list = %d apps, want both the approved and the pending one", len(mine))
	}

	if _, err := admin.DeclineApp(ctx, filed.ID, reviewer, "   "); err != ErrAppReviewNeedsReason {
		t.Errorf("decline with no reason = %v, want ErrAppReviewNeedsReason", err)
	}
	if _, err := admin.DeclineApp(ctx, filed.ID, reviewer,
		strings.Repeat("说", maxAppReviewNoteLen+1)); err != ErrAppReviewNoteTooLong {
		t.Errorf("over-long decline reason = %v, want ErrAppReviewNoteTooLong", err)
	}
	// The cap counts runes: a reason of exactly the limit in Chinese is longer
	// than the limit in bytes and still has to be accepted.
	full := strings.Repeat("说", maxAppReviewNoteLen)
	if len(full) <= maxAppReviewNoteLen {
		t.Fatalf("fixture is not multi-byte: %d bytes for %d runes", len(full), maxAppReviewNoteLen)
	}
	declined, err := admin.DeclineApp(ctx, filed.ID, reviewer, full)
	if err != nil {
		t.Fatalf("decline with %d Chinese characters (%d bytes): %v", maxAppReviewNoteLen, len(full), err)
	}
	if declined.DevReviewStatus != AppReviewDeclined || declined.DevEnabled || declined.DevReviewNote != full {
		t.Fatalf("declined row = status %q / enabled %v / note %d runes, want declined + disabled + the reason",
			declined.DevReviewStatus, declined.DevEnabled, len([]rune(declined.DevReviewNote)))
	}

	if _, _, err := svc.MintKey(ctx, owner, filed.ID, MintKeyInput{Name: "declined"}); err != ErrAppNotApproved {
		t.Errorf("mint on a declined app = %v, want ErrAppNotApproved", err)
	}
	if _, err := admin.ApproveApp(ctx, filed.ID, reviewer); err != ErrAppReviewNotPending {
		t.Errorf("approve a declined app = %v, want ErrAppReviewNotPending", err)
	}

	// The owner may fix the name first; UpdateApp keeps working on a declined row.
	fixed := "awaiting review, take two"
	if _, err := svc.UpdateApp(ctx, owner, filed.ID, &fixed, nil, nil); err != nil {
		t.Fatalf("edit a declined app: %v", err)
	}
	resubmitted, err := svc.ResubmitApp(ctx, owner, filed.ID)
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if resubmitted.DevReviewStatus != AppReviewPending || resubmitted.DevReviewNote != "" {
		t.Fatalf("resubmitted row = status %q / note %q, want pending with the note cleared",
			resubmitted.DevReviewStatus, resubmitted.DevReviewNote)
	}
	if resubmitted.Name != fixed {
		t.Errorf("resubmitted name = %q, want the edit to survive", resubmitted.Name)
	}

	approved, err := admin.ApproveApp(ctx, filed.ID, reviewer)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !approved.DevEnabled || approved.DevReviewStatus != AppReviewApproved || approved.DevReviewNote != "" {
		t.Fatalf("approved row = enabled %v / %q / note %q, want enabled + approved + no note",
			approved.DevEnabled, approved.DevReviewStatus, approved.DevReviewNote)
	}
	if _, _, err := svc.MintKey(ctx, owner, filed.ID, MintKeyInput{Name: "at last"}); err != nil {
		t.Fatalf("mint after approval: %v", err)
	}

	// Pending and declined rows count against the 5-app cap, or a declined
	// applicant could file forever. Archiving is what releases a slot —
	// see TestArchiveReleasesTheAppSlot.
	n, err := repo.CountAppsByOwner(ctx, owner)
	if err != nil {
		t.Fatalf("count apps: %v", err)
	}
	if n != 2 {
		t.Errorf("owner app count = %d, want 2", n)
	}
}

func TestAppReviewLegacyBlankStatusMints(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, _, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	// What the OAuth console writes: a client row that never heard of the
	// review columns. It must keep minting exactly as it did before the flow
	// existed — the gate is on pending/declined, not on "not approved".
	owned := owner
	legacy := &siteModel.OAuthClient{
		ID:              "devapitest_blankstatus",
		Name:            "legacy",
		Secret:          siteModel.HashOAuthClientSecret("s"),
		RedirectURIs:    datatypes.JSON([]byte("[]")),
		Grants:          datatypes.JSON([]byte("[]")),
		OwnerUserID:     &owned,
		DevEnabled:      true,
		DevTier:         TierFree,
		DevReviewStatus: "",
	}
	if err := testDB.Create(legacy).Error; err != nil {
		t.Fatalf("insert legacy client: %v", err)
	}
	if _, _, err := svc.MintKey(ctx, owner, legacy.ID, MintKeyInput{Name: "legacy key"}); err != nil {
		t.Fatalf("mint on a blank-status app = %v, want it to go through", err)
	}
	if err := svc.ArchiveApp(ctx, owner, legacy.ID); err != nil {
		t.Fatalf("archive a blank-status app = %v, want it to go through", err)
	}
}

func TestAdminEnableAlsoApproves(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	approvalMode(t, admin)
	filed, err := svc.CreateApp(ctx, owner, "enabled from the console", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	enabled := true
	updated, err := admin.UpdateAppConfig(ctx, filed.ID, AppConfig{DevEnabled: &enabled})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.DevReviewStatus != AppReviewApproved {
		t.Fatalf("status after the console enable = %q, want approved", updated.DevReviewStatus)
	}
	if _, _, err := svc.MintKey(ctx, owner, filed.ID, MintKeyInput{Name: "k"}); err != nil {
		t.Fatalf("mint on a console-enabled app: %v", err)
	}
}

func TestAdminListAppsByStatus(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, repo, _ := newSelfService(t)
	ctx := context.Background()
	const owner, reviewer = uint(1), uint(9)

	live, err := svc.CreateApp(ctx, owner, "live", "", nil)
	if err != nil {
		t.Fatalf("create live: %v", err)
	}
	disabled := false
	if _, err := admin.UpdateAppConfig(ctx, live.ID, AppConfig{DevEnabled: &disabled}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	kept, _ := svc.CreateApp(ctx, owner, "kept", "", nil)
	gone, _ := svc.CreateApp(ctx, owner, "gone", "", nil)
	// ArchiveApp deletes a row with no history outright, and a row that no
	// longer exists cannot be in the archived bucket. A minted key is not
	// enough — it gets deleted too — so the key has to have SERVED something.
	goneKey, _, err := svc.MintKey(ctx, owner, gone.ID, MintKeyInput{Name: "k"})
	if err != nil {
		t.Fatalf("mint on the app to archive: %v", err)
	}
	giveKeyHistory(t, repo, goneKey.ID)
	if err := svc.ArchiveApp(ctx, owner, gone.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	approvalMode(t, admin)
	pending, _ := svc.CreateApp(ctx, owner, "pending", "", nil)
	rejected, _ := svc.CreateApp(ctx, owner, "rejected", "", nil)
	if _, err := admin.DeclineApp(ctx, rejected.ID, reviewer, "no"); err != nil {
		t.Fatalf("decline: %v", err)
	}

	// Asserting on the row count would make this test depend on the whole
	// oauth_clients table, which is shared: the ai package's s2s fixtures
	// (ai-test-bound / ai-test-unbound, owner NULL, ids cleanupSelf cannot
	// match) run earlier in the suite and land in the disabled and all buckets.
	// So compare only the four rows this test made.
	labels := map[string]string{
		live.ID: "live", kept.ID: "kept", pending.ID: "pending",
		rejected.ID: "rejected", gone.ID: "gone",
	}
	mine := func(t *testing.T, filter string) map[string]bool {
		t.Helper()
		views, err := admin.ListApps(ctx, filter)
		if err != nil {
			t.Fatalf("list %q: %v", filter, err)
		}
		out := map[string]bool{}
		for i := range views {
			if label, ok := labels[views[i].Client.ID]; ok {
				out[label] = true
			}
		}
		return out
	}

	cases := []struct {
		filter string
		want   []string
	}{
		{AppFilterEnabled, []string{"kept"}},
		{AppFilterPending, []string{"pending"}},
		{AppFilterDeclined, []string{"rejected"}},
		{AppFilterDisabled, []string{"live"}},
		{AppFilterArchived, []string{"gone"}},
		{AppFilterAll, []string{"live", "kept", "pending", "rejected", "gone"}},
	}
	for _, tc := range cases {
		got := mine(t, tc.filter)
		want := map[string]bool{}
		for _, label := range tc.want {
			want[label] = true
		}
		for label := range want {
			if !got[label] {
				t.Errorf("status=%s is missing %q", tc.filter, label)
			}
		}
		for label := range got {
			if !want[label] {
				t.Errorf("status=%s wrongly includes %q", tc.filter, label)
			}
		}
	}
}

func TestAdminListAllKeys(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, repo, _ := newSelfService(t)
	ctx := context.Background()
	const ownerA, ownerB = uint(1), uint(2)

	appA, _ := svc.CreateApp(ctx, ownerA, "app-a", "", nil)
	appB, _ := svc.CreateApp(ctx, ownerB, "app-b", "", nil)

	var aKeys []uint
	for i := 0; i < 3; i++ {
		k, _, err := svc.MintKey(ctx, ownerA, appA.ID, MintKeyInput{Name: fmt.Sprintf("a%d", i)})
		if err != nil {
			t.Fatalf("mint a%d: %v", i, err)
		}
		aKeys = append(aKeys, k.ID)
	}
	bKey, _, err := svc.MintKey(ctx, ownerB, appB.ID, MintKeyInput{Name: "b0"})
	if err != nil {
		t.Fatalf("mint b0: %v", err)
	}
	if _, err := svc.RevokeKey(ctx, ownerA, appA.ID, aKeys[0]); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := repo.SetKeyExpiry(ctx, aKeys[1], time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("expire: %v", err)
	}

	count := func(t *testing.T, f KeyListFilter) ([]AdminKeyRow, int64) {
		t.Helper()
		if f.Page == 0 {
			f.Page, f.Limit = 1, 50
		}
		rows, total, err := admin.ListAllKeys(ctx, f)
		if err != nil {
			t.Fatalf("list keys %+v: %v", f, err)
		}
		return rows, total
	}

	rows, total := count(t, KeyListFilter{State: KeyStateAll})
	if total != 4 || len(rows) != 4 {
		t.Fatalf("all keys = %d rows / total %d, want 4/4", len(rows), total)
	}
	for _, r := range rows {
		if r.AppName == "" || r.OwnerUserID == nil {
			t.Errorf("row %d carries no app identity: %+v", r.ID, r)
		}
		// The join selects k.* into an embedded struct; if GORM stopped
		// flattening it the rows would come back with the joined columns set
		// and every key column zeroed, which the identity check above misses.
		if r.ID == 0 || r.ClientID == "" || r.KeyPrefix == "" || r.Last4 == "" ||
			len(r.Scopes) == 0 || r.CreatedAt.IsZero() {
			t.Errorf("row %+v lost its key columns", r)
		}
		if state := r.DeveloperAPIKey.State(time.Now()); state == "" {
			t.Errorf("row %d has no computed state", r.ID)
		}
	}

	if _, total := count(t, KeyListFilter{State: KeyStateActive}); total != 2 {
		t.Errorf("active total = %d, want 2", total)
	}
	if _, total := count(t, KeyListFilter{State: KeyStateRevoked}); total != 1 {
		t.Errorf("revoked total = %d, want 1", total)
	}
	if _, total := count(t, KeyListFilter{State: KeyStateExpired}); total != 1 {
		t.Errorf("expired total = %d, want 1", total)
	}
	if rows, total := count(t, KeyListFilter{ClientID: appB.ID, State: KeyStateAll}); total != 1 ||
		len(rows) != 1 || rows[0].ID != bKey.ID {
		t.Errorf("client filter = %d rows / total %d, want only b0", len(rows), total)
	}

	// Pagination must report the unpaged total, or the console cannot draw the pager.
	page1, total := count(t, KeyListFilter{State: KeyStateAll, Page: 1, Limit: 3})
	page2, _ := count(t, KeyListFilter{State: KeyStateAll, Page: 2, Limit: 3})
	if total != 4 || len(page1) != 3 || len(page2) != 1 {
		t.Fatalf("paging = %d + %d rows of total %d, want 3 + 1 of 4", len(page1), len(page2), total)
	}
	if page1[0].ID == page2[0].ID {
		t.Error("page 2 repeats a row from page 1")
	}
}

func TestSelfServiceReviewHTTPStatuses(t *testing.T) {
	cleanupSelf(t)
	cleanupPolicies(t)
	svc, admin, _, _ := newSelfService(t)
	ctx := context.Background()
	const owner = uint(1)

	approvalMode(t, admin)
	filed, err := svc.CreateApp(ctx, owner, "http states", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	fiberApp := fiber.New()
	NewSelfServiceHandler(svc).Register(fiberApp.Group("/dev", fakeAuth(owner)))

	do := func(t *testing.T, method, path, body string) (int, []byte) {
		t.Helper()
		var r io.Reader
		if body != "" {
			r = bytes.NewBufferString(body)
		}
		req := httptest.NewRequest(method, path, r)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := fiberApp.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode, raw
	}

	base := "/dev/apps/" + filed.ID
	if code, _ := do(t, "POST", base+"/keys", `{"name":"k"}`); code != fiber.StatusConflict {
		t.Errorf("mint on a pending app = %d, want 409", code)
	}
	if code, _ := do(t, "POST", base+"/resubmit", ""); code != fiber.StatusConflict {
		t.Errorf("resubmit a pending app = %d, want 409", code)
	}

	if _, err := admin.DeclineApp(ctx, filed.ID, 9, "not this time"); err != nil {
		t.Fatalf("decline: %v", err)
	}
	code, raw := do(t, "POST", base+"/resubmit", "")
	if code != fiber.StatusOK {
		t.Fatalf("resubmit a declined app = %d (%s), want 200", code, raw)
	}
	var resubmitted struct {
		Data struct {
			ReviewStatus string `json:"review_status"`
			ReviewNote   string `json:"review_note"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resubmitted); err != nil {
		t.Fatalf("decode resubmit: %v", err)
	}
	if resubmitted.Data.ReviewStatus != AppReviewPending || resubmitted.Data.ReviewNote != "" {
		t.Errorf("resubmit response = %+v, want pending with no note", resubmitted.Data)
	}

	if err := admin.SetPolicy(ctx, CapabilityAppCreate, PolicyDisabled, 7); err != nil {
		t.Fatalf("disable app.create: %v", err)
	}
	if code, _ := do(t, "POST", "/dev/apps", `{"name":"blocked"}`); code != fiber.StatusForbidden {
		t.Errorf("create under app.create=disabled = %d, want 403", code)
	}

	code, raw = do(t, "GET", "/dev/policies", "")
	if code != fiber.StatusOK {
		t.Fatalf("GET /dev/policies = %d, want 200", code)
	}
	var policies struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &policies); err != nil {
		t.Fatalf("decode policies: %v", err)
	}
	if policies.Data[CapabilityAppCreate] != PolicyDisabled ||
		policies.Data[CapabilityKeyMint] != PolicySelfService ||
		len(policies.Data) != len(capabilities) {
		t.Errorf("policies face = %+v, want every capability with app.create disabled", policies.Data)
	}
}
