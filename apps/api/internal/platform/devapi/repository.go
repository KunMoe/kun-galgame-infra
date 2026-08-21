package devapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"

	siteModel "api/internal/platform/site/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type resolveRow struct {
	KeyID         uint
	ClientID      string
	AppName       string
	KeyHash       string
	KeyScopes     []byte
	KeyNSFW       bool
	RevokedAt     *time.Time
	ExpiresAt     *time.Time
	DevEnabled    bool
	DevTier       string
	DevNSFW       bool
	DevRatePerMin int
	DevQuotaDaily int
}

func (r *Repository) ResolveByHash(ctx context.Context, hash string, now time.Time) (*Credential, error) {
	var row resolveRow
	err := r.db.WithContext(ctx).
		Table("developer_api_keys AS k").
		Select(`k.id AS key_id, k.client_id AS client_id, c.name AS app_name,
			k.key_hash AS key_hash, k.scopes AS key_scopes, k.nsfw_allowed AS key_nsfw,
			k.revoked_at AS revoked_at, k.expires_at AS expires_at,
			c.dev_enabled AS dev_enabled, c.dev_tier AS dev_tier,
			c.dev_nsfw_allowed AS dev_nsfw, c.dev_rate_per_min AS dev_rate_per_min,
			c.dev_quota_daily AS dev_quota_daily`).
		Joins("JOIN oauth_clients AS c ON c.id = k.client_id").
		Where("k.key_hash = ?", hash).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(row.KeyHash), []byte(hash)) != 1 {
		return nil, nil
	}
	if !row.DevEnabled || row.RevokedAt != nil {
		return nil, nil
	}
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return nil, nil
	}

	var scopes []string
	if len(row.KeyScopes) > 0 {
		_ = json.Unmarshal(row.KeyScopes, &scopes)
	}
	return &Credential{
		KeyID:         row.KeyID,
		ClientID:      row.ClientID,
		AppName:       row.AppName,
		Tier:          row.DevTier,
		Scopes:        scopes,
		NSFWAllowed:   row.KeyNSFW && row.DevNSFW,
		RateOverride:  row.DevRatePerMin,
		QuotaOverride: row.DevQuotaDaily,
	}, nil
}

func (r *Repository) CreateKey(ctx context.Context, key *DeveloperAPIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *Repository) ListKeysByClient(ctx context.Context, clientID string) ([]DeveloperAPIKey, error) {
	var keys []DeveloperAPIKey
	err := r.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *Repository) CountKeysByClient(ctx context.Context, clientID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIKey{}).
		Where("client_id = ? AND revoked_at IS NULL", clientID).
		Count(&n).Error
	return n, err
}

func (r *Repository) GetKey(ctx context.Context, id uint) (*DeveloperAPIKey, error) {
	var key DeveloperAPIKey
	if err := r.db.WithContext(ctx).First(&key, id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *Repository) SetKeyExpiry(ctx context.Context, id uint, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&DeveloperAPIKey{}).
		Where("id = ?", id).
		Update("expires_at", expiresAt).Error
}

func (r *Repository) RevokeKey(ctx context.Context, id uint, now time.Time) error {
	return r.db.WithContext(ctx).
		Model(&DeveloperAPIKey{}).
		Where("id = ?", id).
		Update("revoked_at", now).Error
}

func (r *Repository) TouchLastUsed(ctx context.Context, id uint, now time.Time) error {
	return r.db.WithContext(ctx).
		Model(&DeveloperAPIKey{}).
		Where("id = ?", id).
		Update("last_used_at", now).Error
}

func (r *Repository) GetApp(ctx context.Context, clientID string) (*siteModel.OAuthClient, error) {
	var app siteModel.OAuthClient
	if err := r.db.WithContext(ctx).Where("id = ?", clientID).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) UpdateAppDevConfig(ctx context.Context, clientID string, fields map[string]any) error {
	return r.UpdateAppFields(ctx, clientID, fields)
}

func (r *Repository) UpdateAppFields(ctx context.Context, clientID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&siteModel.OAuthClient{}).
		Where("id = ?", clientID).
		Updates(fields).Error
}

func (r *Repository) CreateApp(ctx context.Context, app *siteModel.OAuthClient) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *Repository) ListAppsByOwner(ctx context.Context, ownerUserID uint) ([]siteModel.OAuthClient, error) {
	var apps []siteModel.OAuthClient
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ?", ownerUserID).
		Order("created_at DESC").
		Find(&apps).Error
	return apps, err
}

