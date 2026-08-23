package handler

import (
	"context"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catsvc "api/internal/platform/catalog/service"
)

func (c *Catalog) ListMyClaims(ctx context.Context, q collect.Query) (repr.List[repr.ClaimRecord], error) {
	if c == nil || c.Claims == nil {
		return repr.List[repr.ClaimRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.List[repr.ClaimRecord]{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	rows, total, lerr := c.Claims.ClaimsByActor(ctx, catsvc.UserClaimQuery{
		ActorUID: uid, Kind: "submitted", Limit: limit,
	})
	if lerr != nil {
		return repr.List[repr.ClaimRecord]{}, lerr
	}
	items := make([]repr.ClaimRecord, 0, len(rows))
	for _, r := range rows {
		st := r.ClaimState
		switch st {
		case "live", "draft", "pending", "declined", "hidden":
		default:
			st = "hidden"
		}
		items = append(items, repr.ClaimRecord{
			Object: "claim", ID: repr.ID(r.WorkID), State: st, DisplayName: r.DisplayName,
		})
	}
	return finishList(items, nil, total, q, nil), nil
}

func (c *Catalog) ListModerationClaims(ctx context.Context, q collect.Query) (repr.List[repr.ClaimRecord], error) {
	if c == nil || c.Claims == nil {
		return repr.List[repr.ClaimRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.List[repr.ClaimRecord]{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	rows, total, lerr := c.Claims.PendingClaims(ctx, "", limit)
	if lerr != nil {
		return repr.List[repr.ClaimRecord]{}, lerr
	}
	items := make([]repr.ClaimRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, repr.ClaimRecord{
			Object: "claim", ID: repr.ID(r.WorkID), State: "pending", DisplayName: r.DisplayName,
		})
	}
	return finishList(items, nil, total, q, nil), nil
}
