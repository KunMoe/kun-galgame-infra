package dbtest

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// RequireSearchEnv is REQUIRE_DB_TESTS for Meilisearch. It exists because the
// works-search suite skipped in full while catalog/service still printed `ok`,
// and the W4 wave shipped a filter that returned zero hits for every query.
const RequireSearchEnv = "REQUIRE_SEARCH_TESTS"

func SearchHost() (host, apiKey string) {
	return os.Getenv("MEILISEARCH_TEST_HOST"), os.Getenv("MEILISEARCH_TEST_API_KEY")
}

// SearchIndexPrefix names indexes for one test binary and no other. The two
// search suites used the fixed prefixes "test_" and "test_svc_" against one
// local Meilisearch, so two sessions running the suite at once shared every
// index — and each test's cleanup DeleteIndex took the other run's corpus out
// from under it mid-assertion. Pair it with SweepSearchIndexes: unique names
// with no sweep only trades the collision for a pile of dead indexes.
func SearchIndexPrefix(suite string) string {
	return fmt.Sprintf("test_%s_%d_%d_", suite, os.Getpid(), time.Now().UnixNano()%1_000_000)
}

func SweepSearchIndexes(svc meilisearch.ServiceManager, prefix string) {
	if svc == nil || prefix == "" {
		return
	}
	list, err := svc.ListIndexes(&meilisearch.IndexesQuery{Limit: 1000})
	if err != nil || list == nil {
		return
	}
	// DeleteIndex only enqueues; the process exited before the tasks ran and a
	// measured sweep left two of its own five indexes behind.
	for _, idx := range list.Results {
		if idx == nil || !strings.HasPrefix(idx.UID, prefix) {
			continue
		}
		task, err := svc.DeleteIndex(idx.UID)
		if err != nil || task == nil {
			continue
		}
		_, _ = svc.WaitForTask(task.TaskUID, 20*time.Millisecond)
	}
}

// SkipSearch skips a search-backed test, or fails it when the run was supposed
// to reach Meilisearch. Every reason a search test declines to run — no host,
// an unusable client, an unreachable server — must come through here.
func SkipSearch(t *testing.T, format string, args ...any) {
	t.Helper()
	reason := fmt.Sprintf(format, args...)
	if os.Getenv(RequireSearchEnv) != "" {
		t.Fatalf("%s is set but Meilisearch is unusable: %s", RequireSearchEnv, reason)
	}
	t.Skipf("%s — search-backed test not run (set %s=1 to make this a failure)", reason, RequireSearchEnv)
}

// SkipSearchMain is SkipSearch for a TestMain, which has no *testing.T.
func SkipSearchMain(suite, format string, args ...any) {
	reason := fmt.Sprintf(format, args...)
	if os.Getenv(RequireSearchEnv) != "" {
		fmt.Fprintf(os.Stderr, "FAIL: %s is set but %s cannot reach Meilisearch: %s\n",
			RequireSearchEnv, suite, reason)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "SKIP: %s not run: %s (set %s=1 to make this a failure)\n",
		suite, reason, RequireSearchEnv)
	os.Exit(0)
}
