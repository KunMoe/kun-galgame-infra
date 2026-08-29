package devapi

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	siteModel "api/internal/platform/site/model"
)

const (
	AppReviewApproved = "approved"
	AppReviewPending  = "pending"
	AppReviewDeclined = "declined"
)

// Counted in runes, not bytes: these notes are written in Chinese, where a byte
// cap of 2000 is really ~666 characters — a limit the UI and the error text both
// promise as 2000.
const maxAppReviewNoteLen = 2000

const (
	AppFilterEnabled  = "enabled"
	AppFilterPending  = "pending"
	AppFilterDeclined = "declined"
	AppFilterDisabled = "disabled"
	AppFilterArchived = "archived"
	AppFilterAll      = "all"
)

var (
	ErrAppReviewNotPending  = errors.New("devapi: only a pending application can be reviewed")
	ErrAppReviewNeedsReason = errors.New("devapi: a decline needs a reason")
	ErrAppReviewNoteTooLong = errors.New("devapi: decline reason too long (max 2000)")
	ErrAppNotDeclined       = errors.New("devapi: only a declined application can be resubmitted")
	ErrAppNotApproved       = errors.New("devapi: application has not cleared review")
)

// A row whose status is the empty string counts as approved: the columns
// arrived on 2026-08-18 and the migration backfilled every existing row, but a
// client created by the OAuth console (which does not know about them) still
// inserts the zero value, and those first-party clients must keep working. The
// gate is therefore written against the two blocking states, never as
// "not approved".
func appAwaitsReview(status string) bool {
	return status == AppReviewPending || status == AppReviewDeclined
}

// Archived rows are excluded from every filter except their own and "all":
// an application its owner deleted is not pending review, not disabled pending
// a decision, and not a live application — leaving it in the working tabs would
// put rows in the operator's queue that nobody is waiting on.
func (r *Repository) ListAppsByFilter(ctx context.Context, filter string) ([]siteModel.OAuthClient, error) {
	q := r.db.WithContext(ctx)
	switch filter {
	case AppFilterPending:
		q = q.Where("dev_review_status = ? AND dev_archived_at IS NULL", AppReviewPending)
	case AppFilterDeclined:
		q = q.Where("dev_review_status = ? AND dev_archived_at IS NULL", AppReviewDeclined)
	case AppFilterDisabled:
		q = q.Where("dev_enabled = ? AND dev_archived_at IS NULL AND dev_review_status NOT IN ?",
			false, []string{AppReviewPending, AppReviewDeclined})
	case AppFilterArchived:
		q = q.Where("dev_archived_at IS NOT NULL")
	case AppFilterAll:
	default:
		q = q.Where("dev_enabled = ? AND dev_archived_at IS NULL", true)
	}
	var apps []siteModel.OAuthClient
	err := q.Order("created_at DESC, name ASC").Find(&apps).Error
	return apps, err
}

func (s *AdminService) ApproveApp(ctx context.Context, clientID string, reviewerID uint) (*siteModel.OAuthClient, error) {
	return s.reviewApp(ctx, clientID, reviewerID, AppReviewApproved, "")
}

func (s *AdminService) DeclineApp(ctx context.Context, clientID string, reviewerID uint, reason string) (*siteModel.OAuthClient, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrAppReviewNeedsReason
	}
	if utf8.RuneCountInString(reason) > maxAppReviewNoteLen {
		return nil, ErrAppReviewNoteTooLong
	}
	return s.reviewApp(ctx, clientID, reviewerID, AppReviewDeclined, reason)
}

func (s *AdminService) reviewApp(ctx context.Context, clientID string, reviewerID uint, status, note string) (*siteModel.OAuthClient, error) {
	app, err := s.repo.GetApp(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if app.DevReviewStatus != AppReviewPending {
		return nil, ErrAppReviewNotPending
	}
	fields := map[string]any{
		"dev_review_status": status,
		"dev_review_note":   note,
		"dev_enabled":       status == AppReviewApproved,
	}
	if err := s.repo.UpdateAppFields(ctx, app.ID, fields); err != nil {
		return nil, err
	}
	slog.Info("devapi application reviewed",
		"client_id", app.ID, "owner", app.OwnerUserID, "status", status, "by", reviewerID)
	return s.repo.GetApp(ctx, app.ID)
}

func (s *SelfServiceService) ResubmitApp(ctx context.Context, ownerUserID uint, clientID string) (*siteModel.OAuthClient, error) {
	app, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if app.DevReviewStatus != AppReviewDeclined {
		return nil, ErrAppNotDeclined
	}
	fields := map[string]any{
		"dev_review_status": AppReviewPending,
		"dev_review_note":   "",
	}
	if err := s.repo.UpdateAppFields(ctx, app.ID, fields); err != nil {
		return nil, err
	}
	slog.Info("devapi application resubmitted", "client_id", app.ID, "owner", ownerUserID)
	return s.repo.GetApp(ctx, app.ID)
}
