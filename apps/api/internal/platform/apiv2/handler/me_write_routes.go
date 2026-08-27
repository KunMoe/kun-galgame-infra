package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

func registerMeWrite(api huma.API, cat *Catalog) {
	me := []string{"me"}
	mod := []string{"moderation"}
	errs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable)
	writeErrs := append(errs, http.StatusUnprocessableEntity, http.StatusConflict, http.StatusPreconditionRequired, http.StatusPreconditionFailed)

	huma.Register(api, huma.Operation{
		OperationID: "batchMyPlaytimes", Method: http.MethodPost, Path: "/v2/me/playtimes",
		Summary: "Batch write playtimes", Description: "207 Multi-Status. Each item is a playtime or a problem. Requires a user access token. Any app may call this; playtime:write is not required.",
		Tags: me, Errors: writeErrs, DefaultStatus: 207, SkipValidateParams: true,
	}, batchMyPlaytimes(cat))
	huma.Register(api, huma.Operation{
		OperationID: "createMyClaim", Method: http.MethodPost, Path: "/v2/me/claims",
		Summary: "Submit a claim", Description: "Mint or claim a work. Requires a user access token bound to a catalog site.",
		Tags: me, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, createMyClaim(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getMyClaim", Method: http.MethodGet, Path: "/v2/me/claims/{id}",
		Summary: "Get one of my claims", Description: "id is the catalog work id. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, getMyClaim(cat))
	huma.Register(api, huma.Operation{
		OperationID: "patchMyClaim", Method: http.MethodPatch, Path: "/v2/me/claims/{id}",
		Summary:     "Move a claim the caller owns",
		Description: "PATCH {state: live|pending|withdrawn}. live publishes a draft without review, pending submits it for review, withdrawn returns it to draft. The owner may act, and an unowned claim is adopted by its first claimant. If-Match required. Requires a user access token bound to a catalog site.",
		Tags:        me, Errors: writeErrs, SkipValidateParams: true,
	}, patchMyClaim(cat))
	huma.Register(api, huma.Operation{
		OperationID: "uploadMyEditImage", Method: http.MethodPost, Path: "/v2/me/edit-images",
		Summary:     "Upload an image for an edit proposal",
		Description: "multipart/form-data with preset and file. Returns the hash an edit proposal carries in a cover or screenshot row. Requires a user access token bound to a catalog site.",
		Tags:        me, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, uploadMyEditImage(cat))
	huma.Register(api, huma.Operation{
		OperationID: "listMyProposals", Method: http.MethodGet, Path: "/v2/me/proposals",
		Summary: "List my proposals", Description: "state= filters open/merged/declined/withdrawn. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, listMyProposals(cat))
	huma.Register(api, huma.Operation{
		OperationID: "createMyProposal", Method: http.MethodPost, Path: "/v2/me/proposals",
		Summary: "File a proposal", Description: "Requires a user access token bound to a catalog site.",
		Tags: me, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, createMyProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getMyProposal", Method: http.MethodGet, Path: "/v2/me/proposals/{id}",
		Summary: "Get one of my proposals", Description: "Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, getMyProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "patchMyProposal", Method: http.MethodPatch, Path: "/v2/me/proposals/{id}",
		Summary: "Amend or withdraw a proposal", Description: "If-Match required. Requires a user access token.",
		Tags: me, Errors: writeErrs, SkipValidateParams: true,
	}, patchMyProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "amendMyProposal", Method: http.MethodPost, Path: "/v2/me/proposals/{id}/amendments",
		Summary: "Append an amendment", Description: "If-Match required. Requires a user access token.",
		Tags: me, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, amendMyProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getModerationClaim", Method: http.MethodGet, Path: "/v2/moderation/claims/{id}",
		Summary: "Get one moderation claim", Description: "id is the catalog work id. Site-fenced. Requires a user access token bound to a catalog site.",
		Tags: mod, Errors: errs, SkipValidateParams: true,
	}, getModerationClaim(cat))
	huma.Register(api, huma.Operation{
		OperationID: "decideModerationClaim", Method: http.MethodPost, Path: "/v2/moderation/claims/{id}/decisions",
		Summary:     "Decide a claim",
		Description: "decision=approve|decline|ban|unban. unban restores the state the claim was hidden from. If-Match required, and the ETag comes from GET /v2/moderation/claims/{id}. Requires the catalog.claim.review permission.",
		Tags:        mod, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, decideModerationClaim(cat))
	huma.Register(api, huma.Operation{
		OperationID: "listModerationProposals", Method: http.MethodGet, Path: "/v2/moderation/proposals",
		Summary: "Moderation proposal queue", Description: "Open proposals on the token site.",
		Tags: mod, Errors: errs, SkipValidateParams: true,
	}, listModerationProposals(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getModerationProposal", Method: http.MethodGet, Path: "/v2/moderation/proposals/{id}",
		Summary:     "Get one moderation proposal",
		Description: "Site-fenced. include=patch adds the proposed and effective patches a decision is taken on. The ETag is the validator POST /v2/moderation/proposals/{id}/decisions takes as If-Match.",
		Tags:        mod, Errors: errs, SkipValidateParams: true,
	}, getModerationProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "decideModerationProposal", Method: http.MethodPost, Path: "/v2/moderation/proposals/{id}/decisions",
		Summary: "Decide a proposal", Description: "decision=merge|decline. If-Match required.",
		Tags: mod, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, decideModerationProposal(cat))
	huma.Register(api, huma.Operation{
		OperationID: "revertModeration", Method: http.MethodPost, Path: "/v2/moderation/reverts",
		Summary: "Revert to a revision", Description: "Body names revision_id. Requires a user access token bound to a catalog site.",
		Tags: mod, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, revertModeration(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getModerationSnapshot", Method: http.MethodGet, Path: "/v2/moderation/snapshots/{object}/{id}",
		Summary: "Current edit snapshot", Description: "Registered field values. Requires a user access token.",
		Tags: mod, Errors: errs, SkipValidateParams: true,
	}, getModerationSnapshot(cat))
}

type batchPlaytimesInput struct {
	Body struct {
		Items []struct {
			WorkID  string `json:"work_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
			Minutes int    `json:"minutes" minimum:"0" maximum:"60000" doc:"Absolute cumulative minutes."`
		} `json:"items" doc:"At most 100 items."`
	}
}
type batchPlaytimesOutput struct {
	Status int
	Body   repr.List[repr.PlaytimeBatchItem]
}
type claimRefBody struct {
	Source     string `json:"source" minLength:"1" maxLength:"64" doc:"Open vocabulary source key such as vndb. Must not be used as a discriminant."`
	ExternalID string `json:"external_id" minLength:"1" maxLength:"256" doc:"Verbatim upstream id. Must not be used as a discriminant beyond exact match."`
}

type createClaimInput struct {
	Body struct {
		SiteWorkID  string         `json:"site_work_id,omitempty" pattern:"^[0-9]+$" maxLength:"20" doc:"The site's own work id."`
		WorkID      string         `json:"work_id,omitempty" pattern:"^[0-9]+$" maxLength:"20" doc:"Existing catalog work id to claim."`
		DisplayName string         `json:"display_name,omitempty" maxLength:"512" doc:"Required to mint when refs do not match. Must not be used as a discriminant."`
		Refs        []claimRefBody `json:"refs,omitempty" doc:"source:external_id anchors. Used when work_id is absent."`
	}
}
type getClaimInput struct {
	ID string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
}
type patchClaimInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required."`
	Body    struct {
		State string `json:"state" enum:"live,pending,withdrawn,draft" doc:"live publishes a draft, pending submits it for review, withdrawn returns a live or pending claim to draft. draft is the older spelling of withdrawn."`
	}
}
type getClaimOutput struct {
	ETag string `header:"ETag" doc:"Opaque validator. Send it back as If-Match to move this claim."`
	Body repr.ClaimRecord
}
type listProposalsInput struct {
	CollectionInput
	State string `query:"state" maxLength:"16" doc:"open, pending, merged, declined, withdrawn."`
}
type listProposalsOutput struct {
	Body repr.List[repr.ProposalRecord]
}
type createProposalInput struct {
	Body struct {
		EntityType string         `json:"entity_type" maxLength:"64" doc:"Editing-engine type, e.g. catalog.work. Must not be used as a discriminant."`
		EntityID   string         `json:"entity_id" pattern:"^[0-9]+$" maxLength:"20" doc:"Target catalog id."`
		Patch      map[string]any `json:"patch" doc:"Field-key to new value."`
		Note       string         `json:"note,omitempty" maxLength:"2000" doc:"Must not be used as a discriminant."`
	}
}
type getProposalInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Proposal id."`
	Include string `query:"include" maxLength:"1024" doc:"Comma-separated blocks: amendments, patch. Unknown token is 400 UNKNOWN_INCLUDE."`
	View    string `query:"view" maxLength:"16" doc:"basic (default) or full. full adds amendments and patch."`
}
type getProposalOutput struct {
	ETag string `header:"ETag" doc:"Opaque validator. Send it back as If-Match to amend, withdraw, or decide this proposal."`
	Body repr.ProposalRecord
}
type patchProposalInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Proposal id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required."`
	Body    struct {
		State string         `json:"state,omitempty" enum:"withdrawn" doc:"Set withdrawn to withdraw."`
		Patch map[string]any `json:"patch,omitempty" doc:"Field-key to new value to amend."`
	}
}
type amendProposalInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Proposal id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required."`
	Body    struct {
		Set   map[string]any `json:"set,omitempty" doc:"Field-key to corrected value."`
		Unset []string       `json:"unset,omitempty" doc:"Field keys to drop."`
		Note  string         `json:"note,omitempty" maxLength:"2000" doc:"Must not be used as a discriminant."`
	}
}
type decideClaimInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required."`
	Body    struct {
		Decision string `json:"decision" enum:"approve,decline,ban,unban" doc:"approve publishes a pending claim, decline sends it back, ban hides it from any state, unban restores the state it was hidden from."`
		Note     string `json:"note,omitempty" maxLength:"2000" doc:"Must not be used as a discriminant. Required to decline."`
	}
}
type decideProposalInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Proposal id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required."`
	Body    struct {
		Decision string `json:"decision" enum:"merge,decline" doc:"merge writes the effective patch and records a revision. decline closes the proposal."`
		Note     string `json:"note,omitempty" maxLength:"2000" doc:"Must not be used as a discriminant."`
	}
}
type decideOutput struct{ Body repr.DecisionRecord }
type revertInput struct {
	Body struct {
		RevisionID string `json:"revision_id" pattern:"^[0-9]+$" maxLength:"20" doc:"edit_revision id."`
		Reason     string `json:"reason,omitempty" maxLength:"2000" doc:"Must not be used as a discriminant."`
	}
}
type snapshotInput struct {
	Object string `path:"object" maxLength:"32" doc:"Family: work, company, character, release, tag, engine, series."`
	ID     string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog id."`
}
type snapshotOutput struct{ Body repr.SnapshotRecord }

func batchMyPlaytimes(cat *Catalog) func(context.Context, *batchPlaytimesInput) (*batchPlaytimesOutput, error) {
	return func(ctx context.Context, in *batchPlaytimesInput) (*batchPlaytimesOutput, error) {
		if in == nil {
			in = &batchPlaytimesInput{}
		}
		items := make([]struct {
			WorkID  string
			Minutes int
		}, 0, len(in.Body.Items))
		for _, it := range in.Body.Items {
			items = append(items, struct {
				WorkID  string
				Minutes int
			}{it.WorkID, it.Minutes})
		}
		page, err := cat.BatchPlaytimes(ctx, items)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &batchPlaytimesOutput{Status: 207, Body: page}, nil
	}
}

func createMyClaim(cat *Catalog) func(context.Context, *createClaimInput) (*getClaimOutput, error) {
	return func(ctx context.Context, in *createClaimInput) (*getClaimOutput, error) {
		if in == nil {
			in = &createClaimInput{}
		}
		refs := make([]repr.Ref, 0, len(in.Body.Refs))
		for _, r := range in.Body.Refs {
			refs = append(refs, repr.Ref{Source: r.Source, ExternalID: r.ExternalID})
		}
		rec, err := cat.CreateClaim(ctx, in.Body.WorkID, in.Body.SiteWorkID, in.Body.DisplayName, refs)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getClaimOutput{ETag: claimETag(rec), Body: rec}, nil
	}
}

func getMyClaim(cat *Catalog) func(context.Context, *getClaimInput) (*getClaimOutput, error) {
	return func(ctx context.Context, in *getClaimInput) (*getClaimOutput, error) {
		if in == nil {
			in = &getClaimInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.GetMyClaim(ctx, id)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getClaimOutput{ETag: claimETag(rec), Body: rec}, nil
	}
}

func patchMyClaim(cat *Catalog) func(context.Context, *patchClaimInput) (*getClaimOutput, error) {
	return func(ctx context.Context, in *patchClaimInput) (*getClaimOutput, error) {
		if in == nil {
			in = &patchClaimInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.PatchClaim(ctx, id, in.Body.State, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getClaimOutput{ETag: claimETag(rec), Body: rec}, nil
	}
}

func listMyProposals(cat *Catalog) func(context.Context, *listProposalsInput) (*listProposalsOutput, error) {
	return func(ctx context.Context, in *listProposalsInput) (*listProposalsOutput, error) {
		if in == nil {
			in = &listProposalsInput{}
		}
		q, err := parseCatalogList(ctx, &in.CollectionInput, collect.ClaimSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListMyProposals(ctx, q, in.State)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listProposalsOutput{Body: page}, nil
	}
}

func createMyProposal(cat *Catalog) func(context.Context, *createProposalInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *createProposalInput) (*getProposalOutput, error) {
		if in == nil {
			in = &createProposalInput{}
		}
		rec, err := cat.CreateProposal(ctx, in.Body.EntityType, in.Body.EntityID, in.Body.Patch, in.Body.Note)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{Body: rec}, nil
	}
}

func proposalInclude(ctx context.Context, view, include string) ([]string, error) {
	q, perr := collect.Parse(collect.Raw{View: view, Include: include}, collect.ProposalSpec())
	if perr != nil {
		return nil, withIdent(ctx, perr)
	}
	return q.Include, nil
}

func getMyProposal(cat *Catalog) func(context.Context, *getProposalInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *getProposalInput) (*getProposalOutput, error) {
		if in == nil {
			in = &getProposalInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		include, ierr := proposalInclude(ctx, in.View, in.Include)
		if ierr != nil {
			return nil, ierr
		}
		rec, etag, err := cat.GetMyProposal(ctx, id, include)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{ETag: etag, Body: rec}, nil
	}
}

func getModerationProposal(cat *Catalog) func(context.Context, *getProposalInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *getProposalInput) (*getProposalOutput, error) {
		if in == nil {
			in = &getProposalInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		include, ierr := proposalInclude(ctx, in.View, in.Include)
		if ierr != nil {
			return nil, ierr
		}
		rec, etag, err := cat.GetModerationProposal(ctx, id, include)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{ETag: etag, Body: rec}, nil
	}
}

func patchMyProposal(cat *Catalog) func(context.Context, *patchProposalInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *patchProposalInput) (*getProposalOutput, error) {
		if in == nil {
			in = &patchProposalInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.PatchProposal(ctx, id, in.Body.State, in.Body.Patch, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{Body: rec}, nil
	}
}

func amendMyProposal(cat *Catalog) func(context.Context, *amendProposalInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *amendProposalInput) (*getProposalOutput, error) {
		if in == nil {
			in = &amendProposalInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.AmendProposal(ctx, id, in.Body.Set, in.Body.Unset, in.Body.Note, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{Body: rec}, nil
	}
}

func getModerationClaim(cat *Catalog) func(context.Context, *getClaimInput) (*getClaimOutput, error) {
	return func(ctx context.Context, in *getClaimInput) (*getClaimOutput, error) {
		if in == nil {
			in = &getClaimInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.GetModerationClaim(ctx, id)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getClaimOutput{ETag: claimETag(rec), Body: rec}, nil
	}
}

func decideModerationClaim(cat *Catalog) func(context.Context, *decideClaimInput) (*decideOutput, error) {
	return func(ctx context.Context, in *decideClaimInput) (*decideOutput, error) {
		if in == nil {
			in = &decideClaimInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.DecideClaim(ctx, id, in.Body.Decision, in.Body.Note, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &decideOutput{Body: rec}, nil
	}
}

func listModerationProposals(cat *Catalog) func(context.Context, *CollectionInput) (*listProposalsOutput, error) {
	return func(ctx context.Context, in *CollectionInput) (*listProposalsOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.ClaimSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListModerationProposals(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listProposalsOutput{Body: page}, nil
	}
}

func decideModerationProposal(cat *Catalog) func(context.Context, *decideProposalInput) (*decideOutput, error) {
	return func(ctx context.Context, in *decideProposalInput) (*decideOutput, error) {
		if in == nil {
			in = &decideProposalInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, err := cat.DecideProposal(ctx, id, in.Body.Decision, in.Body.Note, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &decideOutput{Body: rec}, nil
	}
}

func revertModeration(cat *Catalog) func(context.Context, *revertInput) (*getProposalOutput, error) {
	return func(ctx context.Context, in *revertInput) (*getProposalOutput, error) {
		if in == nil {
			in = &revertInput{}
		}
		rec, err := cat.RevertRevision(ctx, in.Body.RevisionID, in.Body.Reason)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getProposalOutput{Body: rec}, nil
	}
}

type editImageForm struct {
	Preset string `form:"preset" required:"true" enum:"cover,screenshot" doc:"Which editor slot these bytes are for."`
	// Not an image/* contentType list: mime.Writer's CreateFormFile stamps every
	// part application/octet-stream, so a narrower list rejects the exact client
	// this face exists for. The image service validates the bytes it is given.
	File huma.FormFile `form:"file" required:"true" contentType:"application/octet-stream" doc:"The image bytes. The ceiling is the service's own body limit, not a field constraint."`
}

type uploadEditImageInput struct {
	RawBody huma.MultipartFormFiles[editImageForm]
}

type uploadEditImageOutput struct {
	Location string `header:"Location" doc:"Absolute URL of the stored image."`
	Body     repr.EditImage
}

func uploadMyEditImage(cat *Catalog) func(context.Context, *uploadEditImageInput) (*uploadEditImageOutput, error) {
	return func(ctx context.Context, in *uploadEditImageInput) (*uploadEditImageOutput, error) {
		if in == nil {
			in = &uploadEditImageInput{}
		}
		form := in.RawBody.Data()
		if form == nil || !form.File.IsSet {
			p := problem.New(problem.CodeValidationFailed, "", "", "a multipart file part named file is required.")
			p.Errors = []problem.FieldError{{Pointer: "/file", Reason: problem.ReasonRequired,
				Detail: "send multipart/form-data with a file part"}}
			return nil, catalogErr(ctx, p)
		}
		defer form.File.Close()
		rec, err := cat.UploadEditImage(ctx, form.Preset, form.File.Filename, form.File)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &uploadEditImageOutput{Location: rec.URL, Body: rec}, nil
	}
}

func getModerationSnapshot(cat *Catalog) func(context.Context, *snapshotInput) (*snapshotOutput, error) {
	return func(ctx context.Context, in *snapshotInput) (*snapshotOutput, error) {
		if in == nil {
			in = &snapshotInput{}
		}
		rec, err := cat.GetSnapshot(ctx, in.Object, in.ID)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &snapshotOutput{Body: rec}, nil
	}
}
