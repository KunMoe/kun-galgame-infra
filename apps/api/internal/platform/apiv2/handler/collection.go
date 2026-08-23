package handler

import (
	"context"
	"net/http"
	"strings"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/apiv2/vocab"
	"api/internal/platform/devapi"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

type collectionInput struct {
	Cursor       string `query:"cursor" maxLength:"512" doc:"Opaque keyset cursor from a prior next_cursor. Must start with cur_."`
	Limit        string `query:"limit" maxLength:"8" doc:"Page size 1-100, default 20. Values above 100 are 400 LIMIT_TOO_LARGE, not clamped."`
	View         string `query:"view" maxLength:"16" doc:"basic (default) or full. Closed vocabulary."`
	Include      string `query:"include" maxLength:"1024" doc:"Comma-separated blocks. Unknown token is 400 UNKNOWN_INCLUDE."`
	Fields       string `query:"fields" maxLength:"1024" doc:"Comma-separated top-level keys after view/include. Unknown token is 400 UNKNOWN_FIELD. object and id are always kept."`
	IDs          string `query:"ids" maxLength:"4096" doc:"Comma-separated ids, max 100. Batch lane: no pagination."`
	Refs         string `query:"refs" maxLength:"4096" doc:"Comma-separated source:external_id, max 100. Batch lane: no pagination."`
	IncludeTotal string `query:"include_total" maxLength:"8" doc:"true to include total. Only true or false."`
	Facets       string `query:"facets" maxLength:"512" doc:"Comma-separated facet names. Unknown token is 400 UNKNOWN_FACET."`
	Sort         string `query:"sort" maxLength:"32" doc:"Closed per-collection sort key."`
	NSFW         string `query:"nsfw" maxLength:"8" doc:"true includes r18. Requires the NSFW capability. false or absent hides r18. Only true or false."`
}

type listVocabOutput struct {
	Body repr.List[vocab.Vocabulary]
}
type listProblemsOutput struct {
	Body repr.List[problemType]
}
type listWorksOutput struct {
	Body repr.List[repr.Work]
}

func collectionErrors(extra ...int) []int {
	out := []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusInternalServerError}
	return append(out, extra...)
}

func registerCollections(api huma.API, works WorksFunc) {
	tags := []string{"meta"}
	huma.Register(api, huma.Operation{
		OperationID:        "listProblemTypes",
		Method:             http.MethodGet,
		Path:               "/v2/problems",
		Summary:            "List every top-level error code",
		Description:        "The closed registry of top-level error codes. Keyset-paginated. Unauthenticated.",
		Tags:               tags,
		Errors:             collectionErrors(),
		SkipValidateParams: true,
	}, listProblemTypes)
	huma.Register(api, huma.Operation{
		OperationID:        "listVocabularies",
		Method:             http.MethodGet,
		Path:               "/v2/vocabularies",
		Summary:            "List published vocabularies",
		Description:        "Closed and seed-open vocabularies. Keyset-paginated. Unauthenticated.",
		Tags:               tags,
		Errors:             collectionErrors(),
		SkipValidateParams: true,
	}, listVocabularies)
	huma.Register(api, huma.Operation{
		OperationID:        "listCatalogWorks",
		Method:             http.MethodGet,
		Path:               "/v2/catalog/works",
		Summary:            "List catalog works",
		Description:        "Keyset-paginated work collection. Requires an application key. view/include/fields/ids/refs/facets follow the v2 collection contract.",
		Tags:               []string{"catalog"},
		Errors:             collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable),
		SkipValidateParams: true,
	}, listWorks(works))
}

type WorksFunc func(ctx context.Context, q collect.Query) (repr.List[repr.Work], error)

func listVocabularies(ctx context.Context, in *collectionInput) (*listVocabOutput, error) {
	q, err := collect.Parse(rawFrom(in), collect.VocabSpec())
	if err != nil {
		return nil, withIdent(ctx, err)
	}
	page, perr := collect.SliceErr(vocab.All(), q, func(v vocab.Vocabulary) string { return v.Name })
	if perr != nil {
		return nil, withIdent(ctx, perr)
	}
	return &listVocabOutput{Body: page}, nil
}

func listProblemTypes(ctx context.Context, in *collectionInput) (*listProblemsOutput, error) {
	q, err := collect.Parse(rawFrom(in), collect.ProblemSpec())
	if err != nil {
		return nil, withIdent(ctx, err)
	}
	all := make([]problemType, 0, len(problem.Codes))
	for _, d := range problem.Codes {
		all = append(all, problemTypeFrom(d))
	}
	page, perr := collect.SliceErr(all, q, func(v problemType) string { return v.Code })
	if perr != nil {
		return nil, withIdent(ctx, perr)
	}
	return &listProblemsOutput{Body: page}, nil
}

func listWorks(src WorksFunc) func(context.Context, *collectionInput) (*listWorksOutput, error) {
	return func(ctx context.Context, in *collectionInput) (*listWorksOutput, error) {
		q, err := collect.Parse(rawFrom(in), collect.WorkSpec())
		if err != nil {
			return nil, withIdent(ctx, err)
		}
		if q.NSFW {
			if p := refuseNSFW(ctx); p != nil {
				return nil, p
			}
		}
		if src == nil {
			id, inst := ident(ctx)
			return nil, problem.New(problem.CodeServiceUnavailable, id, inst, "works collection is not bound.")
		}
		page, lerr := src(ctx, q)
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listWorksOutput{Body: page}, nil
	}
}

func rawFrom(in *collectionInput) collect.Raw {
	if in == nil {
		return collect.Raw{}
	}
	return collect.Raw{
		Cursor: in.Cursor, Limit: in.Limit, View: in.View, Include: in.Include,
		Fields: in.Fields, IDs: in.IDs, Refs: in.Refs, IncludeTotal: in.IncludeTotal,
		Facets: in.Facets, Sort: in.Sort, NSFW: in.NSFW,
	}
}

func withIdent(ctx context.Context, p *problem.Problem) *problem.Problem {
	if p == nil {
		return nil
	}
	id, inst := ident(ctx)
	if p.RequestID == "" {
		p.RequestID = id
	}
	if p.Instance == "" {
		p.Instance = inst
	}
	return p
}

func catalogAuth(lookup func(context.Context, string) (*devapi.Credential, error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, "/v2/catalog/") || path == "/v2/catalog/openapi.json" || path == "/v2/catalog/stats" {
			return c.Next()
		}
		h := c.Get("Authorization")
		const pfx = "Bearer "
		if h == "" {
			return problem.WriteFiberError(c, problem.New(problem.CodeMissingCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is required."))
		}
		if !strings.HasPrefix(h, pfx) {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		token := strings.TrimSpace(h[len(pfx):])
		if token == "" {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		if lookup == nil {
			return c.Next()
		}
		if !devapi.HasKeyPrefix(token) {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		cred, err := lookup(c.Context(), token)
		if err != nil {
			return problem.WriteFiberError(c, problem.New(problem.CodeServiceUnavailable, problem.RequestID(c), problem.Instance(c),
				"credential store is unavailable."))
		}
		if cred == nil {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		if !cred.HasScope(devapi.ScopeCatalogRead) {
			return problem.WriteFiberError(c, problem.New(problem.CodeScopeRequired, problem.RequestID(c), problem.Instance(c),
				"this operation requires the catalog:read scope."))
		}
		devapi.WithCredential(c, cred)
		return c.Next()
	}
}
