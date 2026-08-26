package devapi

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

const (
	ScopeAppPending  = "pending"
	ScopeAppApproved = "approved"
	ScopeAppDeclined = "declined"
)

// Counted in runes, not bytes: these two fields are written in Chinese, where a
// byte cap of 2000 is really ~666 characters — a limit the UI and the error text
// both promise as 2000.
const maxScopeAppMessageLen = 2000

var (
	ErrScopeNotGrantable   = errors.New("devapi: scope is not applied for (want news:read or store:read)")
	ErrScopeAppMessage     = errors.New("devapi: message is required")
	ErrScopeAppMsgTooLong  = errors.New("devapi: message too long (max 2000)")
	ErrScopeAppPending     = errors.New("devapi: an application for this scope is already awaiting review")
	ErrScopeAppApproved    = errors.New("devapi: this scope is already granted")
	ErrScopeAppNotPending  = errors.New("devapi: only a pending application can be reviewed")
	ErrScopeAppNeedsReason = errors.New("devapi: a decline needs a reason")
)

// One row per (user, scope): a declined applicant re-applies by putting the same
// row back into pending rather than stacking a second request, which is what the
// unique index enforces.
type ScopeApplication struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"not null;index;uniqueIndex:uq_devapi_scope_app,priority:1" json:"user_id"`
	Scope         string     `gorm:"size:64;not null;uniqueIndex:uq_devapi_scope_app,priority:2" json:"scope"`
	Message       string     `gorm:"type:text;not null" json:"message"`
	Status        string     `gorm:"size:16;not null" json:"status"`
	ReviewerID    *uint      `gorm:"column:reviewer_id" json:"reviewer_id,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	DeclineReason string     `gorm:"type:text;default:''" json:"decline_reason"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (ScopeApplication) TableName() string { return "devapi_scope_applications" }

func (r *Repository) CreateScopeApplication(ctx context.Context, app *ScopeApplication) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *Repository) GetScopeApplication(ctx context.Context, id uint) (*ScopeApplication, error) {
	var app ScopeApplication
	if err := r.db.WithContext(ctx).First(&app, id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) FindScopeApplication(ctx context.Context, userID uint, scope string) (*ScopeApplication, error) {
	var app ScopeApplication
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND scope = ?", userID, scope).
		Take(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *Repository) HasApprovedScopeApplication(ctx context.Context, userID uint, scope string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&ScopeApplication{}).
		Where("user_id = ? AND scope = ? AND status = ?", userID, scope, ScopeAppApproved).
		Count(&n).Error
	return n > 0, err
}

func (r *Repository) ListScopeApplicationsByUser(ctx context.Context, userID uint) ([]ScopeApplication, error) {
	var apps []ScopeApplication
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&apps).Error
	return apps, err
}

func (r *Repository) ListScopeApplications(ctx context.Context, status string) ([]ScopeApplication, error) {
	var apps []ScopeApplication
	q := r.db.WithContext(ctx)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("updated_at DESC").Find(&apps).Error
	return apps, err
}

func (r *Repository) UpdateScopeApplicationFields(ctx context.Context, id uint, fields map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&ScopeApplication{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (s *SelfServiceService) ApplyForScope(ctx context.Context, userID uint, scope, message string) (*ScopeApplication, error) {
	if err := s.requireCapability(ctx, CapabilityScopeApply); err != nil {
		return nil, err
	}
	if !slices.Contains(grantableScopes, scope) {
		return nil, ErrScopeNotGrantable
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, ErrScopeAppMessage
	}
	if utf8.RuneCountInString(message) > maxScopeAppMessageLen {
		return nil, ErrScopeAppMsgTooLong
	}

	existing, err := s.repo.FindScopeApplication(ctx, userID, scope)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		switch existing.Status {
		case ScopeAppPending:
			return nil, ErrScopeAppPending
		case ScopeAppApproved:
			return nil, ErrScopeAppApproved
		}
		fields := map[string]any{
			"message":        message,
			"status":         ScopeAppPending,
			"reviewer_id":    nil,
			"reviewed_at":    nil,
			"decline_reason": "",
		}
		if err := s.repo.UpdateScopeApplicationFields(ctx, existing.ID, fields); err != nil {
			return nil, err
		}
		return s.repo.GetScopeApplication(ctx, existing.ID)
	}

	app := &ScopeApplication{
		UserID:  userID,
		Scope:   scope,
		Message: message,
		Status:  ScopeAppPending,
	}
	if err := s.repo.CreateScopeApplication(ctx, app); err != nil {
		return nil, err
	}
	slog.Info("devapi scope application filed", "user_id", userID, "scope", scope, "id", app.ID)
	return app, nil
}

func (s *SelfServiceService) ListScopeApplications(ctx context.Context, userID uint) ([]ScopeApplication, error) {
	return s.repo.ListScopeApplicationsByUser(ctx, userID)
}

func (s *AdminService) ListScopeApplications(ctx context.Context, status string) ([]ScopeApplication, error) {
	return s.repo.ListScopeApplications(ctx, status)
}

func (s *AdminService) ApproveScopeApplication(ctx context.Context, id, reviewerID uint) (*ScopeApplication, error) {
	return s.reviewScopeApplication(ctx, id, reviewerID, ScopeAppApproved, "")
}

func (s *AdminService) DeclineScopeApplication(ctx context.Context, id, reviewerID uint, reason string) (*ScopeApplication, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrScopeAppNeedsReason
	}
	if utf8.RuneCountInString(reason) > maxScopeAppMessageLen {
		return nil, ErrScopeAppMsgTooLong
	}
	return s.reviewScopeApplication(ctx, id, reviewerID, ScopeAppDeclined, reason)
}

func (s *AdminService) reviewScopeApplication(ctx context.Context, id, reviewerID uint, status, reason string) (*ScopeApplication, error) {
	app, err := s.repo.GetScopeApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	if app.Status != ScopeAppPending {
		return nil, ErrScopeAppNotPending
	}
	fields := map[string]any{
		"status":         status,
		"reviewer_id":    reviewerID,
		"reviewed_at":    time.Now(),
		"decline_reason": reason,
	}
	if err := s.repo.UpdateScopeApplicationFields(ctx, app.ID, fields); err != nil {
		return nil, err
	}
	slog.Info("devapi scope application reviewed",
		"id", app.ID, "user_id", app.UserID, "scope", app.Scope, "status", status, "by", reviewerID)
	return s.repo.GetScopeApplication(ctx, app.ID)
}
