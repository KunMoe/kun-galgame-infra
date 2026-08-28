package handler

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/editspec"
	catmodel "api/internal/platform/catalog/model"
	catalogPerm "api/internal/platform/catalog/perm"
	catsvc "api/internal/platform/catalog/service"

	"gorm.io/gorm"
)

type myClaimFilter struct {
	ClaimStates []string
	Kind        string
	Site        string
}

var myClaimKinds = []string{"submitted", "audited", "all"}

func parseMyClaimFilter(claimState, kind, site string) (myClaimFilter, *problem.Problem) {
	var f myClaimFilter
	states, err := closedCSV(claimState, "claim_state", []string{
		catmodel.ClaimStateKeyNone, catmodel.ClaimStateKeyLive, catmodel.ClaimStateKeyDraft,
		catmodel.ClaimStateKeyPending, catmodel.ClaimStateKeyDeclined, catmodel.ClaimStateKeyHidden,
	})
	if err != nil {
		return myClaimFilter{}, err
	}
	f.ClaimStates = states
	switch k := strings.TrimSpace(kind); k {
	case "", "submitted":
		f.Kind = "submitted"
	case "audited":
		f.Kind = "audited"
	case "all":
		f.Kind = ""
	default:
		return myClaimFilter{}, closedParam("kind", strings.Join(myClaimKinds, ", "))
	}
	f.Site = strings.TrimSpace(site)
	return f, nil
}

func (c *Catalog) ListMyClaims(ctx context.Context, q collect.Query, f myClaimFilter) (repr.List[repr.ClaimRecord], error) {
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
		ActorUID: uid, Kind: f.Kind, ClaimStates: f.ClaimStates, Site: f.Site,
		Limit: limit + 1, Before: before,
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

func requireClaimReview(ctx context.Context) (string, error) {
	if _, _, err := requireUser(ctx); err != nil {
		return "", err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return "", err
	}
	if !catalogPerm.Resolver.Can(rolesFrom(ctx), catalogPerm.ClaimReview) {
		return "", problem.New(problem.CodePermissionRequired, "", "",
			"this moderation face requires the "+string(catalogPerm.ClaimReview)+" permission.")
	}
	return site, nil
}

func (c *Catalog) ListModerationClaims(ctx context.Context, q collect.Query) (repr.List[repr.ClaimRecord], error) {
	if c == nil || c.Claims == nil {
		return repr.List[repr.ClaimRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	site, err := requireClaimReview(ctx)
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
		rec := repr.ClaimRecord{
			Object: "claim", ID: repr.ID(r.WorkID), State: "pending", DisplayName: r.DisplayName,
			ProductWorkID: idPtr(r.ProductWorkID),
		}
		if r.Site != nil {
			rec.Site = *r.Site
		}
		items = append(items, rec)
	}
	return finishList(items, next, total, q, nil), nil
}

func (c *Catalog) GetModerationClaim(ctx context.Context, workID int64) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	site, err := requireClaimReview(ctx)
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
	case catmodel.ClaimStateKeyNone, catmodel.ClaimStateKeyLive, catmodel.ClaimStateKeyDraft,
		catmodel.ClaimStateKeyPending, catmodel.ClaimStateKeyDeclined, catmodel.ClaimStateKeyHidden:
	default:
		st = catmodel.ClaimStateKeyHidden
	}
	rec := repr.ClaimRecord{
		Object: "claim", ID: repr.ID(r.WorkID), State: st, DisplayName: r.DisplayName,
		Site: r.Site, ProductWorkID: idPtr(r.ProductWorkID),
	}
	if r.LastEventID > 0 {
		rec.LastEvent = &repr.ClaimEventRef{
			Object: "claim_event", ID: repr.ID(r.LastEventID),
			FromState: r.LastFromState, ToState: r.LastToState, Reason: r.LastReason,
			ActorUID: repr.ID(r.LastActorUID), CreatedAt: rfc3339(r.LastEventAt),
		}
		first := rfc3339(r.FirstActedAt)
		rec.FirstActedAt = &first
		acted := r.ActedCount
		rec.ActedCount = &acted
	}
	return rec
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

func (c *Catalog) CreateClaim(ctx context.Context, workID, siteWorkID, displayName string, refs []repr.Ref, fields map[string]any) (repr.ClaimRecord, error) {
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
		if len(fields) > 0 {
			p := problem.New(problem.CodeValidationFailed, "", "",
				"field_values apply to a mint; changing an existing work is a proposal (POST /v2/me/proposals).")
			p.Errors = []problem.FieldError{{Pointer: "/field_values", Reason: problem.ReasonNotAllowedValue,
				Detail: "work_id and field_values cannot be sent together"}}
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
				// Claiming the match is the behaviour without field_values. With
				// them the caller asserted content for a work they believe is
				// new, and claiming would drop the whole map with a 201.
				if len(fields) > 0 {
					return repr.ClaimRecord{}, problem.New(problem.CodeAlreadyExists, "", "",
						r.Source+":"+r.ExternalID+" already belongs to work "+repr.ID(id)+
							"; claim it without field_values, or propose an edit against it.")
				}
				return c.actClaim(ctx, uid, site, id, siteWorkID)
			}
		}
		mint, perr := claimMintFields(displayName, fields, "display_name is required to mint a work when refs do not match.")
		if perr != nil {
			return repr.ClaimRecord{}, perr
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
		unionWorkLinks(mint, links)
		crefs := make([]catsvc.ClaimRef, 0, len(refs))
		for _, r := range refs {
			crefs = append(crefs, catsvc.ClaimRef{Source: r.Source, ExternalID: r.ExternalID})
		}
		product := int64(0)
		if siteWorkID != "" {
			if n, ok := repr.ParseID(siteWorkID); ok {
				product = n
			}
		}
		return c.mintClaim(ctx, site, uid, product, crefs, mint)
	}
	if siteWorkID != "" || len(fields) > 0 {
		detail := "display_name is required to mint a work from field_values alone."
		product := int64(0)
		if siteWorkID != "" {
			detail = "display_name is required to mint a work from site_work_id alone."
			n, ok := repr.ParseID(siteWorkID)
			if !ok {
				p := problem.New(problem.CodeValidationFailed, "", "", "site_work_id must be a decimal id.")
				p.Errors = []problem.FieldError{{Pointer: "/site_work_id", Reason: problem.ReasonInvalidFormat, Detail: siteWorkID}}
				return repr.ClaimRecord{}, p
			}
			product = n
		}
		mint, perr := claimMintFields(displayName, fields, detail)
		if perr != nil {
			return repr.ClaimRecord{}, perr
		}
		return c.mintClaim(ctx, site, uid, product, nil, mint)
	}
	p := problem.New(problem.CodeValidationFailed, "", "", "work_id, refs, site_work_id, or field_values is required.")
	p.Errors = []problem.FieldError{{Pointer: "/work_id", Reason: problem.ReasonRequired, Detail: "provide work_id, refs, site_work_id, or field_values"}}
	return repr.ClaimRecord{}, p
}

