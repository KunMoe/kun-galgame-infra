package handler

import (
	"context"
	"errors"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type resourceIDInput struct {
	ID      string `path:"id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Decimal catalog id."`
	NSFW    string `query:"nsfw" maxLength:"8" doc:"true includes r18. Requires the NSFW capability. false or absent hides r18. Only true or false."`
	View    string `query:"view" maxLength:"16" doc:"basic (default) or full. Closed vocabulary."`
	Include string `query:"include" maxLength:"1024" doc:"Comma-separated blocks. Unknown token is 400 UNKNOWN_INCLUDE."`
	Fields  string `query:"fields" maxLength:"1024" doc:"Comma-separated top-level keys. Unknown token is 400 UNKNOWN_FIELD. object and id are always kept."`
}

type getWorkOutput struct {
	Body repr.Work
}
type getCompanyOutput struct {
	Body repr.Company
}
type getCreditNameOutput struct {
	Body repr.CreditName
}
type getCharacterOutput struct {
	Body repr.Character
}
type getTagOutput struct {
	Body repr.Tag
}
type getSeriesOutput struct {
	Body repr.Series
}
type getEngineOutput struct {
	Body repr.Engine
}
type getReleaseOutput struct {
	Body repr.Release
}
type getStatsOutput struct {
	Body repr.CatalogStats
}

func registerCatalog(api huma.API, cat *Catalog) {
	catalog := []string{"catalog"}
	authErrs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable)
	statsErrs := collectionErrors(http.StatusServiceUnavailable)

	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogWork",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/works/{id}",
		Summary:            "Get one catalog work",
		Description:        "Work detail. Merged ids are 404 ENTITY_MERGED with Link rel=canonical. r18 is 404 without nsfw=true. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogWork(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogCompany",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/companies/{id}",
		Summary:            "Get one company",
		Description:        "Company registry row (v1 labels). Merged ids are 404 ENTITY_MERGED. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogCompany(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogCreditName",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/credit-names/{id}",
		Summary:            "Get one credit name",
		Description:        "A credited name, not a person. person_id is null when unlinked. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogCreditName(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogCharacter",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/characters/{id}",
		Summary:            "Get one character",
		Description:        "Character detail. Merged ids are 404 ENTITY_MERGED. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogCharacter(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogTag",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/tags/{id}",
		Summary:            "Get one tag",
		Description:        "Canonical tag. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogTag(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogSeries",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/series/{id}",
		Summary:            "Get one series",
		Description:        "Series detail. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogSeries(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogEngine",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/engines/{id}",
		Summary:            "Get one engine",
		Description:        "Engine detail. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogEngine(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogRelease",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/releases/{id}",
		Summary:            "Get one release",
		Description:        "A catalog release. Merged ids are 404 ENTITY_MERGED. r18 parent works are 404 without nsfw=true. Requires an application key.",
		Tags:               catalog,
		Errors:             authErrs,
		SkipValidateParams: true,
	}, getCatalogRelease(cat))
	huma.Register(api, huma.Operation{
		OperationID:        "getCatalogStats",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/stats",
		Summary:            "Catalog totals",
		Description:        "Live entity counts. Unauthenticated.",
		Tags:               catalog,
		Errors:             statsErrs,
		SkipValidateParams: true,
	}, getCatalogStats(cat))
	registerCatalogLists(api, cat)
	registerCatalogFeeds(api, cat)
	registerWorkSubs(api, cat)
	registerNews(api, cat)
}

func getCatalogWork(cat *Catalog) func(context.Context, *resourceIDInput) (*getWorkOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getWorkOutput, error) {
		id, q, err := parseResource(ctx, in, collect.WorkSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetWork(ctx, id, q.NSFW, q.Include)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getWorkOutput{Body: rec}, nil
	}
}

