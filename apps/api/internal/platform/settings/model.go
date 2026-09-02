package settings

import (
	"time"

	"gorm.io/datatypes"
)

const ScopePlatform = "platform"

type SettingOverride struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ScopeKind       string         `gorm:"size:16;not null;uniqueIndex:uq_setting_override,priority:1" json:"scope_kind"`
	ScopeID         string         `gorm:"size:64;not null;uniqueIndex:uq_setting_override,priority:2" json:"scope_id"`
	Key             string         `gorm:"size:128;not null;uniqueIndex:uq_setting_override,priority:3" json:"key"`
	Value           datatypes.JSON `gorm:"type:jsonb;not null" json:"value"`
	Version         int64          `gorm:"not null" json:"version"`
	UpdatedByUserID uint           `gorm:"not null" json:"updated_by_user_id"`
	Note            string         `gorm:"size:512;not null" json:"note"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (SettingOverride) TableName() string { return "setting_overrides" }

const (
	ActionSet   = "set"
	ActionReset = "reset"
)

type SettingAuditLog struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ActorUserID uint           `gorm:"not null;index" json:"actor_user_id"`
	Action      string         `gorm:"size:8;not null" json:"action"`
	ScopeKind   string         `gorm:"size:16;not null" json:"scope_kind"`
	ScopeID     string         `gorm:"size:64;not null" json:"scope_id"`
	Key         string         `gorm:"size:128;not null;index" json:"key"`
	OldValue    datatypes.JSON `gorm:"type:jsonb" json:"old_value"`
	NewValue    datatypes.JSON `gorm:"type:jsonb" json:"new_value"`
	Note        string         `gorm:"size:512;not null" json:"note"`
	CreatedAt   time.Time      `gorm:"index" json:"created_at"`
}

func (SettingAuditLog) TableName() string { return "setting_audit_logs" }
