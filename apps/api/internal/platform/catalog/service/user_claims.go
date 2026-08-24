package service

import (
	"context"
	"time"

	"api/internal/platform/catalog/model"
)

type UserClaimItem struct {
	WorkID        int64     `json:"work_id"`
	DisplayName   string    `json:"display_name"`
	Site          string    `json:"site"`
	ProductWorkID *int64    `json:"product_work_id"`
	ClaimState    string    `json:"claim_state"`
	LastEventID   int64     `json:"last_event_id"`
	LastFromState *string   `json:"last_from_state"`
	LastToState   string    `json:"last_to_state"`
	LastReason    *string   `json:"last_reason"`
	LastActorUID  int64     `json:"last_actor_uid"`
	LastEventAt   time.Time `json:"last_event_at"`
	FirstActedAt  time.Time `json:"first_acted_at"`
	ActedCount    int       `json:"acted_count"`
}

type UserClaimQuery struct {
	ActorUID    int64
	Site        string
	ClaimStates []string
	Before      int64
	WorkID      int64
	Limit       int
	// Kind narrows which events qualify the actor: "submitted" keeps only the
	// works the actor owns (their own submissions), "audited" keeps only the
	// works the actor reviewed but does not own. Empty = every work the actor
	// touched (the historical behaviour).
	Kind string
}

func (s *ClaimLifecycleService) ClaimsByActor(ctx context.Context, q UserClaimQuery) ([]UserClaimItem, int64, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 101 {
		limit = 101
	}
	stateGate, stateArgs := claimStateWhere(q.ClaimStates)
	if stateGate != "" {
		stateGate = " AND " + stateGate
	}
	ownerGate := ""
	var ownerArgs []any
	switch q.Kind {
	case "submitted":
		ownerGate = " AND w.owner_user_id = ?"
		ownerArgs = []any{q.ActorUID}
	case "audited":
		ownerGate = " AND w.owner_user_id IS DISTINCT FROM ?"
		ownerArgs = []any{q.ActorUID}
	}
	if q.WorkID > 0 {
		ownerGate += " AND w.id = ?"
		ownerArgs = append(ownerArgs, q.WorkID)
	}

	const from = `
		FROM (
		    SELECT e.work_id,
		           min(e.created_at) AS first_acted_at,
		           count(*)          AS acted_count
		    FROM catalog_claim_event e
		    WHERE e.actor_uid = ? AND (? = '' OR e.site = ?)
		    GROUP BY e.work_id
		) m
		JOIN catalog_work w ON w.id = m.work_id AND w.deleted_at IS NULL`

	baseArgs := func() []any {
		args := []any{q.ActorUID, q.Site, q.Site}
		args = append(args, stateArgs...)
		return append(args, ownerArgs...)
	}

	var total int64
	if err := s.db.WithContext(ctx).
		Raw(`SELECT count(*)`+from+` WHERE true`+stateGate+ownerGate, baseArgs()...).
		Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var rows []struct {
		WorkID        int64
		DisplayName   string
		Site          *string
		ProductWorkID *int64
		ClaimState    *int16
		LastEventID   int64
		FromState     *int16
		ToState       int16
		Reason        *string
		LastActorUID  int64
		LastEventAt   time.Time
		FirstActedAt  time.Time
		ActedCount    int
	}
	args := append(baseArgs(), q.Before, q.Before, limit)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT w.id AS work_id, w.display_name, w.site, w.product_work_id, w.claim_state,
		       m.first_acted_at, m.acted_count,
		       le.id AS last_event_id, le.from_state, le.to_state, le.reason,
		       le.actor_uid AS last_actor_uid, le.created_at AS last_event_at`+
		from+`
		JOIN LATERAL (
		    SELECT le.id, le.from_state, le.to_state, le.reason, le.actor_uid, le.created_at
		    FROM catalog_claim_event le
		    WHERE le.work_id = m.work_id
		    ORDER BY le.id DESC
		    LIMIT 1
		) le ON true
		WHERE true`+stateGate+ownerGate+`
		  AND (? <= 0 OR le.id < ?)
		ORDER BY le.id DESC
		LIMIT ?`, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]UserClaimItem, 0, len(rows))
	for _, r := range rows {
		item := UserClaimItem{
			WorkID: r.WorkID, DisplayName: r.DisplayName,
			ProductWorkID: r.ProductWorkID,
			ClaimState:    model.ClaimStateKey(r.Site, r.ProductWorkID, r.ClaimState),
			LastEventID:   r.LastEventID, LastToState: stateKeyOf(r.ToState),
			LastReason: r.Reason, LastActorUID: r.LastActorUID, LastEventAt: r.LastEventAt,
			FirstActedAt: r.FirstActedAt, ActedCount: r.ActedCount,
		}
		if r.Site != nil {
			item.Site = *r.Site
		}
		if r.FromState != nil {
			from := stateKeyOf(*r.FromState)
			item.LastFromState = &from
		}
		out = append(out, item)
	}
	return out, total, nil
}
