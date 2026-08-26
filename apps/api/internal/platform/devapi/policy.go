package devapi

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CapabilityAppCreate = "app.create"
	CapabilityAppManage = "app.manage"
	CapabilityKeyMint   = "key.mint"
)

const (
	PolicySelfService = "self_service"
	PolicyApproval    = "approval"
	PolicyDisabled    = "disabled"
)

var (
	ErrUnknownCapability  = errors.New("devapi: unknown capability")
	ErrInvalidPolicyMode  = errors.New("devapi: mode not allowed for this capability")
	ErrCapabilityDisabled = errors.New("devapi: capability turned off by platform policy")
)

type Capability struct {
	Key     string   `json:"key"`
	LabelZH string   `json:"label_zh"`
	LabelEN string   `json:"label_en"`
	DescZH  string   `json:"desc_zh"`
	DescEN  string   `json:"desc_en"`
	Modes   []string `json:"modes"`
	Default string   `json:"default"`
}

// The whole policy matrix. Hand-maintained, and deliberately NOT a mirror of
// everything a portal user can do: which scopes a key may carry keeps its own
// mechanism in selfServiceScopes with its own tests, and putting it here too
// would give one decision two sources of truth. Key revocation is likewise
// absent on purpose — it is the stop-loss action and must survive any policy.
var capabilities = []Capability{
	{
		Key:     CapabilityAppCreate,
		LabelZH: "自助创建应用",
		LabelEN: "Create applications",
		DescZH:  "用户能否在门户里自己建应用;设为「需审批」时新应用先落待审、由管理台放行。",
		DescEN:  "Whether a portal user may register an application; under approval the new row waits disabled until the console lets it through.",
		Modes:   []string{PolicySelfService, PolicyApproval, PolicyDisabled},
		Default: PolicySelfService,
	},
	{
		Key:     CapabilityAppManage,
		LabelZH: "自助管理应用",
		LabelEN: "Manage own applications",
		DescZH:  "用户能否编辑与停用自己的应用。",
		DescEN:  "Whether an owner may edit and deactivate their own applications.",
		Modes:   []string{PolicySelfService, PolicyDisabled},
		Default: PolicySelfService,
	},
	{
		Key:     CapabilityKeyMint,
		LabelZH: "自助铸造密钥",
		LabelEN: "Mint own keys",
		DescZH:  "用户能否为自己的应用铸造与轮换密钥;吊销不受此项影响,任何时候都可用。",
		DescEN:  "Whether an owner may mint and rotate keys on their own applications. Revocation is never gated.",
		Modes:   []string{PolicySelfService, PolicyDisabled},
		Default: PolicySelfService,
	},
}

func capabilityByKey(key string) (Capability, bool) {
	for _, c := range capabilities {
		if c.Key == key {
			return c, true
		}
	}
	return Capability{}, false
}

func validatePolicy(capability, mode string) error {
	c, ok := capabilityByKey(capability)
	if !ok {
		return ErrUnknownCapability
	}
	if !slices.Contains(c.Modes, mode) {
		return ErrInvalidPolicyMode
	}
	return nil
}

// No row for a capability means the code default applies; deleting the row
// restores it. Mirrors role_permission_overrides — the last writer is recorded
// by SetByUserID + UpdatedAt rather than by a separate audit table.
type PolicyOverride struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Capability  string    `gorm:"size:64;not null;uniqueIndex" json:"capability"`
	Mode        string    `gorm:"size:20;not null" json:"mode"`
	SetByUserID uint      `gorm:"not null" json:"set_by_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (PolicyOverride) TableName() string { return "devapi_policy_overrides" }

func (r *Repository) PolicyOverrides(ctx context.Context) ([]PolicyOverride, error) {
	var rows []PolicyOverride
	err := r.db.WithContext(ctx).Order("capability ASC").Find(&rows).Error
	return rows, err
}

func (r *Repository) PolicyMode(ctx context.Context, capability string) (string, error) {
	c, ok := capabilityByKey(capability)
	if !ok {
		return "", ErrUnknownCapability
	}
	var row PolicyOverride
	err := r.db.WithContext(ctx).Where("capability = ?", capability).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Default, nil
	}
	if err != nil {
		return "", err
	}
	return row.Mode, nil
}

func (r *Repository) SetPolicyOverride(ctx context.Context, capability, mode string, userID uint) error {
	row := PolicyOverride{Capability: capability, Mode: mode, SetByUserID: userID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "capability"}},
		DoUpdates: clause.Assignments(map[string]any{
			"mode":           mode,
			"set_by_user_id": userID,
			"updated_at":     time.Now(),
		}),
	}).Create(&row).Error
}

func (r *Repository) DeletePolicyOverride(ctx context.Context, capability string) error {
	return r.db.WithContext(ctx).
		Where("capability = ?", capability).
		Delete(&PolicyOverride{}).Error
}

type PolicyView struct {
	Capability
	Mode        string `json:"mode"`
	Overridden  bool   `json:"overridden"`
	SetByUserID uint   `json:"set_by_user_id,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func (s *AdminService) Policies(ctx context.Context) ([]PolicyView, error) {
	rows, err := s.repo.PolicyOverrides(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]PolicyOverride, len(rows))
	for _, row := range rows {
		byKey[row.Capability] = row
	}
	out := make([]PolicyView, 0, len(capabilities))
	for _, c := range capabilities {
		view := PolicyView{Capability: c, Mode: c.Default}
		if row, ok := byKey[c.Key]; ok {
			view.Mode = row.Mode
			view.Overridden = true
			view.SetByUserID = row.SetByUserID
			view.UpdatedAt = row.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *AdminService) SetPolicy(ctx context.Context, capability, mode string, userID uint) error {
	if err := validatePolicy(capability, mode); err != nil {
		return err
	}
	if err := s.repo.SetPolicyOverride(ctx, capability, mode, userID); err != nil {
		return err
	}
	slog.Info("devapi policy set", "capability", capability, "mode", mode, "by", userID)
	return nil
}

func (s *AdminService) ResetPolicy(ctx context.Context, capability string, userID uint) error {
	if _, ok := capabilityByKey(capability); !ok {
		return ErrUnknownCapability
	}
	if err := s.repo.DeletePolicyOverride(ctx, capability); err != nil {
		return err
	}
	slog.Info("devapi policy reset to the code default", "capability", capability, "by", userID)
	return nil
}

func (s *SelfServiceService) EffectivePolicies(ctx context.Context) (map[string]string, error) {
	rows, err := s.repo.PolicyOverrides(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]string, len(rows))
	for _, row := range rows {
		byKey[row.Capability] = row.Mode
	}
	out := make(map[string]string, len(capabilities))
	for _, c := range capabilities {
		if mode, ok := byKey[c.Key]; ok {
			out[c.Key] = mode
			continue
		}
		out[c.Key] = c.Default
	}
	return out, nil
}

func (s *SelfServiceService) requireCapability(ctx context.Context, capability string) error {
	mode, err := s.repo.PolicyMode(ctx, capability)
	if err != nil {
		return err
	}
	if mode == PolicyDisabled {
		return ErrCapabilityDisabled
	}
	return nil
}
