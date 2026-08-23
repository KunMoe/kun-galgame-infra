package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type listChangesOutput struct {
	Body repr.List[repr.Change]
}
type listRedirectsOutput struct {
	Body repr.List[repr.Redirect]
}

type listRedirectsInput struct {
	collectionInput
	Object string `query:"object" maxLength:"32" doc:"Restrict to one family. Closed: work, release, character, credit_name, person, company, tag, engine."`
}

func registerCatalogFeeds(api huma.API, cat *Catalog) {
	catalog := []string{"catalog"}
	errs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable)
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogChanges",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/changes",
		Summary:            "Catalog changes feed",
		Description:        "Works updated recently, oldest first. Keyset-paginated. Requires an application key. ids= is not accepted.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogChanges(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogRedirects",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/redirects",
		Summary:            "Entity merge feed",
		Description:        "Redirects from merged-away ids, oldest first. Keyset-paginated. object= restricts to one family. Requires an application key. ids= is not accepted.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogRedirects(cat))
}

func listCatalogChanges(cat *Catalog) func(context.Context, *collectionInput) (*listChangesOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listChangesOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.ChangesSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListChanges(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listChangesOutput{Body: page}, nil
	}
}

func listCatalogRedirects(cat *Catalog) func(context.Context, *listRedirectsInput) (*listRedirectsOutput, error) {
	return func(ctx context.Context, in *listRedirectsInput) (*listRedirectsOutput, error) {
		if in == nil {
			in = &listRedirectsInput{}
		}
		q, err := parseCatalogList(ctx, &in.collectionInput, collect.RedirectsSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListRedirects(ctx, q, in.Object)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listRedirectsOutput{Body: page}, nil
	}
}