func (r *Repository) GetAppByOwner(ctx context.Context, clientID string, ownerUserID uint) (*siteModel.OAuthClient, error) {
	var app siteModel.OAuthClient
	if err := r.db.WithContext(ctx).
		Where("id = ? AND owner_user_id = ?", clientID, ownerUserID).
		First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) CountAppsByOwner(ctx context.Context, ownerUserID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&siteModel.OAuthClient{}).
		Where("owner_user_id = ?", ownerUserID).
		Count(&n).Error
	return n, err
}

func (r *Repository) CountActiveKeysByClient(ctx context.Context, clientID string, now time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIKey{}).
		Where("client_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", clientID, now).
		Count(&n).Error
	return n, err
}

const (
	KeyStateActive  = "active"
	KeyStateRevoked = "revoked"
	KeyStateExpired = "expired"
	KeyStateAll     = "all"
)

type KeyListFilter struct {
	ClientID string
	State    string
	Page     int
	Limit    int
}

type AdminKeyRow struct {
	DeveloperAPIKey
	AppName     string `gorm:"column:app_name"`
	OwnerUserID *uint  `gorm:"column:owner_user_id"`
}

// ListAllKeys is the cross-application key inventory behind the console's
// global key page; the per-application list stays ListKeysByClient.
func (r *Repository) ListAllKeys(ctx context.Context, f KeyListFilter, now time.Time) ([]AdminKeyRow, int64, error) {
	scoped := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Table("developer_api_keys AS k").
			Joins("JOIN oauth_clients AS c ON c.id = k.client_id")
		if f.ClientID != "" {
			q = q.Where("k.client_id = ?", f.ClientID)
		}
		switch f.State {
		case KeyStateActive:
			q = q.Where("k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > ?)", now)
		case KeyStateRevoked:
			q = q.Where("k.revoked_at IS NOT NULL")
		case KeyStateExpired:
			q = q.Where("k.revoked_at IS NULL AND k.expires_at IS NOT NULL AND k.expires_at <= ?", now)
		}
		return q
	}

	var total int64
	if err := scoped().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AdminKeyRow
	err := scoped().
		Select("k.*, c.name AS app_name, c.owner_user_id AS owner_user_id").
		Order("k.created_at DESC, k.id DESC").
		Offset((f.Page - 1) * f.Limit).
		Limit(f.Limit).
		Scan(&rows).Error
	return rows, total, err
}

func (k *DeveloperAPIKey) State(now time.Time) string {
	switch {
	case k.RevokedAt != nil:
		return KeyStateRevoked
	case k.ExpiresAt != nil && !k.ExpiresAt.After(now):
		return KeyStateExpired
	default:
		return KeyStateActive
	}
}

type OwnerActiveKey struct {
	KeyID         uint   `gorm:"column:key_id"`
	AppName       string `gorm:"column:app_name"`
	DevTier       string `gorm:"column:dev_tier"`
	DevRatePerMin int    `gorm:"column:dev_rate_per_min"`
	DevQuotaDaily int    `gorm:"column:dev_quota_daily"`
}

func (r *Repository) ListOwnerActiveKeys(ctx context.Context, ownerUserID uint, now time.Time) ([]OwnerActiveKey, error) {
	var rows []OwnerActiveKey
	err := r.db.WithContext(ctx).
		Table("developer_api_keys AS k").
		Select(`k.id AS key_id, c.name AS app_name, c.dev_tier AS dev_tier,
			c.dev_rate_per_min AS dev_rate_per_min, c.dev_quota_daily AS dev_quota_daily`).
		Joins("JOIN oauth_clients AS c ON c.id = k.client_id").
		Where(`c.owner_user_id = ? AND c.dev_enabled = ? AND k.revoked_at IS NULL
			AND (k.expires_at IS NULL OR k.expires_at > ?)`, ownerUserID, true, now).
		Order("c.name ASC, k.id ASC").
		Scan(&rows).Error
	return rows, err
}

type UsageFaceTotal struct {
	Face      string `gorm:"column:face" json:"face"`
	Count     int64  `gorm:"column:count" json:"count"`
	Status4xx int64  `gorm:"column:status_4xx" json:"status_4xx"`
	Status5xx int64  `gorm:"column:status_5xx" json:"status_5xx"`
}