func getCatalogCompany(cat *Catalog) func(context.Context, *resourceIDInput) (*getCompanyOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getCompanyOutput, error) {
		id, q, err := parseResource(ctx, in, collect.CompanySpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetCompany(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getCompanyOutput{Body: rec}, nil
	}
}

func getCatalogCreditName(cat *Catalog) func(context.Context, *resourceIDInput) (*getCreditNameOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getCreditNameOutput, error) {
		id, q, err := parseResource(ctx, in, collect.CreditNameSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetCreditName(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getCreditNameOutput{Body: rec}, nil
	}
}

func getCatalogCharacter(cat *Catalog) func(context.Context, *resourceIDInput) (*getCharacterOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getCharacterOutput, error) {
		id, q, err := parseResource(ctx, in, collect.CharacterSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetCharacter(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getCharacterOutput{Body: rec}, nil
	}
}

func getCatalogTag(cat *Catalog) func(context.Context, *resourceIDInput) (*getTagOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getTagOutput, error) {
		id, q, err := parseResource(ctx, in, collect.TagSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetTag(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getTagOutput{Body: rec}, nil
	}
}

func getCatalogSeries(cat *Catalog) func(context.Context, *resourceIDInput) (*getSeriesOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getSeriesOutput, error) {
		id, q, err := parseResource(ctx, in, collect.SeriesSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetSeries(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getSeriesOutput{Body: rec}, nil
	}
}

func getCatalogEngine(cat *Catalog) func(context.Context, *resourceIDInput) (*getEngineOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getEngineOutput, error) {
		id, q, err := parseResource(ctx, in, collect.EngineSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetEngine(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getEngineOutput{Body: rec}, nil
	}
}

func getCatalogRelease(cat *Catalog) func(context.Context, *resourceIDInput) (*getReleaseOutput, error) {
	return func(ctx context.Context, in *resourceIDInput) (*getReleaseOutput, error) {
		id, q, err := parseResource(ctx, in, collect.ReleaseSpec())
		if err != nil {
			return nil, err
		}
		rec, gerr := cat.GetRelease(ctx, id, q.NSFW)
		if gerr != nil {
			return nil, catalogErr(ctx, gerr)
		}
		return &getReleaseOutput{Body: rec}, nil
	}
}

func getCatalogStats(cat *Catalog) func(context.Context, *struct{}) (*getStatsOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*getStatsOutput, error) {
		rec, err := cat.Stats(ctx)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getStatsOutput{Body: rec}, nil
	}
}

func parseResource(ctx context.Context, in *resourceIDInput, spec collect.Spec) (int64, collect.Query, error) {
	if in == nil {
		in = &resourceIDInput{}
	}
	id, ok := repr.ParseID(in.ID)
	if !ok {
		p := problem.New(problem.CodeInvalidParameter, "", "", "id must be a positive decimal catalog id.")
		p.Errors = []problem.FieldError{{Parameter: "id", Reason: problem.ReasonInvalidFormat, Detail: in.ID}}
		return 0, collect.Query{}, withIdent(ctx, p)
	}
	q, err := collect.Parse(collect.Raw{View: in.View, Include: in.Include, Fields: in.Fields, NSFW: in.NSFW}, spec)
	if err != nil {
		return 0, collect.Query{}, withIdent(ctx, err)
	}
	if q.NSFW {
		if p := refuseNSFW(ctx); p != nil {
			return 0, collect.Query{}, p
		}
	}
	return id, q, nil
}

func refuseNSFW(ctx context.Context) *problem.Problem {
	if nsfwAllowed(ctx) {
		return nil
	}
	id, inst := ident(ctx)
	return problem.New(problem.CodeNSFWCapabilityRequired, id, inst,
		"nsfw=true requires a credential with the NSFW capability.")
}

func nsfwAllowed(ctx context.Context) bool {
	v, _ := ctx.Value("nsfw_allowed").(bool)
	return v
}

func catalogErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var p *problem.Problem
	if errors.As(err, &p) {
		return withIdent(ctx, p)
	}
	return err
}
