package handler

import (
	"context"
	"errors"
	"strconv"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/editspec"
	catmodel "api/internal/platform/catalog/model"
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
	before, berr := claimEventCursor(q.Cursor)
	if berr != nil {
		return repr.List[repr.ClaimRecord]{}, berr
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	rows, total, lerr := c.Claims.ClaimsByActor(ctx, catsvc.UserClaimQuery{
		ActorUID: uid, Kind: "submitted", Limit: limit + 1, Before: before,
	})
	if lerr != nil {
		return repr.List[repr.ClaimRecord]{}, lerr
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		s := strconv.FormatInt(rows[len(rows)-1].LastEventID, 10)
		next = &s
	}
	items := make([]repr.ClaimRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, claimRecordFrom(r))
	}
	return finishList(items, next, total, q, nil), nil
}

func (c *Catalog) ListModerationClaims(ctx context.Context, q collect.Query) (repr.List[repr.ClaimRecord], error) {
	if c == nil || c.Claims == nil {
		return repr.List[repr.ClaimRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.List[repr.ClaimRecord]{}, err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return repr.List[repr.ClaimRecord]{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	before, berr := claimEventCursor(q.Cursor)
	if berr != nil {
		return repr.List[repr.ClaimRecord]{}, berr
	}
	rows, total, lerr := c.Claims.PendingClaimsAfter(ctx, site, limit+1, before)
	if lerr != nil {
		return repr.List[repr.ClaimRecord]{}, lerr
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		if last := rows[len(rows)-1].SubmittedEventID; last != nil {
			s := strconv.FormatInt(*last, 10)
			next = &s
		}
	}
	items := make([]repr.ClaimRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, repr.ClaimRecord{
			Object: "claim", ID: repr.ID(r.WorkID), State: "pending", DisplayName: r.DisplayName,
		})
	}
	return finishList(items, next, total, q, nil), nil
}

func (c *Catalog) GetModerationClaim(ctx context.Context, workID int64) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.ClaimRecord{}, err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	row, lerr := c.Claims.ClaimByWorkID(ctx, workID, site)
	if lerr != nil {
		return repr.ClaimRecord{}, lerr
	}
	if row == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeNotFound, "", "", "No claim with this id.")
	}
	return claimRecordFrom(*row), nil
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

func (c *Catalog) CreateClaim(ctx context.Context, workID, siteWorkID, displayName string, refs []repr.Ref) (repr.ClaimRecord, error) {
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
	if workID != "" {
		wid, ok := repr.ParseID(workID)
		if !ok {
			p := problem.New(problem.CodeValidationFailed, "", "", "work_id must be a decimal catalog id.")
			p.Errors = []problem.FieldError{{Pointer: "/work_id", Reason: problem.ReasonInvalidFormat, Detail: workID}}
			return repr.ClaimRecord{}, p
		}
		return c.actClaim(ctx, uid, site, wid, siteWorkID)
	}
	if len(refs) > 0 {
		for i, r := range refs {
			if r.Source == "" || r.ExternalID == "" {
				p := problem.New(problem.CodeValidationFailed, "", "", "each ref needs source and external_id.")
				p.Errors = []problem.FieldError{{Pointer: "/refs/" + strconv.Itoa(i) + "/source", Reason: problem.ReasonRequired, Detail: "source:external_id"}}
				return repr.ClaimRecord{}, p
			}
			if c.Public == nil {
				continue
			}
			id, lerr := c.Public.LookupEntityID(ctx, r.Source, r.ExternalID, catmodel.EntityTypeWork)
			if lerr != nil {
				return repr.ClaimRecord{}, lerr
			}
			if id != 0 {
				return c.actClaim(ctx, uid, site, id, siteWorkID)
			}
		}
		if displayName == "" {
			p := problem.New(problem.CodeValidationFailed, "", "", "display_name is required to mint a work when refs do not match.")
			p.Errors = []problem.FieldError{{Pointer: "/display_name", Reason: problem.ReasonRequired, Detail: "SubmitWork requires catalog.work.display_name"}}
			return repr.ClaimRecord{}, p
		}
		links := make([]any, 0, len(refs))
		for i, r := range refs {
			u, ok := editspec.WorkLinkURL(r.Source, r.ExternalID)
			if !ok {
				p := problem.New(problem.CodeValidationFailed, "", "", "ref cannot be turned into a work link.")
				p.Errors = []problem.FieldError{{Pointer: "/refs/" + strconv.Itoa(i) + "/source", Reason: problem.ReasonInvalidFormat, Detail: r.Source + ":" + r.ExternalID}}
				return repr.ClaimRecord{}, p
			}
			links = append(links, u)
		}
		fields := map[string]any{
			editspec.FieldWorkDisplayName: displayName,
			editspec.FieldWorkLinks:       links,
		}
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
	p := problem.New(problem.CodeValidationFailed, "", "", "work_id or refs is required.")
	p.Errors = []problem.FieldError{{Pointer: "/work_id", Reason: problem.ReasonRequired, Detail: "provide work_id or refs"}}
	return repr.ClaimRecord{}, p
}

func (c *Catalog) actClaim(ctx context.Context, uid int64, site string, workID int64, siteWorkID string) (repr.ClaimRecord, error) {
	var product *int64
	if siteWorkID != "" {
		n, ok := repr.ParseID(siteWorkID)
		if !ok {
			p := problem.New(problem.CodeValidationFailed, "", "", "site_work_id must be a decimal id.")
			p.Errors = []problem.FieldError{{Pointer: "/site_work_id", Reason: problem.ReasonInvalidFormat, Detail: siteWorkID}}
			return repr.ClaimRecord{}, p
		}
		product = &n
	}
	res, aerr := c.Claims.Act(ctx, catsvc.ClaimActionParams{
		WorkID: workID, Action: catsvc.ClaimActionClaim, Site: site, ProductWorkID: product, ActorUID: uid,
	})
	if aerr != nil {
		return repr.ClaimRecord{}, claimWriteErr(aerr)
	}
	return c.GetMyClaim(ctx, res.WorkID)
}

func claimEventCursor(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, collectInvalidCursor()
	}
	return n, nil
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
	site, serr := requireSite(ctx)
	if serr != nil {
		return repr.DecisionRecord{}, serr
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
		WorkID: workID, Action: action, Site: site, ActorUID: uid, Reason: note,
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
