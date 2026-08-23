package handler

import (
	"context"
	"errors"

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
	site := siteFrom(ctx)
	rows, total, lerr := c.Claims.PendingClaims(ctx, site, limit)
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

func claimRecordFrom(r catsvc.UserClaimItem) repr.ClaimRecord {
	st := r.ClaimState
	switch st {
	case "live", "draft", "pending", "declined", "hidden":
	default:
		st = "hidden"
	}
	return repr.ClaimRecord{Object: "claim", ID: repr.ID(r.WorkID), State: st, DisplayName: r.DisplayName}
}

func (c *Catalog) GetMyClaim(ctx context.Context, workID int64) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	rows, _, lerr := c.Claims.ClaimsByActor(ctx, catsvc.UserClaimQuery{
		ActorUID: uid, Kind: "submitted", WorkID: workID, Limit: 1,
	})
	if lerr != nil {
		return repr.ClaimRecord{}, lerr
	}
	if len(rows) == 0 {
		return repr.ClaimRecord{}, problem.New(problem.CodeNotFound, "", "", "No claim with this id.")
	}
	return claimRecordFrom(rows[0]), nil
}

func (c *Catalog) CreateClaim(ctx context.Context, workID, siteWorkID, displayName string) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	if workID == "" && displayName == "" {
		p := problem.New(problem.CodeValidationFailed, "", "", "work_id or display_name is required.")
		p.Errors = []problem.FieldError{{Pointer: "/work_id", Reason: problem.ReasonRequired, Detail: "provide work_id or display_name to mint"}}
		return repr.ClaimRecord{}, p
	}
	if workID != "" {
		wid, ok := repr.ParseID(workID)
		if !ok {
			p := problem.New(problem.CodeValidationFailed, "", "", "work_id must be a decimal catalog id.")
			p.Errors = []problem.FieldError{{Pointer: "/work_id", Reason: problem.ReasonInvalidFormat, Detail: workID}}
			return repr.ClaimRecord{}, p
		}
		var product *int64
		if siteWorkID != "" {
			n, pok := repr.ParseID(siteWorkID)
			if !pok {
				p := problem.New(problem.CodeValidationFailed, "", "", "site_work_id must be a decimal id.")
				p.Errors = []problem.FieldError{{Pointer: "/site_work_id", Reason: problem.ReasonInvalidFormat, Detail: siteWorkID}}
				return repr.ClaimRecord{}, p
			}
			product = &n
		}
		res, aerr := c.Claims.Act(ctx, catsvc.ClaimActionParams{
			WorkID: wid, Action: catsvc.ClaimActionClaim, Site: site, ProductWorkID: product, ActorUID: uid,
		})
		if aerr != nil {
			return repr.ClaimRecord{}, claimWriteErr(aerr)
		}
		return c.GetMyClaim(ctx, res.WorkID)
	}
	fields := map[string]any{"catalog.work.display_name": displayName}
	product := int64(0)
	if siteWorkID != "" {
		if n, ok := repr.ParseID(siteWorkID); ok {
			product = n
		}
	}
	res, serr := c.Claims.SubmitWork(ctx, catsvc.SubmitWorkParams{
		Site: site, ProductWorkID: product, ActorUID: uid, Fields: fields,
	})
	if serr != nil {
		return repr.ClaimRecord{}, claimWriteErr(serr)
	}
	return repr.ClaimRecord{Object: "claim", ID: repr.ID(res.WorkID), State: res.ClaimState}, nil
}

func (c *Catalog) PatchClaim(ctx context.Context, workID int64, state, ifMatch string) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	cur, gerr := c.GetMyClaim(ctx, workID)
	if gerr != nil {
		return repr.ClaimRecord{}, gerr
	}
	if err := requireIfMatch(ifMatch, claimETag(cur)); err != nil {
		return repr.ClaimRecord{}, err
	}
	if state != "withdrawn" && state != "draft" {
		return repr.ClaimRecord{}, problem.New(problem.CodeInvalidStateTransition, "", "", "this patch only withdraws a claim.")
	}
	_, aerr := c.Claims.Act(ctx, catsvc.ClaimActionParams{
		WorkID: workID, Action: catsvc.ClaimActionWithdraw, Site: siteFrom(ctx), ActorUID: uid, RequireOwner: true,
	})
	if aerr != nil {
		return repr.ClaimRecord{}, claimWriteErr(aerr)
	}
	return c.GetMyClaim(ctx, workID)
}

func (c *Catalog) DecideClaim(ctx context.Context, workID int64, decision, note, ifMatch string) (repr.DecisionRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.DecisionRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.DecisionRecord{}, err
	}
	if err := requireIfMatch(ifMatch, `"`+"c"+repr.ID(workID)+`"`); err != nil {
		return repr.DecisionRecord{}, err
	}
	var action catsvc.ClaimAction
	switch decision {
	case "approve":
		action = catsvc.ClaimActionApprove
	case "decline":
		action = catsvc.ClaimActionDecline
	default:
		p := problem.New(problem.CodeValidationFailed, "", "", "decision must be approve or decline.")
		p.Errors = []problem.FieldError{{Pointer: "/decision", Reason: problem.ReasonUnknownValue, Detail: "approve or decline"}}
		return repr.DecisionRecord{}, p
	}
	res, aerr := c.Claims.Act(ctx, catsvc.ClaimActionParams{
		WorkID: workID, Action: action, Site: siteFrom(ctx), ActorUID: uid, Reason: note,
	})
	if aerr != nil {
		return repr.DecisionRecord{}, claimWriteErr(aerr)
	}
	return repr.DecisionRecord{Object: "decision", ID: repr.ID(res.EventID), Decision: decision, Note: note}, nil
}

func claimETag(rec repr.ClaimRecord) string {
	return `"` + "c" + rec.ID + "." + rec.State + `"`
}

func claimWriteErr(err error) error {
	if err == nil {
		return nil
	}
	var exists *catsvc.ClaimExistsError
	if errors.As(err, &exists) {
		return problem.New(problem.CodeAlreadyExists, "", "", exists.Error())
	}
	var trans *catsvc.ClaimTransitionError
	if errors.As(err, &trans) {
		return problem.New(problem.CodeInvalidStateTransition, "", "", trans.Error())
	}
	switch {
	case errors.Is(err, catsvc.ErrClaimReasonRequired), errors.Is(err, catsvc.ErrSubmitDisplayNameRequired),
		errors.Is(err, catsvc.ErrSubmitTargetRequired):
		p := problem.New(problem.CodeValidationFailed, "", "", err.Error())
		p.Errors = []problem.FieldError{{Pointer: "/", Reason: problem.ReasonRequired, Detail: err.Error()}}
		return p
	}
	return err
}
