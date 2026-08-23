package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type listCompaniesOutput struct {
	Body repr.List[repr.Company]
}
type listTagsOutput struct {
	Body repr.List[repr.Tag]
}
type listSeriesOutput struct {
	Body repr.List[repr.Series]
}
type listEnginesOutput struct {
	Body repr.List[repr.Engine]
}
type listReleasesOutput struct {
	Body repr.List[repr.Release]
}

func registerCatalogLists(api huma.API, cat *Catalog) {
	catalog := []string{"catalog"}
	errs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable)
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogCompanies",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/companies",
		Summary:            "List companies",
		Description:        "Keyset-paginated company registry (v1 labels). Requires an application key. ids=/refs= is a batch lane and does not paginate.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogCompanies(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogTags",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/tags",
		Summary:            "List tags",
		Description:        "Keyset-paginated canonical tags. Requires an application key. ids= is a batch lane and does not paginate.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogTags(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogSeries",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/series",
		Summary:            "List series",
		Description:        "Keyset-paginated series. Requires an application key. ids= is a batch lane and does not paginate.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogSeries(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogEngines",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/engines",
		Summary:            "List engines",
		Description:        "Keyset-paginated engines. Requires an application key. ids= is a batch lane and does not paginate.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogEngines(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogReleases",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/releases",
		Summary:            "List releases",
		Description:        "Keyset-paginated dated releases, sorted by date_desc by default. Requires an application key. ids= is a batch lane and does not paginate.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogReleases(cat))
}

func listCatalogCompanies(cat *Catalog) func(context.Context, *collectionInput) (*listCompaniesOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listCompaniesOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.CompanySpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListCompanies(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listCompaniesOutput{Body: page}, nil
	}
}

func listCatalogTags(cat *Catalog) func(context.Context, *collectionInput) (*listTagsOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listTagsOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.TagSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListTags(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listTagsOutput{Body: page}, nil
	}
}

func listCatalogSeries(cat *Catalog) func(context.Context, *collectionInput) (*listSeriesOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listSeriesOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.SeriesSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListSeries(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listSeriesOutput{Body: page}, nil
	}
}

func listCatalogEngines(cat *Catalog) func(context.Context, *collectionInput) (*listEnginesOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listEnginesOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.EngineSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListEngines(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listEnginesOutput{Body: page}, nil
	}
}

func listCatalogReleases(cat *Catalog) func(context.Context, *collectionInput) (*listReleasesOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listReleasesOutput, error) {
		q, err := parseCatalogList(ctx, in, collect.ReleaseSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListReleases(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listReleasesOutput{Body: page}, nil
	}
}

func parseCatalogList(ctx context.Context, in *collectionInput, spec collect.Spec) (collect.Query, error) {
	q, err := collect.Parse(rawFrom(in), spec)
	if err != nil {
		return collect.Query{}, withIdent(ctx, err)
	}
	if q.NSFW {
		if p := refuseNSFW(ctx); p != nil {
			return collect.Query{}, p
		}
	}
	return q, nil
}
