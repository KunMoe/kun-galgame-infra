package dbtest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"api/internal/infrastructure/opensearch"
	"api/pkg/config"
)

// RequireSearchEnv is REQUIRE_DB_TESTS for the live search engine. It exists
// because the works-search suite skipped in full while catalog/service still
// printed `ok`, and the W4 wave shipped a filter that returned zero hits for
// every query.
const RequireSearchEnv = "REQUIRE_SEARCH_TESTS"

const openSearchTestHostEnv = "KUN_OPENSEARCH_TEST_HOST"

func SearchHost() string {
	return os.Getenv(openSearchTestHostEnv)
}

// SearchIndexPrefix names indexes for one test binary and no other. The two
// search suites used the fixed prefixes "test_" and "test_svc_" against one
// local engine, so two sessions running the suite at once shared every
// index — and each test's cleanup pulled the other run's corpus out from
// under it mid-assertion. Pair it with SweepSearchIndexes: unique names
// with no sweep only trades the collision for a pile of dead indexes.
func SearchIndexPrefix(suite string) string {
	return fmt.Sprintf("test_%s_%d_%d_", suite, os.Getpid(), time.Now().UnixNano()%1_000_000)
}

func OpenSearchClient(t *testing.T, prefix string) *opensearch.Client {
	t.Helper()
	host := SearchHost()
	if host == "" {
		SkipSearch(t, "%s unset", openSearchTestHostEnv)
	}
	client, err := opensearch.NewClient(config.OpenSearchConfig{Host: host, IndexPrefix: prefix})
	if err != nil {
		SkipSearch(t, "opensearch client: %v", err)
	}
	if err := client.Health(t.Context()); err != nil {
		SkipSearch(t, "opensearch unreachable: %v", err)
	}
	return client
}

func SweepSearchIndexes(client *opensearch.Client, prefix string) {
	if client == nil || prefix == "" {
		return
	}
	var rows []struct {
		Index string `json:"index"`
	}
	if err := client.Do(context.Background(), http.MethodGet, "/_cat/indices?format=json", nil, &rows); err != nil {
		return
	}
	for _, row := range rows {
		if row.Index == "" || !strings.HasPrefix(row.Index, prefix) {
			continue
		}
		_ = client.Do(context.Background(), http.MethodDelete, "/"+row.Index, nil, nil)
	}
}

// SkipSearch skips a search-backed test, or fails it when the run was supposed
// to reach OpenSearch. Every reason a search test declines to run — no host,
// an unusable client, an unreachable server — must come through here.
func SkipSearch(t *testing.T, format string, args ...any) {
	t.Helper()
	reason := fmt.Sprintf(format, args...)
	if os.Getenv(RequireSearchEnv) != "" {
		t.Fatalf("%s is set but OpenSearch is unusable: %s", RequireSearchEnv, reason)
	}
	t.Skipf("%s — search-backed test not run (set %s=1 to make this a failure)", reason, RequireSearchEnv)
}

// SkipSearchMain is SkipSearch for a TestMain, which has no *testing.T.
func SkipSearchMain(suite, format string, args ...any) {
	reason := fmt.Sprintf(format, args...)
	if os.Getenv(RequireSearchEnv) != "" {
		fmt.Fprintf(os.Stderr, "FAIL: %s is set but %s cannot reach OpenSearch: %s\n",
			RequireSearchEnv, suite, reason)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "SKIP: %s not run: %s (set %s=1 to make this a failure)\n",
		suite, reason, RequireSearchEnv)
	os.Exit(0)
}
