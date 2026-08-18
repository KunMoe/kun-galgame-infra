package handler

import (
	"context"
	"log/slog"
	"net/http"

	"api/internal/platform/catalog/dto"
	catperm "api/internal/platform/catalog/perm"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
)

type UserClaimServer struct {
	claims *service.ClaimLifecycleService
}

func RegisterUserClaimOps(api huma.API, lifecycle *service.ClaimLifecycleService) {
	s := &UserClaimServer{claims: lifecycle}
	tags := []string{"catalog-user"}

	huma.Register(api, huma.Operation{
		OperationID: "submitCatalogWorkUser", Method: http.MethodPost,
		Path:    UserPrefix + "/works/submit",
		Summary: "Submit a work for review AS THE BEARER TOKEN'S OWN USER: mints it in the pending claim state (registry row + content + birth event, one transaction) and stamps the submitter as its owner. The submitting tenant is the token client's catalog site and the submitter is the token's user; the body names neither. product_work_id is OPTIONAL — omit it and the registry issues the identity, the claim adopting the minted work id (returned as product_work_id). IDEMPOTENCY is the S2S op's: with product_work_id a repeat is a 409 echoing the existing work (matched_by=claim); without it a repeat is recognized only by the identity anchors the payload's links assert (matched_by=anchor), and a submission carrying neither WILL mint a second work if retried",
		Tags:    tags,
	}, s.submitWork)
	huma.Register(api, huma.Operation{
		OperationID: "actOnCatalogClaimUser", Method: http.MethodPost,
		Path:    UserPrefix + "/works/{id}/claim-actions/{action}",
		Summary: "Move a claim through its lifecycle AS THE BEARER TOKEN'S OWN USER: claim / submit / publish / withdraw (the token's user must be the entry's owner — or its FIRST CLAIMANT when the entry is unowned, in which case the action adopts it: this is how a person claims one of the machine-imported drafts) or approve / decline / ban / unban (the token's roles must carry catalog.claim.review). 409 on an illegal transition, echoing the current state; 403 on ANOTHER user's claim or another tenant's",
		Tags:    tags,
	}, s.act)
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogClaimsMine", Method: http.MethodGet,
		Path:    UserPrefix + "/claims/mine",
		Summary: "The claims the BEARER TOKEN'S OWN USER has acted on, on the token client's catalog site: current state, latest transition and reason, most recent activity first (cursor: before=last_event_id). The total is the per-user statistic — 'published by me' is this call with claim_state=live&limit=1",
		Tags:    tags,
	}, s.mine)
}

type userSubmitWorkInput struct {
	Body dto.UserWorkSubmitRequest
}

func (s *UserClaimServer) submitWork(ctx context.Context, in *userSubmitWorkInput) (*submitWorkOutput, error) {
	uid, site, he := userActor(ctx)
	if he != nil {
		return nil, he
	}
	trusted := catperm.Resolver.Can(userRolesFromCtx(ctx), catperm.EditTrusted) &&
		!isThirdPartyClient(clientFromCtx(ctx))
	params := service.SubmitWorkParams{
		Site: site, ActorUID: uid, Fields: in.Body.Fields, Trusted: trusted,
	}
	if id := in.Body.ProductWorkID; id != nil {
		params.ProductWorkID = *id
	}
	if d := in.Body.Released; d != nil {
		params.Released = service.ReleaseDate{Y: d.Y, M: d.M, D: d.D}
	}
	res, err := s.claims.SubmitWork(ctx, params)
	if err != nil {
		return nil, submitErr(err)
	}
	return &submitWorkOutput{Body: okEnvelope(dto.WorkSubmitResponse{
		WorkID: res.WorkID, ProductWorkID: res.ProductWorkID, ClaimState: res.ClaimState,
		EventID: res.EventID, ReleaseID: res.ReleaseID,
	})}, nil
}

type userClaimActionInput struct {
	ID     int64  `path:"id" minimum:"1"`
	Action string `path:"action" doc:"claim | submit | publish | withdraw | approve | decline | ban | unban"`
	Body   dto.UserClaimActionRequest
}

func (s *UserClaimServer) act(ctx context.Context, in *userClaimActionInput) (*claimActionOutput, error) {
	action := service.ClaimAction(in.Action)
	if _, known := service.TransitionRule(action); !known {
		return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam, "unknown claim action "+in.Action)
	}
	uid, site, he := userActor(ctx)
	if he != nil {
		return nil, he
	}
	review := service.ReviewActions[action]
	if review {
		if isThirdPartyClient(clientFromCtx(ctx)) {
			return nil, apiErrMsg(http.StatusForbidden, errors.ErrForbidden,
				"a third-party application is not a moderation surface; claim review needs a first-party site client")
		}
		if !catperm.Resolver.Can(userRolesFromCtx(ctx), catperm.ClaimReview) {
			return nil, apiErrMsg(http.StatusForbidden, errors.ErrForbidden,
				"reviewing a claim requires the "+string(catperm.ClaimReview)+" permission")
		}
	}
	res, err := s.claims.Act(ctx, service.ClaimActionParams{
		WorkID: in.ID, Action: action,
		Site:          site,
		ProductWorkID: in.Body.ProductWorkID,
		ActorUID:      uid,
		Reason:        in.Body.Reason,
		RequireOwner:  !review,
	})
	if err != nil {
		return nil, claimErr(err)
	}
	return &claimActionOutput{Body: okEnvelope(*res)}, nil
}

type userMineClaimsInput struct {
	ClaimState string `query:"claim_state" doc:"Comma-separated subset of none, live, draft, pending, declined, hidden; absent = every state"`
	Before     int64  `query:"before" doc:"Exclusive cursor: return works whose last_event_id is smaller (0 = first page)"`
	Limit      int    `query:"limit" doc:"Page size (default 20, max 100)"`
	Kind       string `query:"kind" doc:"Which events qualify the actor: submitted (works they own) or audited (works they reviewed but do not own). Absent = every work they touched"`
}

func (s *UserClaimServer) mine(ctx context.Context, in *userMineClaimsInput) (*userClaimsOutput, error) {
	uid, site, he := userActor(ctx)
	if he != nil {
		return nil, he
	}
	claimStates, ok := claimStatesPub(in.ClaimState)
	if !ok {
		return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam, msgBadClaimState)
	}
	items, total, err := s.claims.ClaimsByActor(ctx, service.UserClaimQuery{
		ActorUID: uid, Site: site, ClaimStates: claimStates,
		Before: in.Before, Limit: in.Limit, Kind: in.Kind,
	})
	if err != nil {
		slog.Error("catalog claims mine", "err", err)
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	page := dto.CursorPage[service.UserClaimItem]{
		Items: make([]service.UserClaimItem, 0, len(items)), Total: total,
	}
	page.Items = append(page.Items, items...)
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if len(items) >= limit {
		page.NextBefore = items[len(items)-1].LastEventID
	}
	return &userClaimsOutput{Body: okEnvelope(page)}, nil
}
