package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type createNewsInput struct {
	Body struct {
		Source      string   `json:"source" minLength:"1" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"News source key. Must be a source bound to the bearer and active."`
		Lane        string   `json:"lane,omitempty" enum:"news,column" doc:"Section of the source. Defaults to news."`
		Title       string   `json:"title" minLength:"1" maxLength:"512" doc:"Must not be used as a discriminant."`
		Summary     string   `json:"summary" minLength:"1" maxLength:"2000" doc:"The lede the source wrote. At most 200 runes; longer is 422 VALIDATION_FAILED with reason TOO_LONG. Must not be used as a discriminant."`
		SourceURL   string   `json:"source_url" minLength:"1" maxLength:"1024" format:"uri" doc:"Canonical link to the original item. Attribution always carries a link."`
		PublishedAt string   `json:"published_at,omitempty" format:"date-time" maxLength:"32" doc:"RFC 3339. Defaults to the moment of submission."`
		BannerHash  string   `json:"banner_hash,omitempty" maxLength:"64" pattern:"^([0-9a-f]{64})?$" doc:"Image-service content hash of the banner. The format is checked; existence is not."`
		WorkIDs     []string `json:"work_ids,omitempty" maxItems:"100" doc:"Catalog work ids to link. Stored with manual confidence."`
	}
}

type getNewsSubmissionInput struct {
	ID string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"News item id."`
}

type patchNewsSubmissionInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"News item id."`
	IfMatch string `header:"If-Match" doc:"Current ETag. Required to withdraw."`
	Body    struct {
		Status     *string   `json:"status,omitempty" enum:"withdrawn" doc:"Only withdrawn. Publishing and rejection happen in the moderation queue, never here."`
		Title      *string   `json:"title,omitempty" minLength:"1" maxLength:"512" doc:"Must not be used as a discriminant."`
		Summary    *string   `json:"summary,omitempty" minLength:"1" maxLength:"2000" doc:"At most 200 runes; longer is 422 VALIDATION_FAILED with reason TOO_LONG. Must not be used as a discriminant."`
		SourceURL  *string   `json:"source_url,omitempty" minLength:"1" maxLength:"1024" format:"uri" doc:"Canonical link to the original item."`
		BannerHash *string   `json:"banner_hash,omitempty" maxLength:"64" pattern:"^([0-9a-f]{64})?$" doc:"Image-service content hash. The empty string clears the banner."`
		WorkIDs    *[]string `json:"work_ids,omitempty" maxItems:"100" doc:"Replaces the whole linked-work set."`
	}
}

type listNewsSubmissionsOutput struct {
	Body repr.List[repr.NewsSubmission]
}

type newsSubmissionOutput struct {
	ETag string `header:"ETag"`
	Body repr.NewsSubmission
}

type createNewsSubmissionOutput struct {
	Location string `header:"Location"`
	ETag     string `header:"ETag"`
	Body     repr.NewsSubmission
}

func registerMeNews(api huma.API, cat *Catalog) {
	me := []string{"me"}
	errs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable)
	writeErrs := append(errs, http.StatusUnprocessableEntity, http.StatusConflict, http.StatusPreconditionRequired, http.StatusPreconditionFailed)

	huma.Register(api, huma.Operation{
		OperationID: "listMyNews", Method: http.MethodGet, Path: "/v2/me/news",
		Summary: "List my news items", Description: "Items under the sources bound to the bearer, pending included. Keyset-paginated. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, listMyNews(cat))
	huma.Register(api, huma.Operation{
		OperationID: "createMyNews", Method: http.MethodPost, Path: "/v2/me/news",
		Summary: "Submit a news item", Description: "Always lands on pending: publishing is a human step. source must be bound to the bearer and active. Requires a user access token.",
		Tags: me, Errors: writeErrs, DefaultStatus: http.StatusCreated, SkipValidateParams: true,
	}, createMyNews(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getMyNewsItem", Method: http.MethodGet, Path: "/v2/me/news/{id}",
		Summary: "Get one of my news items", Description: "Carries an ETag for If-Match. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, getMyNewsItem(cat))
	huma.Register(api, huma.Operation{
		OperationID: "patchMyNewsItem", Method: http.MethodPatch, Path: "/v2/me/news/{id}",
		Summary: "Edit or withdraw one of my news items", Description: `While pending, edits title/summary/source_url/banner_hash/work_ids and sends the item back for machine scoring. Once published the only legal transition is {"status":"withdrawn"} with If-Match. rejected is terminal.`,
		Tags: me, Errors: writeErrs, SkipValidateParams: true,
	}, patchMyNewsItem(cat))
}

func listMyNews(cat *Catalog) func(context.Context, *CollectionInput) (*listNewsSubmissionsOutput, error) {
	return func(ctx context.Context, in *CollectionInput) (*listNewsSubmissionsOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.NewsSubmissionSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListMyNews(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listNewsSubmissionsOutput{Body: page}, nil
	}
}

func createMyNews(cat *Catalog) func(context.Context, *createNewsInput) (*createNewsSubmissionOutput, error) {
	return func(ctx context.Context, in *createNewsInput) (*createNewsSubmissionOutput, error) {
		if in == nil {
			in = &createNewsInput{}
		}
		rec, etag, err := cat.CreateMyNews(ctx, newsSubmissionBody{
			Source: in.Body.Source, Lane: in.Body.Lane, Title: in.Body.Title,
			Summary: in.Body.Summary, SourceURL: in.Body.SourceURL,
			PublishedAt: in.Body.PublishedAt, BannerHash: in.Body.BannerHash,
			WorkIDs: in.Body.WorkIDs,
		})
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &createNewsSubmissionOutput{Location: "/v2/me/news/" + rec.ID, ETag: etag, Body: rec}, nil
	}
}

func getMyNewsItem(cat *Catalog) func(context.Context, *getNewsSubmissionInput) (*newsSubmissionOutput, error) {
	return func(ctx context.Context, in *getNewsSubmissionInput) (*newsSubmissionOutput, error) {
		if in == nil {
			in = &getNewsSubmissionInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, etag, err := cat.GetMyNews(ctx, id)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &newsSubmissionOutput{ETag: etag, Body: rec}, nil
	}
}

func patchMyNewsItem(cat *Catalog) func(context.Context, *patchNewsSubmissionInput) (*newsSubmissionOutput, error) {
	return func(ctx context.Context, in *patchNewsSubmissionInput) (*newsSubmissionOutput, error) {
		if in == nil {
			in = &patchNewsSubmissionInput{}
		}
		id, ok := repr.ParseID(in.ID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.ID))
		}
		rec, etag, err := cat.PatchMyNews(ctx, id, newsPatchBody{
			Status: in.Body.Status, Title: in.Body.Title, Summary: in.Body.Summary,
			SourceURL: in.Body.SourceURL, BannerHash: in.Body.BannerHash, WorkIDs: in.Body.WorkIDs,
		}, in.IfMatch)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &newsSubmissionOutput{ETag: etag, Body: rec}, nil
	}
}
