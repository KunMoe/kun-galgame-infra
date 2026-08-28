package editing_test

import (
	"errors"
	"testing"

	"api/internal/platform/editing"
)

const strictReview = "edit.test.owned.strict_review"

// Revert used to ask the overlay of the caller's OWN site, so a tenant whose
// rules are looser than the owner's reviewed the owner's fields under its own.
// AmendProposal and MergeProposal have always asked prop.Site.
func revertEngine(t *testing.T, owners map[int64]*string, overlaySite string) *editing.Engine {
	t.Helper()
	cleanTables(t)
	createWidget(t, 1)
	spec := ownedSpec(owners, nil)
	spec.DefaultPolicy = editing.Policy{
		Propose: editing.ProposeOpen, Review: editing.ReviewPerm(permReview), Automerge: editing.AutomergeNever,
	}
	if overlaySite != "" {
		spec.SiteOverlays = map[string]map[string]editing.Policy{
			overlaySite: {"test.owned.name": {
				Propose: editing.ProposeOpen, Review: editing.ReviewPerm(strictReview), Automerge: editing.AutomergeNever,
			}},
		}
	}
	reg := editing.NewRegistry()
	if err := reg.Register(spec); err != nil {
		t.Fatalf("register owned spec: %v", err)
	}
	return editing.NewEngine(testDB, reg)
}

func revertHistory(t *testing.T, eng *editing.Engine, actor editing.PolicyContext) {
	t.Helper()
	for _, name := range []string{"first", "second"} {
		prop, _, err := eng.CreateProposal(testCtx, editing.CreateProposalInput{
			EntityType: "test.owned", EntityID: 1,
			Patch: map[string]any{"test.owned.name": name}, Actor: actor,
		})
		if err != nil {
			t.Fatalf("create proposal %q: %v", name, err)
		}
		if _, err := eng.MergeProposal(testCtx, prop.ID, actor, ""); err != nil {
			t.Fatalf("merge proposal %q: %v", name, err)
		}
	}
}

func siteActor(uid int64, site string, perms ...string) editing.PolicyContext {
	a := actorWith(uid, 0, perms...)
	a.Site = site
	return a
}

func TestRevertResolvesPolicyUnderTheOwnerSite(t *testing.T) {
	const ownerSite = "owner-site"
	site := ownerSite
	eng := revertEngine(t, map[int64]*string{1: &site}, ownerSite)
	revertHistory(t, eng, siteActor(1, ownerSite, strictReview))

	_, _, err := eng.Revert(testCtx, editing.RevertInput{
		EntityType: "test.owned", EntityID: 1, ToSeq: 1,
		Actor: siteActor(2, "some-other-site", permReview),
	})
	var perm *editing.PermissionError
	if !errors.As(err, &perm) {
		t.Fatalf("the owner's overlay names a review permission this caller does not hold; got %v", err)
	}
	if got := loadWidget(t, 1).Name; got != "second" {
		t.Fatalf("the refused revert must not have written: %q", got)
	}

	if _, _, err := eng.Revert(testCtx, editing.RevertInput{
		EntityType: "test.owned", EntityID: 1, ToSeq: 1,
		Actor: siteActor(3, "some-other-site", strictReview),
	}); err != nil {
		t.Fatalf("positive control: the owner overlay's own permission still reverts: %v", err)
	}
	if got := loadWidget(t, 1).Name; got != "first" {
		t.Fatalf("positive control: want first, got %q", got)
	}
}

// An entity nobody claims has no owner overlay to resolve under, so the
// caller's site stays the answer and the default policy applies.
func TestRevertWithoutAnOwnerKeepsTheCallerSite(t *testing.T) {
	eng := revertEngine(t, map[int64]*string{}, "")
	actor := siteActor(1, "some-other-site", permReview)
	revertHistory(t, eng, actor)

	if _, _, err := eng.Revert(testCtx, editing.RevertInput{
		EntityType: "test.owned", EntityID: 1, ToSeq: 1, Actor: actor,
	}); err != nil {
		t.Fatalf("unowned entity falls back to the default policy: %v", err)
	}
	if got := loadWidget(t, 1).Name; got != "first" {
		t.Fatalf("want first, got %q", got)
	}
}
