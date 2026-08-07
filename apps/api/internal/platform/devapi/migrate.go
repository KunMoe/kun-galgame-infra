package devapi

import (
	"fmt"

	"gorm.io/gorm"
)

// Models returns the developer-platform models for AutoMigrate registration in
// cmd/migrate (the two new tables in kun_galgame_infra).
func Models() []any {
	return []any{&DeveloperAPIKey{}, &DeveloperAPIUsage{}}
}

// AddOAuthClientDevColumns adds the developer-platform columns to an existing
// oauth_clients table using raw SQL (裁定 8). A fresh database is a no-op here:
// the later AutoMigrate call creates the table with every column already
// present. For an existing table, the NOT NULL intent columns are added WITH a
// temporary DEFAULT so existing rows backfill, then the DEFAULT is dropped so
// the GORM zero-value INSERT trap can't reintroduce a silent default.
// owner_user_id is nullable (third-party app owner; NULL for first-party site
// clients) and gets a plain index.
//
// Idempotent — ADD COLUMN IF NOT EXISTS + a no-op DROP DEFAULT on re-run, so a
// second migration is a zero-change pass. cmd/migrate MUST call this BEFORE
// AutoMigrate(oauth_clients) so AutoMigrate never tries to add a NOT NULL column
// to a populated table (which Postgres rejects). Column types match GORM's
// mapping for the model fields (Go int → bigint, string size:20 → varchar(20)),
// so AutoMigrate reconciles to a no-op afterward.
func AddOAuthClientDevColumns(db *gorm.DB) error {
	if !db.Migrator().HasTable("oauth_clients") {
		return nil
	}

	stmts := []string{
		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS owner_user_id bigint`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_clients_owner_user_id ON oauth_clients (owner_user_id)`,

		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS dev_enabled boolean NOT NULL DEFAULT false`,
		`ALTER TABLE oauth_clients ALTER COLUMN dev_enabled DROP DEFAULT`,
		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS dev_tier varchar(20) NOT NULL DEFAULT 'free'`,
		`ALTER TABLE oauth_clients ALTER COLUMN dev_tier DROP DEFAULT`,
		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS dev_nsfw_allowed boolean NOT NULL DEFAULT false`,
		`ALTER TABLE oauth_clients ALTER COLUMN dev_nsfw_allowed DROP DEFAULT`,
		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS dev_rate_per_min bigint NOT NULL DEFAULT 0`,
		`ALTER TABLE oauth_clients ALTER COLUMN dev_rate_per_min DROP DEFAULT`,
		`ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS dev_quota_daily bigint NOT NULL DEFAULT 0`,
		`ALTER TABLE oauth_clients ALTER COLUMN dev_quota_daily DROP DEFAULT`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("devapi migrate %q: %w", s, err)
		}
	}
	return nil
}
