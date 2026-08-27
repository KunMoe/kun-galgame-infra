package handler

import (
	"context"
	"net/http"
	"strconv"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catsvc "api/internal/platform/catalog/service"

	"github.com/danielgtaylor/huma/v2"
)

type listClaimEventsInput struct {
	Cursor       string `query:"cursor" maxLength:"512" doc:"Opaque keyset cursor from a prior next_cursor. Must start with cur_."`
	Limit        string `query:"limit" maxLength:"8" doc:"Page size 1-100, default 20. Values above 100 are 400 LIMIT_TOO_LARGE, not clamped."`
	View         string `query:"view" maxLength:"16" doc:"basic (default) or full. Closed vocabulary."`
	Fields       string `query:"fields" maxLength:"1024" doc:"Comma-separated top-level keys. Unknown token is 400 UNKNOWN_FIELD. object and id are always kept."`
	IDs          string `query:"ids" maxLength:"4096" doc:"Comma-separated event ids, max 100. Batch lane: no pagination."`
	IncludeTotal string `query:"include_total" maxLength:"8" doc:"true to include total. Only true or false."`
	Sort         string `query:"sort" maxLength:"32" doc:"recorded_desc (default) or recorded_asc. Closed. recorded_asc is the watermark walk a mirror reads."`
	Site         string `query:"site" maxLength:"64" doc:"Tenant key the event was recorded under. Open vocabulary; unknown values match nothing."`
	ActorUID     string `query:"actor_uid" maxLength:"20" doc:"The claiming site's own user id of the actor. Not a catalog id."`
	WorkID       string `query:"work_id" maxLength:"20" doc:"Catalog work id to narrow to one work's claim history."`
}

type listClaimEventsOutput struct {
	Body repr.List[repr.ClaimEvent]
}

func registerCatalogClaimEvents(api huma.API, cat *Catalog) {
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogClaimEvents", Method: http.MethodGet, Path: "/v2/catalog/claim-events",
		Summary:     "Claim event history",
		Description: "Every claim lifecycle transition, newest first by default. sort=recorded_asc walks the same collection oldest-first by id, which is the shape a mirror or a reward cron reads with a watermark. Requires an application key with the claim_events:read scope on top of catalog:read; the scope is granted by an operator, not self-service, because events carry decline reasons and moderator actions.",
		Tags:        []string{"catalog"}, Errors: collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable),
		SkipValidateParams: true,
	}, listCatalogClaimEvents(cat))
}

func listCatalogClaimEvents(cat *Catalog) func(context.Context, *listClaimEventsInput) (*listClaimEventsOutput, error) {
	return func(ctx context.Context, in *listClaimEventsInput) (*listClaimEventsOutput, error) {
		if in == nil {
			in = &listClaimEventsInput{}
		}
		q, perr := collect.Parse(collect.Raw{
			Cursor: in.Cursor, Limit: in.Limit, View: in.View, Fields: in.Fields,
			IDs: in.IDs, IncludeTotal: in.IncludeTotal, Sort: in.Sort,
		}, collect.ClaimEventSpec())
		if perr != nil {
			return nil, withIdent(ctx, perr)
		}
		page, lerr := cat.ListClaimEvents(ctx, q, in.Site, in.ActorUID, in.WorkID)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listClaimEventsOutput{Body: page}, nil
	}
}

func (c *Catalog) ListClaimEvents(ctx context.Context, q collect.Query, site, actorUID, workID string) (repr.List[repr.ClaimEvent], error) {
	if c == nil || c.Claims == nil {
		return repr.List[repr.ClaimEvent]{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	page := catsvc.ClaimEventPage{
		Site: site, Ascending: q.Sort == "recorded_asc", WithTotal: q.IncludeTotal,
	}
	if actorUID != "" {
		id, ok := repr.ParseID(actorUID)
		if !ok {
			return repr.List[repr.ClaimEvent]{}, badFilterID("actor_uid", actorUID)
		}
		page.ActorUID = id
	}
	if workID != "" {
		id, ok := repr.ParseID(workID)
		if !ok {
			return repr.List[repr.ClaimEvent]{}, badFilterID("work_id", workID)
		}
		page.WorkID = id
	}
	var missing []string
	if q.Batch {
		ids, err := parseIDList(q.IDs)
		if err != nil {
			return repr.List[repr.ClaimEvent]{}, err
		}
		page.IDs = ids
	} else {
		cur, cerr := claimEventCursor(q.Cursor)
		if cerr != nil {
			return repr.List[repr.ClaimEvent]{}, cerr
		}
		page.Cursor = cur
		page.Limit = q.Limit
		if page.Limit <= 0 {
			page.Limit = collect.DefaultLimit
		}
		page.Limit++
	}
	rows, total, err := c.Claims.ClaimEvents(ctx, page)
	if err != nil {
		return repr.List[repr.ClaimEvent]{}, err
	}
	var next *string
	if !q.Batch {
		limit := page.Limit - 1
		if len(rows) > limit {
			rows = rows[:limit]
			s := strconv.FormatInt(rows[len(rows)-1].ID, 10)
			next = &s
		}
	}
	items := make([]repr.ClaimEvent, 0, len(rows))
	found := map[string]bool{}
	for i := range rows {
		items = append(items, claimEventFrom(&rows[i]))
		found[repr.ID(rows[i].ID)] = true
	}
	if q.Batch {
		for _, want := range q.IDs {
			if !found[want] {
				missing = append(missing, want)
			}
		}
	}
	return finishList(items, next, total, q, missing), nil
}

func claimEventFrom(r *catsvc.ClaimEventItem) repr.ClaimEvent {
	return repr.ClaimEvent{
		Object: "claim_event", ID: repr.ID(r.ID), WorkID: repr.ID(r.WorkID),
		FromState: r.FromState, ToState: r.ToState, Reason: r.Reason,
		ActorUID: repr.ID(r.ActorUID), Site: r.Site,
		ProductWorkID: idPtr(r.ProductWorkID), CreatedAt: rfc3339(r.CreatedAt),
	}
}
