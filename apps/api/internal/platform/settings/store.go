package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

var (
	ErrVersionConflict = errors.New("settings: override changed since it was read")
	ErrNoOverride      = errors.New("settings: no override row to reset")
)

type OverrideRow struct {
	SettingOverride
	UpdatedByName string
}

func (s *Store) Overrides(ctx context.Context, scope Scope) ([]OverrideRow, error) {
	var rows []OverrideRow
	err := s.db.WithContext(ctx).
		Model(&SettingOverride{}).
		Select("setting_overrides.*, users.name AS updated_by_name").
		Joins("LEFT JOIN users ON users.id = setting_overrides.updated_by_user_id").
		Where("setting_overrides.scope_kind = ? AND setting_overrides.scope_id = ?", scope.Kind, scope.ID).
		Order("setting_overrides.key").
		Scan(&rows).Error
	return rows, err
}

func (s *Store) Values(ctx context.Context, scope Scope) (map[string]json.RawMessage, error) {
	var rows []SettingOverride
	err := s.db.WithContext(ctx).
		Where("scope_kind = ? AND scope_id = ?", scope.Kind, scope.ID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(rows))
	for _, r := range rows {
		out[r.Key] = json.RawMessage(r.Value)
	}
	return out, nil
}

func (s *Store) Set(ctx context.Context, scope Scope, key string, value json.RawMessage, note string, expectVersion *int64, actorID uint) (*SettingOverride, error) {
	compacted, err := compactJSON(value)
	if err != nil {
		return nil, err
	}

	var out SettingOverride
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row SettingOverride
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("scope_kind = ? AND scope_id = ? AND key = ?", scope.Kind, scope.ID, key).
			Take(&row).Error
		exists := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if expectVersion != nil {
			if exists && row.Version != *expectVersion {
				return ErrVersionConflict
			}
			if !exists && *expectVersion != 0 {
				return ErrVersionConflict
			}
		}

		var oldValue datatypes.JSON
		if exists {
			oldValue = row.Value
			row.Value = compacted
			row.Version++
			row.Note = note
			row.UpdatedByUserID = actorID
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		} else {
			row = SettingOverride{
				ScopeKind:       scope.Kind,
				ScopeID:         scope.ID,
				Key:             key,
				Value:           compacted,
				Version:         1,
				UpdatedByUserID: actorID,
				Note:            note,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}

		out = row
		return tx.Create(&SettingAuditLog{
			ActorUserID: actorID,
			Action:      ActionSet,
			ScopeKind:   scope.Kind,
			ScopeID:     scope.ID,
			Key:         key,
			OldValue:    oldValue,
			NewValue:    compacted,
			Note:        note,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) Reset(ctx context.Context, scope Scope, key string, note string, actorID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row SettingOverride
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("scope_kind = ? AND scope_id = ? AND key = ?", scope.Kind, scope.ID, key).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNoOverride
		}
		if err != nil {
			return err
		}

		if err := tx.Delete(&SettingOverride{}, row.ID).Error; err != nil {
			return err
		}
		return tx.Create(&SettingAuditLog{
			ActorUserID: actorID,
			Action:      ActionReset,
			ScopeKind:   scope.Kind,
			ScopeID:     scope.ID,
			Key:         key,
			OldValue:    row.Value,
			Note:        note,
		}).Error
	})
}

type AuditEntry struct {
	ID          uint            `json:"id"`
	ActorUserID uint            `json:"actor_user_id"`
	ActorName   string          `json:"actor_name"`
	Action      string          `json:"action"`
	ScopeKind   string          `json:"scope_kind"`
	ScopeID     string          `json:"scope_id"`
	Key         string          `json:"key"`
	OldValue    json.RawMessage `json:"old_value"`
	NewValue    json.RawMessage `json:"new_value"`
	Note        string          `json:"note"`
	CreatedAt   string          `json:"created_at"`
}

func (s *Store) RecentAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []struct {
		SettingAuditLog
		ActorName string
	}
	err := s.db.WithContext(ctx).
		Model(&SettingAuditLog{}).
		Select("setting_audit_logs.*, users.name AS actor_name").
		Joins("LEFT JOIN users ON users.id = setting_audit_logs.actor_user_id").
		Order("setting_audit_logs.id DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]AuditEntry, len(rows))
	for i, r := range rows {
		out[i] = AuditEntry{
			ID:          r.ID,
			ActorUserID: r.ActorUserID,
			ActorName:   r.ActorName,
			Action:      r.Action,
			ScopeKind:   r.ScopeKind,
			ScopeID:     r.ScopeID,
			Key:         r.Key,
			OldValue:    jsonOrNull(r.OldValue),
			NewValue:    jsonOrNull(r.NewValue),
			Note:        r.Note,
			CreatedAt:   r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	return out, nil
}

func compactJSON(raw json.RawMessage) (datatypes.JSON, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, err
	}
	return datatypes.JSON(buf.Bytes()), nil
}

func jsonOrNull(v datatypes.JSON) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(v)
}
