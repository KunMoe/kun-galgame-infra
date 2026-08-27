package service

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type ClaimEventPage struct {
	Site      string
	ActorUID  int64
	WorkID    int64
	IDs       []int64
	Cursor    int64
	Limit     int
	Ascending bool
	WithTotal bool
}

func (s *ClaimLifecycleService) ClaimEvents(ctx context.Context, p ClaimEventPage) ([]ClaimEventItem, int64, error) {
	base := func() *gorm.DB {
		q := s.db.WithContext(ctx).Table("catalog_claim_event AS e")
		if p.Site != "" {
			q = q.Where("e.site = ?", p.Site)
		}
		if p.ActorUID > 0 {
			q = q.Where("e.actor_uid = ?", p.ActorUID)
		}
		if p.WorkID > 0 {
			q = q.Where("e.work_id = ?", p.WorkID)
		}
		if len(p.IDs) > 0 {
			q = q.Where("e.id IN ?", p.IDs)
		}
		return q
	}

	var total int64
	if p.WithTotal {
		if err := base().Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	q := base().
		Select(`e.id, e.work_id, e.from_state, e.to_state, e.actor_uid, e.reason, e.site,
		        e.created_at, w.product_work_id`).
		Joins("LEFT JOIN catalog_work w ON w.id = e.work_id")
	if len(p.IDs) == 0 {
		if p.Cursor > 0 {
			if p.Ascending {
				q = q.Where("e.id > ?", p.Cursor)
			} else {
				q = q.Where("e.id < ?", p.Cursor)
			}
		}
		if p.Ascending {
			q = q.Order("e.id ASC")
		} else {
			q = q.Order("e.id DESC")
		}
		if p.Limit > 0 {
			q = q.Limit(p.Limit)
		}
	} else {
		q = q.Order("e.id ASC")
	}

	var rows []struct {
		ID            int64
		WorkID        int64
		FromState     *int16
		ToState       int16
		ActorUID      int64
		Reason        *string
		Site          string
		CreatedAt     time.Time
		ProductWorkID *int64
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]ClaimEventItem, 0, len(rows))
	for _, r := range rows {
		item := ClaimEventItem{
			ID: r.ID, WorkID: r.WorkID, ActorUID: r.ActorUID, Reason: r.Reason,
			Site: r.Site, ProductWorkID: r.ProductWorkID, CreatedAt: r.CreatedAt,
			ToState: stateKeyOf(r.ToState),
		}
		if r.FromState != nil {
			from := stateKeyOf(*r.FromState)
			item.FromState = &from
		}
		out = append(out, item)
	}
	return out, total, nil
}