func (r *Repository) SumUsageByFace(ctx context.Context, clientIDs []string, sinceDay string) ([]UsageFaceTotal, error) {
	var rows []UsageFaceTotal
	if len(clientIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIUsage{}).
		Select("face, SUM(count) AS count, SUM(status_4xx) AS status_4xx, SUM(status_5xx) AS status_5xx").
		Where("client_id IN ? AND day >= ?", clientIDs, sinceDay).
		Group("face").
		Order("count DESC, face ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *Repository) CountUsageBefore(ctx context.Context, beforeDay string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIUsage{}).
		Where("day < ?", beforeDay).
		Count(&n).Error
	return n, err
}

func (r *Repository) PruneUsageBefore(ctx context.Context, beforeDay string) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("day < ?", beforeDay).
		Delete(&DeveloperAPIUsage{})
	return res.RowsAffected, res.Error
}

type UsageDayFace struct {
	Day       string `gorm:"column:day" json:"day"`
	Face      string `gorm:"column:face" json:"face"`
	Count     int64  `gorm:"column:count" json:"count"`
	Status4xx int64  `gorm:"column:status_4xx" json:"status_4xx"`
	Status5xx int64  `gorm:"column:status_5xx" json:"status_5xx"`
}

func (r *Repository) AggregateUsageByClient(ctx context.Context, clientID, sinceDay string) ([]UsageDayFace, error) {
	var rows []UsageDayFace
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIUsage{}).
		Select("day, face, SUM(count) AS count, SUM(status_4xx) AS status_4xx, SUM(status_5xx) AS status_5xx").
		Where("client_id = ? AND day >= ?", clientID, sinceDay).
		Group("day, face").
		Order("day DESC, face ASC").
		Scan(&rows).Error
	return rows, err
}

type UsageDayTotal struct {
	Day       string `gorm:"column:day" json:"day"`
	Count     int64  `gorm:"column:count" json:"count"`
	Status4xx int64  `gorm:"column:status_4xx" json:"status_4xx"`
	Status5xx int64  `gorm:"column:status_5xx" json:"status_5xx"`
}

type UsageClientTotal struct {
	ClientID  string `gorm:"column:client_id" json:"client_id"`
	Count     int64  `gorm:"column:count" json:"count"`
	Status4xx int64  `gorm:"column:status_4xx" json:"status_4xx"`
	Status5xx int64  `gorm:"column:status_5xx" json:"status_5xx"`
}

func (r *Repository) SumUsageByDay(ctx context.Context, clientIDs []string, sinceDay string) ([]UsageDayTotal, error) {
	var rows []UsageDayTotal
	if len(clientIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIUsage{}).
		Select("day, SUM(count) AS count, SUM(status_4xx) AS status_4xx, SUM(status_5xx) AS status_5xx").
		Where("client_id IN ? AND day >= ?", clientIDs, sinceDay).
		Group("day").
		Order("day ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *Repository) SumUsageByClient(ctx context.Context, clientIDs []string, sinceDay string) ([]UsageClientTotal, error) {
	var rows []UsageClientTotal
	if len(clientIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).
		Model(&DeveloperAPIUsage{}).
		Select("client_id, SUM(count) AS count, SUM(status_4xx) AS status_4xx, SUM(status_5xx) AS status_5xx").
		Where("client_id IN ? AND day >= ?", clientIDs, sinceDay).
		Group("client_id").
		Scan(&rows).Error
	return rows, err
}

func (r *Repository) UpsertUsage(ctx context.Context, rows []DeveloperAPIUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		// Every column of idx_usage_day belongs here. A conflict target narrower
		// than the unique index makes the upsert judge "already counted this" on
		// the wrong tuple: correct-looking, and then 23505 rolls back the whole
		// batched INSERT — and Flush re-merges on error, so the stall repeats.
		Columns: []clause.Column{
			{Name: "client_id"}, {Name: "key_id"}, {Name: "face"}, {Name: "day"}, {Name: "path"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"count":      gorm.Expr("developer_api_usage.count + excluded.count"),
			"status_4xx": gorm.Expr("developer_api_usage.status_4xx + excluded.status_4xx"),
			"status_5xx": gorm.Expr("developer_api_usage.status_5xx + excluded.status_5xx"),
			"updated_at": gorm.Expr("excluded.updated_at"),
		}),
	}).Create(&rows).Error
}