func (c *Catalog) DeleteClaim(ctx context.Context, workID int64) error {
	if c == nil || c.Claims == nil {
		return problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return err
	}
	if derr := c.Claims.DeleteDraft(ctx, workID, uid); derr != nil {
		return claimWriteErr(derr)
	}
	return nil
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

// draft is the v1 withdraw target and stayed accepted alongside the spec's
// withdrawn; both name the same executor action.
var meClaimTransitions = []struct {
	Token  string
	Action catsvc.ClaimAction
}{
	{"live", catsvc.ClaimActionPublish},
	{"pending", catsvc.ClaimActionSubmit},
	{"withdrawn", catsvc.ClaimActionWithdraw},
	{"draft", catsvc.ClaimActionWithdraw},
}

func meClaimAction(state string) (catsvc.ClaimAction, bool) {
	for _, t := range meClaimTransitions {
		if t.Token == state {
			return t.Action, true
		}
	}
	return "", false
}

func allowedClaimTargets(current string) []string {
	var out []string
	for _, t := range meClaimTransitions {
		if t.Token == "draft" {
			continue
		}
		from, known := catsvc.TransitionRule(t.Action)
		if !known {
			continue
		}
		if slices.Contains(from, current) {
			out = append(out, t.Token)
		}
	}
	return out
}

func claimTransitionProblem(current, target string) *problem.Problem {
	allowed := allowedClaimTargets(current)
	detail := "this claim is in state " + current + "; "
	if len(allowed) == 0 {
		detail += "no state transition is available to its owner from here"
	} else {
		detail += "the owner may move it to " + strings.Join(allowed, " or ")
	}
	p := problem.New(problem.CodeInvalidStateTransition, "", "", detail+", not "+target+".")
	p.Errors = []problem.FieldError{{Pointer: "/state", Reason: problem.ReasonNotAllowedValue, Detail: detail}}
	return p
}

func (c *Catalog) claimForWrite(ctx context.Context, workID int64, site string) (repr.ClaimRecord, error) {
	row, err := c.Claims.ClaimByWorkID(ctx, workID, site)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	if row == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeNotFound, "", "", "No claim with this id.")
	}
	return claimRecordFrom(*row), nil
}

