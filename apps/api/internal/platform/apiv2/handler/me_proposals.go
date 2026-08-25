package handler

import (
	"context"
	"errors"
	"strconv"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/authz"
	catalogPerm "api/internal/platform/catalog/perm"
	"api/internal/platform/editing"
)

func (c *Catalog) policyActor(ctx context.Context) (editing.PolicyContext, error) {
	uid, _, err := requireUser(ctx)
	if err != nil {
		return editing.PolicyContext{}, err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return editing.PolicyContext{}, err
	}
	roles := rolesFrom(ctx)
	return editing.PolicyContext{
		UserID: uid, Site: site,
		HasPerm: func(key string) bool {
			return catalogPerm.Resolver.Can(roles, authz.Permission(key))
		},
	}, nil
}

func proposalFrom(p *editing.Proposal) repr.ProposalRecord {
	st := editing.StatusName[p.Status]
	if st == "" {
		st = "open"
	}
	rec := repr.ProposalRecord{
		Object: "proposal", ID: repr.ID(p.ID), State: st,
		TargetObject: schemaObject(p.EntityType),
		EntityType:   p.EntityType, EntityID: repr.ID(p.EntityID), Note: p.Note,
		ProposerUID: repr.ID(p.ProposerUID), Site: p.Site,
		BaseRevisionSeq: p.BaseRevisionSeq,
		DecidedByUID:    idPtr(p.DecidedByUID),
		CreatedAt:       rfc3339(p.CreatedAt), UpdatedAt: rfc3339(p.UpdatedAt),
	}
	if p.DecidedAt != nil {
		at := rfc3339(*p.DecidedAt)
		rec.DecidedAt = &at
	}
	return rec
}

// The decision note is the reviewer's internal reasoning and is the one field
// the v1 proposal view carries that the public transparency face must not.
func (c *Catalog) proposalDetail(ctx context.Context, id int64, include []string) (repr.ProposalRecord, string, error) {
	prop, amendments, effective, err := c.Engine.GetProposal(ctx, id)
	if err != nil {
		return repr.ProposalRecord{}, "", proposalErr(err)
	}
	rec := proposalFrom(prop)
	if hasToken(include, "amendments") {
		list := amendmentsFrom(amendments)
		rec.Amendments = &list
	}
	if hasToken(include, "patch") {
		patch := decodeJSONObject(prop.Patch)
		rec.Patch = &patch
		if effective == nil {
			effective = map[string]any{}
		}
		rec.EffectivePatch = &effective
	}
	return rec, proposalETag(prop), nil
}

func proposalETag(p *editing.Proposal) string {
	return `"p` + repr.ID(p.ID) + "." + strconv.FormatInt(p.UpdatedAt.Unix(), 10) + `"`
}

