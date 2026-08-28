// Package newstest provisions a news test database for the packages above the
// migration. It lives beside dbtest rather than inside it because dbtest is
// imported by the migration's own test, and a helper that runs the migration
// would close that loop into an import cycle.
package newstest

import (
	"fmt"
	"os"

	suitelock "api/internal/platform/news/dbtest"
	"api/internal/platform/news/migrate"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to the news test database and provisions it with the exact
// production migration, returning the handle and the suite-lock release.
//
// A missing database SKIPS the package — and a skipped package still reports
// ok, so REQUIRE_DB_TESTS is the only thing standing between "the suite ran"
// and "the suite printed ok". Every refusal below goes through it.
func Open() (*gorm.DB, func(), bool) {
	dsn, ok := dbtest.DSN()
	if !ok {
		fmt.Fprintln(os.Stderr, "SKIP: TEST_DATABASE_DSN unset — news test database not provisioned")
		return nil, nil, false
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, nil, unusable("cannot connect to test database: %v", err)
	}
	sqlDB, _ := db.DB()
	release := suitelock.AcquireSuiteLock(sqlDB)

	if err := migrate.Run(db); err != nil {
		release()
		return nil, nil, unusable("news migration failed: %v", err)
	}
	return db, release, true
}

// unusable always reports false; it exits first when the run was supposed to
// have a database, because Open is called from TestMains that os.Exit(0) on a
// false and from a per-test helper that t.Skips on one.
func unusable(format string, args ...any) bool {
	reason := fmt.Sprintf(format, args...)
	if dbtest.Required() {
		fmt.Fprintf(os.Stderr, "FAIL: %s is set but the news test database is unusable: %s\n",
			dbtest.RequireEnv, reason)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "SKIP: %s\n", reason)
	return false
}

// Truncate empties every news item table, children first. news_source survives:
// it is the seeded registry, not fixture data.
func Truncate(db *gorm.DB) error {
	return db.Exec(`TRUNCATE news_moderation_decision, news_moderation_verdict,
		news_item_work, news_item_image, news_item RESTART IDENTITY CASCADE`).Error
}
