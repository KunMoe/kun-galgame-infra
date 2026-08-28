// Package dbtest decides, in one place, what a service-backed suite does when it
// has no service to run against — Postgres below, Meilisearch in search.go.
//
// The two failure modes it sits between are both real and both have bitten this
// repo:
//
//   - Skipping silently. A suite whose TestMain exits 0 when it cannot reach a
//     database still prints `ok` for the package, so a whole DB-backed suite can
//     stop running and CI stays green. Nothing tells you; the only tell is the
//     runtime (~1s instead of ~5s).
//   - Failing unconditionally. The `unit` job in .github/workflows/test.yml runs
//     `go test ./...` with NO service containers on purpose, and its contract is
//     that service-gated tests skip cleanly. A suite that hard-fails on a missing
//     DSN turns that always-green gate permanently red, which is what happened
//     between 2026-08-02 and 2026-08-03.
//
// The distinction that resolves it is not "is a DSN present" but "was this run
// SUPPOSED to have a database". Only the caller of `go test` knows that, so it
// says so: the integration job sets REQUIRE_DB_TESTS=1, and a missing DSN there
// is a misconfigured job, not an absent service.
//
// Deliberately no default DSN. Falling back to a hard-coded localhost database
// is how a suite silently adopts whichever database happens to be listening —
// including one another track is mid-run against. On 2026-08-28 that was not
// hypothetical: 75 of the 76 files reading TEST_DATABASE_DSN read os.Getenv
// directly and never reached this package, 30 of them carrying a
// `dbname=kun_catalog_test` fallback whose database exists on the development
// box. Route every read through DSN(); a bare os.Getenv here is the defect.
package dbtest

import (
	"fmt"
	"os"
	"testing"
)

const RequireEnv = "REQUIRE_DB_TESTS"

func DSN() (dsn string, ok bool) {
	dsn = os.Getenv("TEST_DATABASE_DSN")
	if dsn != "" {
		return dsn, true
	}
	if Required() {
		fmt.Fprintf(os.Stderr,
			"FAIL: %s is set but TEST_DATABASE_DSN is empty — this run was supposed to have a database\n",
			RequireEnv)
		os.Exit(2)
	}
	return "", false
}

func SkipMain(suite string) {
	fmt.Fprintf(os.Stderr,
		"SKIP: TEST_DATABASE_DSN unset — %s not run (set %s=1 to make this a failure)\n",
		suite, RequireEnv)
	os.Exit(0)
}

func Skip(t *testing.T) {
	t.Helper()
	t.Skipf("TEST_DATABASE_DSN unset — DB-backed test not run (set %s=1 to make this a failure)", RequireEnv)
}

// SkipMainf and Skipf are for the refusals that happen AFTER a DSN was handed
// over: an unreachable server, a migration that will not run. Those used to
// os.Exit(0) too, so a suite whose migration failed under REQUIRE_DB_TESTS=1
// still printed `ok` — the same lie DSN() closes, one step further down.
func SkipMainf(suite, format string, args ...any) {
	reason := fmt.Sprintf(format, args...)
	if Required() {
		fmt.Fprintf(os.Stderr, "FAIL: %s is set but %s cannot use the database: %s\n",
			RequireEnv, suite, reason)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "SKIP: %s not run: %s (set %s=1 to make this a failure)\n",
		suite, reason, RequireEnv)
	os.Exit(0)
}

func Skipf(t *testing.T, format string, args ...any) {
	t.Helper()
	reason := fmt.Sprintf(format, args...)
	if Required() {
		t.Fatalf("%s is set but the database is unusable: %s", RequireEnv, reason)
	}
	t.Skipf("%s — DB-backed test not run (set %s=1 to make this a failure)", reason, RequireEnv)
}

// Required is for the few callers that decide for themselves what an unusable
// database means — newstest.Open is reached from a TestMain and from a per-test
// helper, so neither os.Exit nor t.Fatal fits both.
func Required() bool { return os.Getenv(RequireEnv) != "" }