// The pre-read is site-fenced, not owner-fenced: v1's executor lets the FIRST
// claimant adopt an unowned machine-imported draft, and gating on ownership here
// made adopt-and-publish — the highest-frequency claim write — 404 instead.
func (c *Catalog) PatchClaim(ctx context.Context, workID int64, state, ifMatch string) (repr.ClaimRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.ClaimRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.ClaimRecord{}, err
	}
	site, serr := requireSite(ctx)
	if serr != nil {
		return repr.ClaimRecord{}, serr
	}
	cur, gerr := c.claimForWrite(ctx, workID, site)
	if gerr != nil {
		return repr.ClaimRecord{}, gerr
	}
	if err := requireIfMatch(ifMatch, claimETag(cur)); err != nil {
		return repr.ClaimRecord{}, err
	}
	action, known := meClaimAction(state)
	if !known {
		p := problem.New(problem.CodeValidationFailed, "", "", "state is not a claim transition the owner may request.")
		p.Errors = []problem.FieldError{{Pointer: "/state", Reason: problem.ReasonUnknownValue,
			Detail: "expected one of: live, pending, withdrawn"}}
		return repr.ClaimRecord{}, p
	}
	if from, ok := catsvc.TransitionRule(action); ok && !slices.Contains(from, cur.State) {
		return repr.ClaimRecord{}, claimTransitionProblem(cur.State, state)
	}
	if _, aerr := c.Claims.Act(ctx, catsvc.ClaimActionParams{
		WorkID: workID, Action: action, Site: site, ActorUID: uid, RequireOwner: true,
	}); aerr != nil {
		return repr.ClaimRecord{}, claimWriteErr(aerr)
	}
	return c.claimForWrite(ctx, workID, site)
}

func (c *Catalog) DecideClaim(ctx context.Context, workID int64, decision, note, ifMatch string) (repr.DecisionRecord, error) {
	if c == nil || c.Claims == nil {
		return repr.DecisionRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "claims are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.DecisionRecord{}, err
	}
	site, serr := requireClaimReview(ctx)
	if serr != nil {
		return repr.DecisionRecord{}, serr
	}
	cur, gerr := c.claimForWrite(ctx, workID, site)
	if gerr != nil {
		return repr.DecisionRecord{}, gerr
	}
	if err := requireIfMatch(ifMatch, claimETag(cur)); err != nil {
		return repr.DecisionRecord{}, err
	}
	action, known := moderationClaimAction(decision)
	if !known {
		p := problem.New(problem.CodeValidationFailed, "", "", "decision must be approve, decline, ban, or unban.")
		p.Errors = []problem.FieldError{{Pointer: "/decision", Reason: problem.ReasonUnknownValue,
			Detail: "expected one of: approve, decline, ban, unban"}}
		return repr.DecisionRecord{}, p
	}
	if from, ok := catsvc.TransitionRule(action); ok && !slices.Contains(from, cur.State) {
		p := problem.New(problem.CodeInvalidStateTransition, "", "",
			"this claim is in state "+cur.State+"; "+decision+" applies to "+strings.Join(from, " or ")+".")
		p.Errors = []problem.FieldError{{Pointer: "/decision", Reason: problem.ReasonNotAllowedValue,
			Detail: "claim state is " + cur.State}}
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

func moderationClaimAction(decision string) (catsvc.ClaimAction, bool) {
	switch decision {
	case "approve":
		return catsvc.ClaimActionApprove, true
	case "decline":
		return catsvc.ClaimActionDecline, true
	case "ban":
		return catsvc.ClaimActionBan, true
	case "unban":
		return catsvc.ClaimActionUnban, true
	default:
		return "", false
	}
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
	var notOwned *catsvc.ClaimNotOwnedError
	if errors.As(err, &notOwned) {
		return problem.New(problem.CodeClaimNotOwned, "", "",
			"this claim belongs to another user; only its owner may move it.")
	}
	var otherSite *catsvc.ClaimOwnershipError
	if errors.As(err, &otherSite) {
		return problem.New(problem.CodeTenantMismatch, "", "", otherSite.Error())
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return problem.New(problem.CodeNotFound, "", "", "No claim with this id.")
	}
	var field *editspec.SubmissionFieldError
	if errors.As(err, &field) {
		reason := problem.ReasonUnknownValue
		if field.Err != nil {
			reason = problem.ReasonInvalidFormat
		}
		p := problem.New(problem.CodeValidationFailed, "", "", field.Error())
		p.Errors = []problem.FieldError{{Pointer: "/field_values/" + field.Field, Reason: reason, Detail: field.Error()}}
		return p
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
