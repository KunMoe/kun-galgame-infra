package jobs

import (
	"reflect"
	"testing"

	"api/internal/platform/catalog/imagerefs"
	"api/internal/platform/settings/keys"
)

// The whole point of the diff is that an already-known breakage must NOT
// re-alert: the 12 unrescuable references live in production would otherwise
// make this job permanently red, which is how an alert stops being read.
func TestNewlyBrokenOnlyReportsUnseenHashes(t *testing.T) {
	previous := map[string]struct{}{"aaa": {}, "bbb": {}}

	if got := newlyBroken([]string{"aaa", "bbb"}, previous); len(got) != 0 {
		t.Fatalf("known breakage must not alert, got %v", got)
	}
	if got := newlyBroken([]string{"aaa", "ccc"}, previous); !reflect.DeepEqual(got, []string{"ccc"}) {
		t.Fatalf("want [ccc], got %v", got)
	}
	if got := newlyBroken([]string{"aaa"}, previous); len(got) != 0 {
		t.Fatalf("shrinking set must not alert, got %v", got)
	}
	if got := newlyBroken([]string{"aaa"}, map[string]struct{}{}); !reflect.DeepEqual(got, []string{"aaa"}) {
		t.Fatalf("want [aaa], got %v", got)
	}
}

func TestDistinctHashesIsSortedAndDeduped(t *testing.T) {
	refs := []imagerefs.Ref{
		{Hash: "ccc", Kind: imagerefs.KindWorkCover, EntityID: 1},
		{Hash: "aaa", Kind: imagerefs.KindWorkScreenshot, EntityID: 2},
		{Hash: "ccc", Kind: imagerefs.KindWorkScreenshot, EntityID: 3},
		{Hash: "bbb", Kind: imagerefs.KindCharacterBust, EntityID: 4},
	}
	want := []string{"aaa", "bbb", "ccc"}
	for i := range 5 {
		if got := distinctHashes(refs); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: want %v, got %v", i, want, got)
		}
	}
}

func TestAffectedEntitiesCountsDistinctEntitiesPerKind(t *testing.T) {
	refs := []imagerefs.Ref{
		{Hash: "a", Kind: imagerefs.KindWorkCover, EntityID: 10},
		{Hash: "b", Kind: imagerefs.KindWorkCover, EntityID: 10},
		{Hash: "c", Kind: imagerefs.KindWorkCover, EntityID: 11},
		{Hash: "d", Kind: imagerefs.KindWorkScreenshot, EntityID: 10},
		{Hash: "e", Kind: imagerefs.KindCharacterBust, EntityID: 99},
	}
	want := map[string]int{"work_cover": 2, "work_screenshot": 1, imagerefs.KindCharacterBust: 1}
	if got := affectedEntities(refs); !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestImageRefAuditRunsAfterGCAndRefpings(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	at := map[string]string{}
	for _, j := range r.List() {
		jk, ok := keys.Job(j.Name)
		if !ok {
			t.Fatalf("%s has no settings keys", j.Name)
		}
		s, err := ParseSchedule(jk.Schedule.Default().(string))
		if err != nil {
			t.Fatalf("%s: %v", j.Name, err)
		}
		at[j.Name] = s.DailyAt
	}

	audit, ok := at[JobImageRefAudit]
	if !ok {
		t.Fatalf("%s is not registered", JobImageRefAudit)
	}
	for _, earlier := range []string{"image-gc", "galgame-image-refping", "catalog-image-refping", "user-avatar-refping"} {
		when, ok := at[earlier]
		if !ok {
			t.Fatalf("%s is not registered", earlier)
		}
		if when >= audit {
			t.Errorf("%s runs at %s, audit at %s — the audit must be last or it reads a mid-sweep state",
				earlier, when, audit)
		}
	}
}
