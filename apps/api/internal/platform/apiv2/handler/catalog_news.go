package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type newsIDInput struct {
	ID string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Decimal news item id."`
}

type listNewsOutput struct {
	Body repr.List[repr.NewsItem]
}
type getNewsOutput struct {
	Body repr.NewsItem
}
type listNewsSourcesOutput struct {
	Body repr.List[repr.NewsSource]
}

func registerNews(api huma.API, cat *Catalog) {
	news := []string{"news"}
	huma.Register(api, huma.Operation{
		OperationID:        "listNews",
		Method:             http.MethodGet,
		Path:               "/v2/news",
		Summary:            "List news items",
		Description:        "Published news feed. Keyset-paginated. Unauthenticated. Attribution fields are required on every item.",
		Tags:               news,
		Errors:             collectionErrors(http.StatusServiceUnavailable),
		SkipValidateParams: true,
	}, listNews(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "listNewsSources",
		Method:             http.MethodGet,
		Path:               "/v2/news/sources",
		Summary:            "List news sources",
		Description:        "Every attributed news source. Unauthenticated.",
		Tags:               news,
		Errors:             collectionErrors(http.StatusServiceUnavailable),
		SkipValidateParams: true,
	}, listNewsSources(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getNewsItem",
		Method:             http.MethodGet,
		Path:               "/v2/news/{id}",
		Summary:            "Get one news item",
		Description:        "A published news item. Withdrawn items are 404. Unauthenticated. source and source_url are always present.",
		Tags:               news,
		Errors:             collectionErrors(http.StatusNotFound, http.StatusServiceUnavailable),
		SkipValidateParams: true,
	}, getNewsItem(cat))
}

func listNews(cat *Catalog) func(context.Context, *collectionInput) (*listNewsOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listNewsOutput, error) {
		q, err := collect.Parse(rawFrom(in), collect.NewsSpec())
		if err != nil {
			return nil, withIdent(ctx, err)
		}
		page, lerr := cat.ListNews(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listNewsOutput{Body: page}, nil
	}
}

func listNewsSources(cat *Catalog) func(context.Context, *struct{}) (*listNewsSourcesOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*listNewsSourcesOutput, error) {
		page, err := cat.NewsSources(ctx)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &listNewsSourcesOutput{Body: page}, nil
	}
}

func getNewsItem(cat *Catalog) func(context.Context, *newsIDInput) (*getNewsOutput, error) {
	return func(ctx context.Context, in *newsIDInput) (*getNewsOutput, error) {
		id, ok := repr.ParseID(in.ID)
		if !ok {
			p := problem.New(problem.CodeInvalidParameter, "", "", "id must be a positive decimal id.")
			p.Errors = []problem.FieldError{{Parameter: "id", Reason: problem.ReasonInvalidFormat, Detail: in.ID}}
			return nil, withIdent(ctx, p)
		}
		rec, err := cat.NewsItem(ctx, id)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getNewsOutput{Body: rec}, nil
	}
}
