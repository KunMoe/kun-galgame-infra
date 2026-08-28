package dbtest

import (
	"fmt"
	"os"
	"testing"
)

// RequireSearchEnv is REQUIRE_DB_TESTS for Meilisearch. It exists because the
// works-search suite skipped in full while catalog/service still printed `ok`,
// and the W4 wave shipped a filter that returned zero hits for every query.
const RequireSearchEnv = "REQUIRE_SEARCH_TESTS"

func SearchHost() (host, apiKey string) {
	return os.Getenv("MEILISEARCH_TEST_HOST"), os.Getenv("MEILISEARCH_TEST_API_KEY")
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
