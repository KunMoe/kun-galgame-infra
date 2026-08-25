package service

import (
	"context"
	stderrors "errors"
	"time"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPlaytimeActorRequired   = stderrors.New("catalog: a playtime report requires a reporting user")
	ErrPlaytimeClientRequired  = stderrors.New("catalog: a playtime report requires a client id")
	ErrPlaytimeWorkUnavailable = stderrors.New("catalog: work not available for playtime reporting")
	ErrPlaytimeMinutesRange    = stderrors.New("catalog: minutes must be between 0 and 60000")
	ErrPlaytimeBadStatus       = stderrors.New("catalog: unknown playtime status")
	ErrPlaytimeUnknownSource   = stderrors.New("catalog: unknown external source key")
	ErrPlaytimeRefUnresolved   = stderrors.New("catalog: no work is anchored to that external id")
)

type UserPlaytimeService struct{ db *gorm.DB }

func NewUserPlaytimeService(db *gorm.DB) *UserPlaytimeService {
	return &UserPlaytimeService{db: db}
}

type PlaytimeReport struct {
	ActorUID     int64
	WorkID       int64
	ClientID     string
	Minutes      int
	Status       int16
	LastPlayedAt *time.Time
}

type PlaytimeRecord struct {
	WorkID       int64
	Minutes      int
	Status       int16
	LastPlayedAt *time.Time
	ClientID     string
	UpdatedAt    time.Time
}

func (s *UserPlaytimeService) Report(ctx context.Context, r PlaytimeReport) (*PlaytimeRecord, error) {
	if err := validateReport(r); err != nil {
		return nil, err
	}
	row := model.CatalogUserPlaytime{
		ActorUID: r.ActorUID, WorkID: r.WorkID, ClientID: r.ClientID,
		Minutes: r.Minutes, Status: r.Status, LastPlayedAt: r.LastPlayedAt,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := assertReportableWork(tx, r.WorkID); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "actor_uid"}, {Name: "work_id"}, {Name: "client_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"minutes":        r.Minutes,
				"status":         r.Status,
				"last_played_at": r.LastPlayedAt,
				"updated_at":     time.Now(),
			}),
		}).Create(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &PlaytimeRecord{
		WorkID: row.WorkID, Minutes: row.Minutes, Status: row.Status,
		LastPlayedAt: row.LastPlayedAt, ClientID: row.ClientID, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *UserPlaytimeService) ResolveRef(ctx context.Context, sourceKey, externalID string) (int64, error) {
	var sourceID int16
	err := s.db.WithContext(ctx).Raw(
		`SELECT id FROM catalog_source WHERE key = ?`, sourceKey).Scan(&sourceID).Error
	if err != nil {
		return 0, err
	}
	if sourceID == 0 {
		return 0, ErrPlaytimeUnknownSource
	}
	var workID int64
	err = s.db.WithContext(ctx).Raw(`
		SELECT entity_id FROM catalog_external_ref
		 WHERE entity_type = ? AND source_id = ? AND external_id = ? AND link_kind = 0
		 ORDER BY id DESC LIMIT 1`,
		model.EntityTypeWork, sourceID, externalID).Scan(&workID).Error
	if err != nil {
		return 0, err
	}
	if workID == 0 {
		return 0, ErrPlaytimeRefUnresolved
	}
	return workID, nil
}

func (s *UserPlaytimeService) ListMine(ctx context.Context, uid int64, since time.Time, limit int) ([]PlaytimeRecord, error) {
	if uid <= 0 {
		return nil, ErrPlaytimeActorRequired
	}
	q := s.db.WithContext(ctx).Model(&model.CatalogUserPlaytime{}).Where("actor_uid = ?", uid)
	if !since.IsZero() {
		q = q.Where("updated_at > ?", since)
	}
	var rows []model.CatalogUserPlaytime
	if err := q.Order("updated_at ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PlaytimeRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, PlaytimeRecord{
			WorkID: r.WorkID, Minutes: r.Minutes, Status: r.Status,
			LastPlayedAt: r.LastPlayedAt, ClientID: r.ClientID, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

type UserWorkPlaytime struct {
	WorkID       int64
	Minutes      int
	Status       int16
	LastPlayedAt *time.Time
	Clients      int
}

func (s *UserPlaytimeService) DeleteMine(ctx context.Context, uid, workID int64) error {
	if uid <= 0 {
		return ErrPlaytimeActorRequired
	}
	if workID <= 0 {
		return ErrPlaytimeWorkUnavailable
	}
	return s.db.WithContext(ctx).
		Where("actor_uid = ? AND work_id = ?", uid, workID).
		Delete(&model.CatalogUserPlaytime{}).Error
}

func (s *UserPlaytimeService) GetMine(ctx context.Context, uid, workID int64) (*UserWorkPlaytime, error) {
	if uid <= 0 {
		return nil, ErrPlaytimeActorRequired
	}
	var rows []model.CatalogUserPlaytime
	if err := s.db.WithContext(ctx).
		Where("actor_uid = ? AND work_id = ?", uid, workID).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := UserWorkPlaytime{WorkID: workID, Clients: len(rows)}
	best := 0
	finished := false
	for i, r := range rows {
		if r.Minutes > rows[best].Minutes {
			best = i
		}
		if r.Status == model.PlaytimeStatusFinished {
			finished = true
		}
		if r.LastPlayedAt != nil && (out.LastPlayedAt == nil || r.LastPlayedAt.After(*out.LastPlayedAt)) {
			out.LastPlayedAt = r.LastPlayedAt
		}
	}
	out.Minutes = rows[best].Minutes
	out.Status = rows[best].Status
	if finished {
		out.Status = model.PlaytimeStatusFinished
	}
	return &out, nil
}

func validateReport(r PlaytimeReport) error {
	if r.ActorUID <= 0 {
		return ErrPlaytimeActorRequired
	}
	if r.ClientID == "" {
		return ErrPlaytimeClientRequired
	}
	if r.Minutes < 0 || r.Minutes > model.PlaytimeMinutesMax {
		return ErrPlaytimeMinutesRange
	}
	switch r.Status {
	case model.PlaytimeStatusPlaying, model.PlaytimeStatusFinished,
		model.PlaytimeStatusDropped, model.PlaytimeStatusOnHold:
	default:
		return ErrPlaytimeBadStatus
	}
	return nil
}

func assertReportableWork(tx *gorm.DB, workID int64) error {
	var work model.CatalogWork
	err := tx.Select("status").Where("id = ?", workID).Take(&work).Error
	switch {
	case stderrors.Is(err, gorm.ErrRecordNotFound):
		return ErrPlaytimeWorkUnavailable
	case err != nil:
		return err
	case work.Status != model.WorkStatusLive:
		return ErrPlaytimeWorkUnavailable
	}
	return nil
}
