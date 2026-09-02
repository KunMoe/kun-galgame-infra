package jobs

import (
	"sort"
	"strings"
	"testing"
	"time"

	"api/internal/platform/settings/keys"
)

func TestScheduleZero(t *testing.T) {
	if !(Schedule{}).Zero() {
		t.Fatal("empty schedule should be Zero")
	}
	if (Schedule{DailyAt: "03:00"}).Zero() {
		t.Fatal("DailyAt schedule should not be Zero")
	}
	if (Schedule{Every: time.Hour}).Zero() {
		t.Fatal("Every schedule should not be Zero")
	}
}

func TestScheduleNextDailyAt(t *testing.T) {
	loc := time.UTC
	s := Schedule{DailyAt: "03:00"}

	now := time.Date(2026, 5, 16, 1, 0, 0, 0, loc)
	got := s.Next(now)
	want := time.Date(2026, 5, 16, 3, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("before fire: got %v want %v", got, want)
	}

	now = time.Date(2026, 5, 16, 3, 0, 0, 0, loc)
	got = s.Next(now)
	want = time.Date(2026, 5, 17, 3, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("at fire: got %v want %v", got, want)
	}

	now = time.Date(2026, 5, 16, 9, 30, 0, 0, loc)
	got = s.Next(now)
	want = time.Date(2026, 5, 17, 3, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("after fire: got %v want %v", got, want)
	}
}

func TestScheduleNextEveryAndBad(t *testing.T) {
	now := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)

	if got := (Schedule{Every: 2 * time.Hour}).Next(now); !got.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("Every: got %v", got)
	}
	if got := (Schedule{}).Next(now); !got.IsZero() {
		t.Fatalf("zero schedule Next should be zero, got %v", got)
	}
	if got := (Schedule{DailyAt: "nonsense"}).Next(now); !got.IsZero() {
		t.Fatalf("bad DailyAt Next should be zero, got %v", got)
	}
}

func TestParseScheduleRoundTrip(t *testing.T) {
	for _, in := range []string{
		"daily@03:30", "daily@00:00", "daily@23:59",
		"every:1m", "every:10m", "every:1h", "every:36h",
	} {
		s, err := ParseSchedule(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := s.String(); got != in {
			t.Errorf("%q: String() = %q", in, got)
		}
	}

	s, err := ParseSchedule("every:60m")
	if err != nil {
		t.Fatalf("every:60m: %v", err)
	}
	if got := s.String(); got != "every:1h" {
		t.Errorf("every:60m String() = %q, want %q (60 minutes is a whole number of hours)", got, "every:1h")
	}

	for _, in := range []string{"", "daily@24:00", "daily@3:30", "every:0m", "every:90s", "weekly", "manual"} {
		_, err := ParseSchedule(in)
		if err == nil {
			t.Errorf("%q: want error", in)
			continue
		}
		if in != "" && !strings.Contains(err.Error(), in) {
			t.Errorf("%q: error %q does not name the input", in, err)
		}
	}
}

// TestYmgalJobsAreActuallyScheduled guards the quiet failure: a poll whose
// jobs.<name>.schedule default lost its every: cadence would sit idle until
// the string changed and nothing would report an error at boot.
// ymgal-news-poll is the first job in the registry to rely on an every: cadence.
func TestYmgalJobsAreActuallyScheduled(t *testing.T) {
	for name, want := range map[string]time.Duration{
		"ymgal-news-poll": 10 * time.Minute,
		"news-moderate":   5 * time.Minute,
	} {
		jk, ok := keys.Job(name)
		if !ok {
			t.Fatalf("%s has no settings keys", name)
		}
		s, err := ParseSchedule(jk.Schedule.Default().(string))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if s.Every != want {
			t.Errorf("%s Every = %v, want %v", name, s.Every, want)
		}
	}

	jk, ok := keys.Job("ymgal-news-sweep")
	if !ok {
		t.Fatal("ymgal-news-sweep has no settings keys")
	}
	s, err := ParseSchedule(jk.Schedule.Default().(string))
	if err != nil {
		t.Fatalf("ymgal-news-sweep: %v", err)
	}
	if s.DailyAt == "" {
		t.Error("ymgal-news-sweep must keep a daily schedule: the poll only reads page 1, so the sweep is the only thing that can notice an upstream deletion")
	}
}

func TestEveryRegisteredJobHasKeys(t *testing.T) {
	reg := NewRegistry()
	RegisterAll(reg)

	got := make([]string, 0, len(reg.List()))
	for _, j := range reg.List() {
		got = append(got, j.Name)
	}
	want := append([]string(nil), keys.JobNames()...)
	sort.Strings(got)
	sort.Strings(want)

	gotSet := make(map[string]bool, len(got))
	for _, n := range got {
		gotSet[n] = true
	}
	wantSet := make(map[string]bool, len(want))
	for _, n := range want {
		wantSet[n] = true
	}
	for _, n := range got {
		if !wantSet[n] {
			t.Errorf("registered job %q is missing from keys.JobNames()", n)
		}
	}
	for _, n := range want {
		if !gotSet[n] {
			t.Errorf("keys.JobNames() has %q with no registered job", n)
		}
	}
}

func TestRegisterPanicsWithoutKeys(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal(`Register(Job{Name: "no-such-job"}) did not panic`)
		}
	}()
	NewRegistry().Register(Job{Name: "no-such-job"})
}

func TestRegistryListSortedAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(Job{Name: "store-stats-sync"})
	r.Register(Job{Name: "image-gc"})
	r.Register(Job{Name: "galgame-image-refping"})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("want 3 jobs, got %d", len(list))
	}
	if list[0].Name != "galgame-image-refping" || list[2].Name != "store-stats-sync" {
		t.Fatalf("not name-sorted: %v", []string{list[0].Name, list[1].Name, list[2].Name})
	}
	if _, ok := r.Get("image-gc"); !ok {
		t.Fatal("Get(image-gc) missing")
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("Get(nope) should be absent")
	}
}

func TestAdvisoryKeyStableAndDistinct(t *testing.T) {
	a, b := advisoryKey("sync-vndb"), advisoryKey("sync-vndb")
	if a != b {
		t.Fatal("advisoryKey not stable")
	}
	if advisoryKey("sync-vndb") == advisoryKey("image-gc") {
		t.Fatal("advisoryKey collision on distinct names")
	}
}