func (c *Catalog) ListMyProposals(ctx context.Context, q collect.Query, state string) (repr.List[repr.ProposalRecord], error) {
	if c == nil || c.Engine == nil {
		return repr.List[repr.ProposalRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.List[repr.ProposalRecord]{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	before, berr := claimEventCursor(q.Cursor)
	if berr != nil {
		return repr.List[repr.ProposalRecord]{}, berr
	}
	f := editing.ProposalFilter{ProposerUID: uid, Limit: limit + 1, Status: -1, BeforeID: before}
	if site := siteFrom(ctx); site != "" {
		f.Site = site
	}
	if st, ok := proposalStateValue(state); ok {
		f.Status = st
	}
	rows, total, lerr := c.Engine.ListProposalsWithTotal(ctx, f)
	if lerr != nil {
		return repr.List[repr.ProposalRecord]{}, lerr
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		s := strconv.FormatInt(rows[len(rows)-1].ID, 10)
		next = &s
	}
	items := make([]repr.ProposalRecord, 0, len(rows))
	for i := range rows {
		items = append(items, proposalFrom(&rows[i]))
	}
	return finishList(items, next, total, q, nil), nil
}

func (c *Catalog) GetMyProposal(ctx context.Context, id int64, include []string) (repr.ProposalRecord, string, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, "", problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.ProposalRecord{}, "", err
	}
	rec, etag, derr := c.proposalDetail(ctx, id, include)
	if derr != nil {
		return repr.ProposalRecord{}, "", derr
	}
	if rec.ProposerUID != repr.ID(uid) {
		return repr.ProposalRecord{}, "", problem.New(problem.CodeNotFound, "", "", "No proposal with this id.")
	}
	return rec, etag, nil
}

func (c *Catalog) GetModerationProposal(ctx context.Context, id int64, include []string) (repr.ProposalRecord, string, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, "", problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.ProposalRecord{}, "", err
	}
	site, serr := requireSite(ctx)
	if serr != nil {
		return repr.ProposalRecord{}, "", serr
	}
	rec, etag, derr := c.proposalDetail(ctx, id, include)
	if derr != nil {
		return repr.ProposalRecord{}, "", derr
	}
	if rec.Site != site {
		return repr.ProposalRecord{}, "", problem.New(problem.CodeTenantMismatch, "", "",
			"this proposal was filed on another catalog site.")
	}
	return rec, etag, nil
}

func (c *Catalog) CreateProposal(ctx context.Context, entityType, entityID string, patch map[string]any, note string) (repr.ProposalRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	actor, err := c.policyActor(ctx)
	if err != nil {
		return repr.ProposalRecord{}, err
	}
	eid, ok := repr.ParseID(entityID)
	if !ok {
		p := problem.New(problem.CodeValidationFailed, "", "", "entity_id must be a decimal catalog id.")
		p.Errors = []problem.FieldError{{Pointer: "/entity_id", Reason: problem.ReasonInvalidFormat, Detail: entityID}}
		return repr.ProposalRecord{}, p
	}
	prop, _, cerr := c.Engine.CreateProposal(ctx, editing.CreateProposalInput{
		EntityType: entityType, EntityID: eid, Patch: patch, Note: note, Actor: actor,
	})
	if cerr != nil {
		return repr.ProposalRecord{}, proposalErr(cerr)
	}
	return proposalFrom(prop), nil
}

func (c *Catalog) PatchProposal(ctx context.Context, id int64, state string, patch map[string]any, ifMatch string) (repr.ProposalRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	actor, err := c.policyActor(ctx)
	if err != nil {
		return repr.ProposalRecord{}, err
	}
	prop, _, _, gerr := c.Engine.GetProposal(ctx, id)
	if gerr != nil {
		return repr.ProposalRecord{}, proposalErr(gerr)
	}
	if err := requireIfMatch(ifMatch, proposalETag(prop)); err != nil {
		return repr.ProposalRecord{}, err
	}
	if state == "withdrawn" {
		if werr := c.Engine.WithdrawProposal(ctx, id, actor); werr != nil {
			return repr.ProposalRecord{}, proposalErr(werr)
		}
	} else if len(patch) > 0 {
		if _, aerr := c.Engine.AmendProposal(ctx, id, editing.AmendInput{Set: patch, Actor: actor}); aerr != nil {
			return repr.ProposalRecord{}, proposalErr(aerr)
		}
	} else {
		return repr.ProposalRecord{}, problem.New(problem.CodeValidationFailed, "", "", "patch must withdraw or amend.")
	}
	prop, _, _, gerr = c.Engine.GetProposal(ctx, id)
	if gerr != nil {
		return repr.ProposalRecord{}, proposalErr(gerr)
	}
	return proposalFrom(prop), nil
}

func (c *Catalog) AmendProposal(ctx context.Context, id int64, set map[string]any, unset []string, note, ifMatch string) (repr.ProposalRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	actor, err := c.policyActor(ctx)
	if err != nil {
		return repr.ProposalRecord{}, err
	}
	prop, _, _, gerr := c.Engine.GetProposal(ctx, id)
	if gerr != nil {
		return repr.ProposalRecord{}, proposalErr(gerr)
	}
	if err := requireIfMatch(ifMatch, proposalETag(prop)); err != nil {
		return repr.ProposalRecord{}, err
	}
	if _, aerr := c.Engine.AmendProposal(ctx, id, editing.AmendInput{Set: set, Unset: unset, Note: note, Actor: actor}); aerr != nil {
		return repr.ProposalRecord{}, proposalErr(aerr)
	}
	prop, _, _, gerr = c.Engine.GetProposal(ctx, id)
	if gerr != nil {
		return repr.ProposalRecord{}, proposalErr(gerr)
	}
	return proposalFrom(prop), nil
}

func (c *Catalog) ListModerationProposals(ctx context.Context, q collect.Query) (repr.List[repr.ProposalRecord], error) {
	if c == nil || c.Engine == nil {
		return repr.List[repr.ProposalRecord]{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.List[repr.ProposalRecord]{}, err
	}
	site, err := requireSite(ctx)
	if err != nil {
		return repr.List[repr.ProposalRecord]{}, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	before, berr := claimEventCursor(q.Cursor)
	if berr != nil {
		return repr.List[repr.ProposalRecord]{}, berr
	}
	f := editing.ProposalFilter{Status: editing.StatusOpen, Limit: limit + 1, Site: site, BeforeID: before}
	rows, total, lerr := c.Engine.ListProposalsWithTotal(ctx, f)
	if lerr != nil {
		return repr.List[repr.ProposalRecord]{}, lerr
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		s := strconv.FormatInt(rows[len(rows)-1].ID, 10)
		next = &s
	}
	items := make([]repr.ProposalRecord, 0, len(rows))
	for i := range rows {
		items = append(items, proposalFrom(&rows[i]))
	}
	return finishList(items, next, total, q, nil), nil
}

func (c *Catalog) DecideProposal(ctx context.Context, id int64, decision, note, ifMatch string) (repr.DecisionRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.DecisionRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	actor, err := c.policyActor(ctx)
	if err != nil {
		return repr.DecisionRecord{}, err
	}
	prop, _, _, gerr := c.Engine.GetProposal(ctx, id)
	if gerr != nil {
		return repr.DecisionRecord{}, proposalErr(gerr)
	}
	if err := requireIfMatch(ifMatch, proposalETag(prop)); err != nil {
		return repr.DecisionRecord{}, err
	}
	switch decision {
	case "merge":
		if _, merr := c.Engine.MergeProposal(ctx, id, actor, note); merr != nil {
			return repr.DecisionRecord{}, proposalErr(merr)
		}
	case "decline":
		if derr := c.Engine.DeclineProposal(ctx, id, actor, note); derr != nil {
			return repr.DecisionRecord{}, proposalErr(derr)
		}
	default:
		p := problem.New(problem.CodeValidationFailed, "", "", "decision must be merge or decline.")
		p.Errors = []problem.FieldError{{Pointer: "/decision", Reason: problem.ReasonUnknownValue, Detail: "merge or decline"}}
		return repr.DecisionRecord{}, p
	}
	return repr.DecisionRecord{Object: "decision", ID: repr.ID(id), Decision: decision, Note: note}, nil
}

func (c *Catalog) RevertRevision(ctx context.Context, revisionID, reason string) (repr.ProposalRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.ProposalRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "proposals are not bound.")
	}
	actor, err := c.policyActor(ctx)
	if err != nil {
		return repr.ProposalRecord{}, err
	}
	rid, ok := repr.ParseID(revisionID)
	if !ok {
		p := problem.New(problem.CodeValidationFailed, "", "", "revision_id must be a decimal id.")
		p.Errors = []problem.FieldError{{Pointer: "/revision_id", Reason: problem.ReasonInvalidFormat, Detail: revisionID}}
		return repr.ProposalRecord{}, p
	}
	rev, rerr := c.Engine.RevisionByID(ctx, rid)
	if rerr != nil {
		return repr.ProposalRecord{}, proposalErr(rerr)
	}
	prop, _, verr := c.Engine.Revert(ctx, editing.RevertInput{
		EntityType: rev.EntityType, EntityID: rev.EntityID, ToSeq: rev.Seq, Note: reason, Actor: actor,
	})
	if verr != nil {
		return repr.ProposalRecord{}, proposalErr(verr)
	}
	return proposalFrom(prop), nil
}

func (c *Catalog) GetSnapshot(ctx context.Context, object, id string) (repr.SnapshotRecord, error) {
	if c == nil || c.Engine == nil {
		return repr.SnapshotRecord{}, problem.New(problem.CodeServiceUnavailable, "", "", "snapshots are not bound.")
	}
	if _, _, err := requireUser(ctx); err != nil {
		return repr.SnapshotRecord{}, err
	}
	entityType := schemaEntityType(object)
	if entityType == "" {
		return repr.SnapshotRecord{}, problem.New(problem.CodeNotFound, "", "", "No schema for family "+object+".")
	}
	eid, ok := repr.ParseID(id)
	if !ok {
		return repr.SnapshotRecord{}, problemInvalidID(id)
	}
	vals, err := c.Engine.CurrentSnapshot(ctx, entityType, eid)
	if err != nil {
		return repr.SnapshotRecord{}, proposalErr(err)
	}
	if vals == nil {
		vals = map[string]any{}
	}
	return repr.SnapshotRecord{Object: "snapshot", EntityType: entityType, EntityID: repr.ID(eid), FieldValues: vals}, nil
}

func proposalStateValue(state string) (int16, bool) {
	switch state {
	case "pending", "open":
		return editing.StatusOpen, true
	case "merged":
		return editing.StatusMerged, true
	case "declined":
		return editing.StatusDeclined, true
	case "withdrawn":
		return editing.StatusWithdrawn, true
	default:
		return -1, false
	}
}

func proposalErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, editing.ErrProposalNotFound), errors.Is(err, editing.ErrRevisionNotFound),
		errors.Is(err, editing.ErrUnknownEntityType), errors.Is(err, editing.ErrEntityNotFound):
		return problem.New(problem.CodeNotFound, "", "", err.Error())
	case errors.Is(err, editing.ErrEmptyPatch), errors.Is(err, editing.ErrEmptyDelta):
		p := problem.New(problem.CodeValidationFailed, "", "", err.Error())
		p.Errors = []problem.FieldError{{Pointer: "/patch", Reason: problem.ReasonRequired, Detail: err.Error()}}
		return p
	case errors.Is(err, editing.ErrNoEffectiveChanges):
		p := problem.New(problem.CodeValidationFailed, "", "", err.Error())
		p.Errors = []problem.FieldError{{Pointer: "/patch", Reason: problem.ReasonNotAllowedValue, Detail: err.Error()}}
		return p
	// A trusted proposer's edit auto-merges on creation, so the very next decide
	// lands on a closed proposal. Unmapped this surfaced as a 500.
	case errors.Is(err, editing.ErrNotOpen):
		return problem.New(problem.CodeDecisionAlreadyMade, "", "", err.Error())
	case errors.Is(err, editing.ErrNotProposer):
		return problem.New(problem.CodePermissionRequired, "", "", err.Error())
	}
	var conflict *editing.ConflictError
	if errors.As(err, &conflict) {
		p := problem.New(problem.CodeValidationFailed, "", "", conflict.Error())
		for _, key := range conflict.Keys {
			p.Errors = append(p.Errors, problem.FieldError{
				Pointer: "/patch/" + key, Reason: problem.ReasonInconsistentWith,
				Detail: "another revision changed this key since the proposal was written",
			})
		}
		return p
	}
	var unknownField *editing.UnknownFieldError
	if errors.As(err, &unknownField) {
		p := problem.New(problem.CodeValidationFailed, "", "", unknownField.Error())
		p.Errors = []problem.FieldError{{Pointer: "/patch/" + unknownField.Key, Reason: problem.ReasonUnknownValue, Detail: unknownField.Error()}}
		return p
	}
	var lockedField *editing.LockedFieldError
	if errors.As(err, &lockedField) {
		p := problem.New(problem.CodeValidationFailed, "", "", lockedField.Error())
		p.Errors = []problem.FieldError{{Pointer: "/patch/" + lockedField.Key, Reason: problem.ReasonImmutable, Detail: lockedField.Error()}}
		return p
	}
	var perm *editing.PermissionError
	if errors.As(err, &perm) {
		return problem.New(problem.CodePermissionRequired, "", "", perm.Error())
	}
	var val *editing.ValidationError
	if errors.As(err, &val) {
		p := problem.New(problem.CodeValidationFailed, "", "", val.Error())
		p.Errors = []problem.FieldError{{Pointer: "/" + val.Key, Reason: problem.ReasonUnknownValue, Detail: val.Error()}}
		return p
	}
	return err
}
