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
	CollectionInput
	Object string `query:"object" maxLength:"32" doc:"Restrict to one family. Closed: work, release, character, credit_name, person, company, tag, engine."`
}

type calendarInput struct {
	CollectionInput
	Month     string `query:"month" maxLength:"7" doc:"Dated month window YYYY-MM. Default: current month in Asia/Tokyo."`
	Year      string `query:"year" maxLength:"4" doc:"Year-only window YYYY (v1 pending). Default with precision=year: current year in Asia/Tokyo."`
	Precision string `query:"precision" maxLength:"8" doc:"day, month, or year. year selects the year-only window. day and month use the dated month window."`
	Status    string `query:"status" maxLength:"16" doc:"released, dated, announced, cancelled, unknown. announced and unknown select the undated window. cancelled is empty until the catalog records cancellations."`
}

type listCalendarOutput struct {
	Body repr.List[repr.Work]
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
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogCalendar",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/calendar",
		Summary:            "Release calendar",
		Description:        "One collection. month=/year= pick a window; precision= and status= select among the dated month, year-only, and undated views that were three v1 routes. Requires an application key. ids= is not accepted.",
		Tags:               catalog,
		Errors:             errs,
		SkipValidateParams: true,
	}, listCatalogCalendar(cat))
}

func listCatalogChanges(cat *Catalog) func(context.Context, *CollectionInput) (*listChangesOutput, error) {
	return func(ctx context.Context, in *CollectionInput) (*listChangesOutput, error) {
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
		q, err := parseCatalogList(ctx, &in.CollectionInput, collect.RedirectsSpec())
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

func listCatalogCalendar(cat *Catalog) func(context.Context, *calendarInput) (*listCalendarOutput, error) {
	return func(ctx context.Context, in *calendarInput) (*listCalendarOutput, error) {
		if in == nil {
			in = &calendarInput{}
		}
		q, err := parseCatalogList(ctx, &in.CollectionInput, collect.CalendarSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListCalendar(ctx, q, in.Month, in.Year, in.Precision, in.Status)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listCalendarOutput{Body: page}, nil
	}
}
