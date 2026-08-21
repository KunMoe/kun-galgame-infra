package devapi

import (
	"time"

	"gorm.io/datatypes"
)

type DeveloperAPIKey struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ClientID        string         `gorm:"size:50;not null;index" json:"client_id"`
	Name            string         `gorm:"size:100;not null" json:"name"`
	KeyHash         string         `gorm:"size:80;not null;uniqueIndex" json:"-"`
	KeyPrefix       string         `gorm:"size:24;not null;index" json:"key_prefix"`
	Last4           string         `gorm:"size:4;not null" json:"last4"`
	Scopes          datatypes.JSON `gorm:"type:jsonb;not null" json:"scopes"`
	NSFWAllowed     bool           `gorm:"not null" json:"nsfw_allowed"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	RevokedAt       *time.Time     `json:"revoked_at,omitempty"`
	LastUsedAt      *time.Time     `json:"last_used_at,omitempty"`
	CreatedByUserID uint           `gorm:"not null" json:"created_by_user_id"`
	CreatedAt       time.Time      `json:"created_at"`
}

func (DeveloperAPIKey) TableName() string { return "developer_api_keys" }

func (k *DeveloperAPIKey) Active(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return false
	}
	return true
}

type DeveloperAPIUsage struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	ClientID string `gorm:"size:50;not null;uniqueIndex:idx_usage_day,priority:1" json:"client_id"`
	KeyID    uint   `gorm:"not null;uniqueIndex:idx_usage_day,priority:2" json:"key_id"`
	// Face names grew past the original varchar(20) — galgame_internal_write (22)
	// and galgame_internal_propose (24) overflowed it, and because UpsertUsage is
	// one batched INSERT and Flush re-merges on error, a single long face poisoned
	// EVERY subsequent flush on that process (silent metering stall, 06a→06b W1).
	// Those two faces retired in wave-161 P5, but the column stays 40: historical
	// usage rows still carry their names, and re-narrowing a varchar is how a
	// future long face gets silently truncated instead of loudly rejected.
	Face string `gorm:"size:40;not null;uniqueIndex:idx_usage_day,priority:3" json:"face"`
	Day  string `gorm:"size:10;not null;uniqueIndex:idx_usage_day,priority:4" json:"day"`
	// The MATCHED ROUTE PATTERN (/v1/catalog/works/:id), never a concrete path —
	// a concrete id would make one row per work per key per day. Unbounded `text`
	// rather than a varchar for the reason the Face comment above records: a face
	// that outgrew its column poisoned every later flush in the same batch, and a
	// route pattern is exactly the kind of value that grows.
	Path  string `gorm:"type:text;not null;uniqueIndex:idx_usage_day,priority:5" json:"path"`
	Count int64  `gorm:"not null" json:"count"`
	// Explicit column tags: GORM's naming strategy renders Status4xx as
	// "status4xx" (no underscore before a digit — the acronym/digit trap), but
	// the upsert SQL and readers use status_4xx / status_5xx.
	Status4xx int64     `gorm:"column:status_4xx;not null" json:"status_4xx"`
	Status5xx int64     `gorm:"column:status_5xx;not null" json:"status_5xx"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DeveloperAPIUsage) TableName() string { return "developer_api_usage" }

const DeveloperUsageRetentionDays = 400

func RetentionCutoffDay(now time.Time) string {
	return now.UTC().AddDate(0, 0, -DeveloperUsageRetentionDays).Format("2006-01-02")
}
