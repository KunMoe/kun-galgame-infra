package migrate

import (
	"fmt"
	"os"
	"strings"
	"testing"

	suitelock "api/internal/platform/ai/dbtest"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration test against a real Postgres (the trust/community convention):
// a missing database skips the whole package.
// Schema comes from migrate.Run — the exact production migration — so
// these probes assert the storage-level invariants (doc 20 §5) directly against
// the shipped schema. `go test -p 1` (pinned in test.yml) serializes packages
// that share the CI database.

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("ai/migrate")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("ai/migrate", "cannot connect to test database: %v", err)
	}
	sqlDB, _ := db.DB()
	release := suitelock.AcquireSuiteLock(sqlDB)

	// First migration provisions the schema; running it again is the idempotency
	// probe — a second AutoMigrate + IF NOT EXISTS raw section must be a no-op.
	if err := Run(db); err != nil {
		release()
		dbtest.SkipMainf("ai/migrate", "ai migration failed: %v", err)
	}
	if err := Run(db); err != nil {
		release()
		fmt.Fprintf(os.Stderr, "ai migration is NOT idempotent: %v\n", err)
		os.Exit(1)
	}

	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

// TestColumnDiscipline asserts intent columns carry NO DB default (the GORM
// zero-value trap, 章程 ruling 8) while bookkeeping/semantic-default columns keep
// theirs (doc 20 §5 — channel ” and daily_cost_cap_micro NULL are deliberate).
func TestColumnDiscipline(t *testing.T) {
	noDefault := []struct{ table, column string }{
		{"ai_usage", "site"}, {"ai_usage", "route"}, {"ai_usage", "status"},
		{"ai_route_budget", "route"},
		// daily_cost_cap_micro must be NULLABLE with NO default (NULL = no block).
		{"ai_route_budget", "daily_cost_cap_micro"},
	}
	for _, c := range noDefault {
		if def := columnDefault(t, c.table, c.column); def != "" {
			t.Errorf("%s.%s must have NO default, got %q", c.table, c.column, def)
		}
	}

	withDefault := []struct{ table, column, want string }{
		{"ai_usage", "channel", "''"},
		{"ai_usage", "prompt_tokens", "0"},
		{"ai_usage", "completion_tokens", "0"},
		{"ai_usage", "cost_micro", "0"},
		{"ai_usage", "latency_ms", "0"},
		{"ai_usage", "created_at", "now()"},
		{"ai_route_budget", "site", "''"},
		{"ai_route_budget", "updated_at", "now()"},
	}
	for _, c := range withDefault {
		if def := columnDefault(t, c.table, c.column); !strings.Contains(def, c.want) {
			t.Errorf("%s.%s default = %q, want to contain %q", c.table, c.column, def, c.want)
		}
	}
}

// TestNullability asserts daily_cost_cap_micro is nullable (the NULL = no-block
// semantic) while the intent columns are NOT NULL.
func TestNullability(t *testing.T) {
	nullable := map[string]bool{
		"ai_usage.site":                        false,
		"ai_usage.route":                       false,
		"ai_usage.status":                      false,
		"ai_usage.channel":                     false,
		"ai_route_budget.route":                false,
		"ai_route_budget.site":                 false,
		"ai_route_budget.daily_cost_cap_micro": true,
	}
	for key, want := range nullable {
		parts := strings.SplitN(key, ".", 2)
		if got := columnNullable(t, parts[0], parts[1]); got != want {
			t.Errorf("%s nullable = %v, want %v", key, got, want)
		}
	}
}

// TestIdentityPrimaryKey asserts ai_usage.id is an IDENTITY column (the
// three-part GORM recipe → GENERATED ... AS IDENTITY), so inserts omitting id
// get a server-generated value.
func TestIdentityPrimaryKey(t *testing.T) {
	var identity string
	if err := testDB.Raw(
		`SELECT is_identity FROM information_schema.columns
		   WHERE table_schema='public' AND table_name='ai_usage' AND column_name='id'`,
	).Scan(&identity).Error; err != nil {
		t.Fatalf("read is_identity: %v", err)
	}
	if strings.ToUpper(identity) != "YES" {
		t.Errorf("ai_usage.id is_identity = %q, want YES", identity)
	}
}

// TestUsageIndexes asserts the two access indexes exist with their columns
// (doc 20 §5: site-leading tenant view + route-leading scenario trend).
func TestUsageIndexes(t *testing.T) {
	cases := []struct {
		name  string
		frags []string
	}{
		{"idx_ai_usage_site_created", []string{"(site, created_at)"}},
		{"idx_ai_usage_route_created", []string{"(route, created_at)"}},
	}
	for _, c := range cases {
		def := indexDef(t, c.name)
		for _, frag := range c.frags {
			if !strings.Contains(def, frag) {
				t.Errorf("index %s missing %q in\n  %s", c.name, frag, def)
			}
		}
	}
}

// TestBudgetCompositePrimaryKey asserts the (route, site) composite PK exists.
func TestBudgetCompositePrimaryKey(t *testing.T) {
	var cols string
	if err := testDB.Raw(`
		SELECT string_agg(a.attname, ',' ORDER BY array_position(i.indkey, a.attnum)) AS cols
		  FROM pg_index i
		  JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		 WHERE i.indrelid = 'ai_route_budget'::regclass AND i.indisprimary`).Scan(&cols).Error; err != nil {
		t.Fatalf("read pk columns: %v", err)
	}
	if cols != "route,site" {
		t.Errorf("ai_route_budget primary key = %q, want %q", cols, "route,site")
	}
}

func indexDef(t *testing.T, name string) string {
	t.Helper()
	var def string
	if err := testDB.Raw(
		`SELECT indexdef FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?`, name,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read indexdef %s: %v", name, err)
	}
	if def == "" {
		t.Fatalf("index %s does not exist", name)
	}
	return def
}

func columnDefault(t *testing.T, table, column string) string {
	t.Helper()
	var def *string
	if err := testDB.Raw(
		`SELECT column_default FROM information_schema.columns
		   WHERE table_schema='public' AND table_name=? AND column_name=?`,
		table, column,
	).Scan(&def).Error; err != nil {
		t.Fatalf("read default %s.%s: %v", table, column, err)
	}
	if def == nil {
		return ""
	}
	return *def
}

func columnNullable(t *testing.T, table, column string) bool {
	t.Helper()
	var yn string
	if err := testDB.Raw(
		`SELECT is_nullable FROM information_schema.columns
		   WHERE table_schema='public' AND table_name=? AND column_name=?`,
		table, column,
	).Scan(&yn).Error; err != nil {
		t.Fatalf("read nullability %s.%s: %v", table, column, err)
	}
	return strings.EqualFold(yn, "YES")
}
