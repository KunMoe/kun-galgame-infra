package collect

import (
	"sort"
	"strings"

	"api/internal/platform/apiv2/parse"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
)

const DefaultLimit = 20

type Raw struct {
	Cursor       string
	Limit        string
	View         string
	Include      string
	Fields       string
	IDs          string
	Refs         string
	IncludeTotal string
	Facets       string
	Sort         string
}

type Spec struct {
	Sort    []string
	Include []string
	FullSet []string
	Fields  []string
	Facets  []string
}

type Query struct {
	Cursor       string
	Limit        int
	View         string
	Include      []string
	Fields       []string
	IDs          []string
	Refs         []repr.Ref
	IncludeTotal bool
	Facets       []string
	Sort         string
	Batch        bool
}

func Parse(raw Raw, spec Spec) (Query, *problem.Problem) {
	q := Query{View: "basic", Limit: DefaultLimit}
	if spec.Sort != nil {
		q.Sort = spec.Sort[0]
	}

	if raw.View != "" {
		v, err := parse.Enum(raw.View, "view", []string{"basic", "full"})
		if err != nil {
			return Query{}, err
		}
		q.View = v
	}

	limit, err := parse.Limit(raw.Limit, DefaultLimit, parse.MaxPageLimit)
	if err != nil {
		return Query{}, err
	}
	q.Limit = limit

	if raw.Sort != "" {
		if len(spec.Sort) == 0 {
			return Query{}, problem.New(problem.CodeUnknownSort, "", "", "this collection does not take sort.")
		}
		s, err := parse.Enum(raw.Sort, "sort", spec.Sort)
		if err != nil {
			d, _ := problem.Lookup(problem.CodeUnknownSort)
			err.Code = d.Code
			err.Title = d.Title
			err.Type = d.TypeURI()
			err.Status = d.Status
			return Query{}, err
		}
		q.Sort = s
	}

	ids, err := splitIDs(raw.IDs)
	if err != nil {
		return Query{}, err
	}
	refs, err := splitRefs(raw.Refs)
	if err != nil {
		return Query{}, err
	}
	q.IDs, q.Refs = ids, refs
	q.Batch = len(ids) > 0 || len(refs) > 0

	if raw.Cursor != "" && q.Batch {
		p := problem.New(problem.CodeMutuallyExclusiveParameters, "", "", "cursor cannot be combined with ids or refs.")
		p.Errors = []problem.FieldError{{
			Parameter: "cursor",
			Reason:    problem.ReasonNotAllowedValue,
			Detail:    "ids= and refs= are a batch lane and do not paginate",
		}}
		return Query{}, p
	}
	if raw.Cursor != "" {
		key, err := DecodeCursor(raw.Cursor)
		if err != nil {
			return Query{}, err
		}
		q.Cursor = key
	}

	if q.Batch {
		q.Limit = 0
	}

	inc, err := tokens(raw.Include, "include", spec.Include, problem.CodeUnknownInclude)
	if err != nil {
		return Query{}, err
	}
	if q.View == "full" {
		inc = union(inc, spec.FullSet)
	}
	q.Include = inc

	fields, err := tokens(raw.Fields, "fields", spec.Fields, problem.CodeUnknownField)
	if err != nil {
		return Query{}, err
	}
	if len(fields) > 0 {
		fields = union(fields, []string{"object", "id"})
		sort.Strings(fields)
	}
	q.Fields = fields

	if raw.IncludeTotal != "" {
		v, err := parse.Bool(raw.IncludeTotal, "include_total")
		if err != nil {
			return Query{}, err
		}
		q.IncludeTotal = v
	}

	facets, err := tokens(raw.Facets, "facets", spec.Facets, problem.CodeUnknownFacet)
	if err != nil {
		return Query{}, err
	}
	q.Facets = facets
	return q, nil
}

func tokens(raw, name string, allowed []string, code string) ([]string, *problem.Problem) {
	if raw == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if seen[t] {
			continue
		}
		if allowed != nil && !contains(allowed, t) {
			p := problem.New(code, "", "", "unknown "+name+" token.")
			p.Errors = []problem.FieldError{{
				Parameter: name,
				Reason:    problem.ReasonUnknownValue,
				Detail:    "allowed values: " + strings.Join(allowed, ", "),
			}}
			return nil, p
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, nil
}

func splitIDs(raw string) ([]string, *problem.Problem) {
	if raw == "" {
		return nil, nil
	}
	var ids []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		ids = append(ids, t)
	}
	if len(ids) > 100 {
		p := problem.New(problem.CodeTooManyIDs, "", "", "ids= accepts at most 100 values.")
		p.Errors = []problem.FieldError{{
			Parameter: "ids",
			Reason:    problem.ReasonTooManyItems,
			Detail:    "maximum 100",
		}}
		return nil, p
	}
	return ids, nil
}

func splitRefs(raw string) ([]repr.Ref, *problem.Problem) {
	if raw == "" {
		return nil, nil
	}
	var refs []repr.Ref
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		src, ext, ok := strings.Cut(t, ":")
		if !ok || src == "" || ext == "" {
			p := problem.New(problem.CodeInvalidParameter, "", "", "refs= entries must be source:external_id.")
			p.Errors = []problem.FieldError{{
				Parameter: "refs",
				Reason:    problem.ReasonInvalidFormat,
				Detail:    "expected source:external_id",
			}}
			return nil, p
		}
		refs = append(refs, repr.Ref{Source: src, ExternalID: ext})
	}
	if len(refs) > 100 {
		p := problem.New(problem.CodeTooManyIDs, "", "", "refs= accepts at most 100 values.")
		p.Errors = []problem.FieldError{{
			Parameter: "refs",
			Reason:    problem.ReasonTooManyItems,
			Detail:    "maximum 100",
		}}
		return nil, p
	}
	return refs, nil
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, xs := range [][]string{a, b} {
		for _, x := range xs {
			if seen[x] {
				continue
			}
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
