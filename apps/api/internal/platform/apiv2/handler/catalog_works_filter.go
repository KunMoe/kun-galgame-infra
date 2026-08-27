package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/parse"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catmodel "api/internal/platform/catalog/model"
	catsvc "api/internal/platform/catalog/service"
)

type worksFilter struct {
	Q              string
	ContentRating  *int16
	Claimed        *bool
	ClaimStates    []string
	DisplayLimits  []string
	Site           string
	CompanyID      int64
	CompanyRollup  bool
	TagIDs         []int64
	SeriesID       int64
	EngineID       int64
	Platform       string
	ReleasedAfter  int64
	ReleasedBefore int64
	OLang          catsvc.PublicOLang
}

func parseWorksFilter(in *listWorksInput) (worksFilter, *problem.Problem) {
	f := worksFilter{OLang: catsvc.PublicOLang{All: true}}
	if in == nil {
		return f, nil
	}
	f.Q = strings.TrimSpace(in.Q)
	if in.ContentRating != "" {
		v, ok := contentRatingFromKey(in.ContentRating)
		if !ok {
			return worksFilter{}, closedParam("content_rating", "all_ages, sensitive, r18")
		}
		f.ContentRating = &v
	}
	if in.Claimed != "" {
		v, err := parse.Bool(in.Claimed, "claimed")
		if err != nil {
			return worksFilter{}, err
		}
		f.Claimed = &v
	}
	states, err := closedCSV(in.ClaimState, "claim_state", []string{
		catmodel.ClaimStateKeyNone, catmodel.ClaimStateKeyLive, catmodel.ClaimStateKeyDraft,
		catmodel.ClaimStateKeyPending, catmodel.ClaimStateKeyDeclined, catmodel.ClaimStateKeyHidden,
	})
	if err != nil {
		return worksFilter{}, err
	}
	f.ClaimStates = states
	limits, err := closedCSV(in.ContentLimit, "content_limit", []string{
		catmodel.DisplayLimitKeySFW, catmodel.DisplayLimitKeyNSFW,
	})
	if err != nil {
		return worksFilter{}, err
	}
	f.DisplayLimits = limits
	f.Site = strings.TrimSpace(in.Site)
	id, err := optionalID(in.CompanyID, "company_id")
	if err != nil {
		return worksFilter{}, err
	}
	f.CompanyID = id
	if in.CompanyRollup != "" {
		v, berr := parse.Bool(in.CompanyRollup, "company_rollup")
		if berr != nil {
			return worksFilter{}, berr
		}
		f.CompanyRollup = v
	}
	tags, err := idList(in.TagID, "tag_id", 10)
	if err != nil {
		return worksFilter{}, err
	}
	f.TagIDs = tags
	if f.SeriesID, err = optionalID(in.SeriesID, "series_id"); err != nil {
		return worksFilter{}, err
	}
	if f.EngineID, err = optionalID(in.EngineID, "engine_id"); err != nil {
		return worksFilter{}, err
	}
	f.Platform = strings.TrimSpace(in.Platform)
	if f.ReleasedAfter, err = dateOrdinal(in.ReleasedAfter, "released_after"); err != nil {
		return worksFilter{}, err
	}
	if f.ReleasedBefore, err = dateOrdinal(in.ReleasedBefore, "released_before"); err != nil {
		return worksFilter{}, err
	}
	olang, err := parseOLang(in.OLang)
	if err != nil {
		return worksFilter{}, err
	}
	f.OLang = olang
	return f, nil
}

func (c *Catalog) ListWorksFiltered(ctx context.Context, q collect.Query, f worksFilter) (repr.List[repr.Work], error) {
	if c == nil || c.Public == nil {
		return repr.List[repr.Work]{}, problem.New(problem.CodeServiceUnavailable, "", "", "works collection is not bound.")
	}
	if f.ContentRating != nil && *f.ContentRating == catmodel.ContentRatingR18 && !q.NSFW {
		p := problem.New(problem.CodeInvalidParameter, "", "", "content_rating=r18 requires nsfw=true.")
		p.Errors = []problem.FieldError{{Parameter: "content_rating", Reason: problem.ReasonNotAllowedValue, Detail: "nsfw=true is required"}}
		return repr.List[repr.Work]{}, p
	}
	if q.Sort == "relevance" && f.Q == "" && !q.Batch {
		p := problem.New(problem.CodeInvalidParameter, "", "", "sort=relevance requires q=.")
		p.Errors = []problem.FieldError{{Parameter: "sort", Reason: problem.ReasonNotAllowedValue, Detail: "pass q="}}
		return repr.List[repr.Work]{}, p
	}
	if q.Batch && f.Q != "" {
		p := problem.New(problem.CodeMutuallyExclusiveParameters, "", "", "q= cannot be combined with ids= or refs=.")
		p.Errors = []problem.FieldError{{Parameter: "q", Reason: problem.ReasonNotAllowedValue, Detail: "ids=/refs= is a batch lane"}}
		return repr.List[repr.Work]{}, p
	}
	inc := listWorksInclude(q.Include)
	if q.Batch || (!searchWorksRequested(q, f)) {
		return c.listWorksSQL(ctx, q, f, inc)
	}
	return c.listWorksSearch(ctx, q, f, inc)
}

func searchWorksRequested(q collect.Query, f worksFilter) bool {
	if f.Q != "" || collect.SearchSort(q.Sort) {
		return true
	}
	return len(q.Facets) > 0
}

func (c *Catalog) listWorksSQL(ctx context.Context, q collect.Query, f worksFilter, inc catsvc.WorksListInclude) (repr.List[repr.Work], error) {
	lf := catsvc.WorksListFilter{
		ContentRating: f.ContentRating, Claimed: f.Claimed, ClaimStates: f.ClaimStates,
		DisplayLimits: f.DisplayLimits, Site: f.Site, LabelID: f.CompanyID, LabelRollup: f.CompanyRollup,
		TagIDs: f.TagIDs, SeriesID: f.SeriesID, EngineID: f.EngineID, Platform: f.Platform,
		ReleasedAfter: f.ReleasedAfter, ReleasedBefore: f.ReleasedBefore, NSFW: q.NSFW,
		Sort: q.Sort, OLang: f.OLang, Include: inc,
	}
	if lf.Sort == "" || collect.SearchSort(lf.Sort) {
		lf.Sort = "id"
	}
	var missing []string
	if q.Batch {
		ids, miss, err := c.batchWorkIDs(ctx, q)
		if err != nil {
			return repr.List[repr.Work]{}, err
		}
		lf.IDs = ids
		missing = miss
		q.Cursor = ""
	}
	// Every entity list guards the all-miss batch; works shipped without it, so
	// refs= that resolved nothing dropped the IDs filter and answered an
	// unfiltered page 1 of the whole catalogue (found by the forum's client).
	if q.Batch && len(lf.IDs) == 0 {
		sqlQ := q
		sqlQ.IncludeTotal = false
		out := finishList([]repr.Work{}, nil, 0, sqlQ, missing)
		if len(q.Facets) > 0 {
			out.Facets = emptyFacets(q.Facets)
		}
		return out, nil
	}
	limit := q.Limit
	if q.Batch {
		limit = 100
	}
	data, err := c.Public.WorksList(ctx, lf, q.Cursor, limit)
	if err != nil {
		if errors.Is(err, catsvc.ErrBadCursor) {
			return repr.List[repr.Work]{}, collectInvalidCursor()
		}
		return repr.List[repr.Work]{}, err
	}
	items := make([]repr.Work, 0, len(data.Items))
	seen := map[int64]bool{}
	for _, it := range data.Items {
		items = append(items, workFromListItem(it, q.Include))
		seen[it.ID] = true
	}
	if q.Batch {
		for _, id := range lf.IDs {
			if !seen[id] {
				missing = append(missing, repr.ID(id))
			}
		}
	}
	sqlQ := q
	sqlQ.IncludeTotal = false
	out := finishList(items, data.NextCursor, 0, sqlQ, missing)
	if len(q.Facets) > 0 {
		out.Facets = emptyFacets(q.Facets)
	}
	return out, nil
}

func (c *Catalog) listWorksSearch(ctx context.Context, q collect.Query, f worksFilter, inc catsvc.WorksListInclude) (repr.List[repr.Work], error) {
	page := 1
	if q.Cursor != "" {
		n, err := strconv.Atoi(q.Cursor)
		if err != nil || n < 1 {
			return repr.List[repr.Work]{}, collectInvalidCursor()
		}
		page = n
	}
	sort := q.Sort
	if f.Q != "" && (sort == "" || sort == "id") {
		sort = "relevance"
	}
	sf := catsvc.WorksSearchFilter{
		Q: f.Q, ContentRating: f.ContentRating, Claimed: f.Claimed, ClaimStates: f.ClaimStates,
		DisplayLimits: f.DisplayLimits, LabelID: f.CompanyID, TagIDs: f.TagIDs, EngineID: f.EngineID,
		SeriesID: f.SeriesID, ReleasedAfter: f.ReleasedAfter, ReleasedBefore: f.ReleasedBefore,
		OLang: f.OLang, NSFW: q.NSFW, Sort: sort, Facets: searchFacetTokens(q.Facets),
		Page: page, Limit: q.Limit, Include: inc,
	}
	data, err := c.Public.WorksSearch(ctx, sf)
	if err != nil {
		if errors.Is(err, catsvc.ErrSearchUnavailable) {
			return repr.List[repr.Work]{}, problem.New(problem.CodeServiceUnavailable, "", "", "works search is not bound.")
		}
		return repr.List[repr.Work]{}, err
	}
	items := make([]repr.Work, 0, len(data.Items))
	for _, it := range data.Items {
		items = append(items, workFromListItem(it, q.Include))
	}
	var next *string
	if int64(page)*int64(data.Limit) < data.Total && len(data.Items) > 0 {
		enc := collect.EncodeCursor(strconv.Itoa(page + 1))
		next = &enc
	}
	out := repr.NewList(items, next)
	if q.IncludeTotal {
		n := data.Total
		out.Total = &n
	}
	if len(q.Facets) > 0 {
		out.Facets = mapSearchFacets(q.Facets, data.Facets)
	}
	return out, nil
}

func listWorksInclude(tokens []string) catsvc.WorksListInclude {
	var inc catsvc.WorksListInclude
	for _, t := range tokens {
		switch t {
		case "titles":
			inc.Names = true
		case "intros":
			inc.Intros = true
		case "companies":
			inc.Labels = true
		case "ratings":
			inc.Ratings = true
		case "covers":
			inc.Covers = true
		case "refs":
			inc.Refs = true
		}
	}
	return inc
}

func searchFacetTokens(in []string) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		switch t {
		case "company_id":
			out = append(out, "company_id")
		case "tag_id", "olang", "content_rating":
			out = append(out, t)
		}
	}
	return out
}

func mapSearchFacets(want []string, dist map[string]map[string]int64) *map[string][]repr.FacetValue {
	out := map[string][]repr.FacetValue{}
	for _, name := range want {
		srcName := name
		if name == "company_id" {
			if _, ok := dist["company_id"]; !ok {
				srcName = "label_id"
			}
		}
		bucket := dist[srcName]
		vals := make([]repr.FacetValue, 0, len(bucket))
		for k, n := range bucket {
			vals = append(vals, repr.FacetValue{Value: k, DisplayName: k, Count: int(n)})
		}
		if vals == nil {
			vals = []repr.FacetValue{}
		}
		out[name] = vals
	}
	return &out
}

func emptyFacets(want []string) *map[string][]repr.FacetValue {
	out := map[string][]repr.FacetValue{}
	for _, name := range want {
		out[name] = []repr.FacetValue{}
	}
	return &out
}

func contentRatingFromKey(s string) (int16, bool) {
	switch s {
	case "all_ages":
		return catmodel.ContentRatingAllAges, true
	case "sensitive":
		return catmodel.ContentRatingSensitive, true
	case "r18":
		return catmodel.ContentRatingR18, true
	default:
		return 0, false
	}
}

func closedParam(name, allowed string) *problem.Problem {
	p := problem.New(problem.CodeUnknownEnumValue, "", "", name+" is not in the closed vocabulary.")
	p.Errors = []problem.FieldError{{Parameter: name, Reason: problem.ReasonUnknownValue, Detail: "allowed values: " + allowed}}
	return p
}

func closedCSV(raw, name string, allowed []string) ([]string, *problem.Problem) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	allow := map[string]bool{}
	for _, a := range allowed {
		allow[a] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if !allow[tok] {
			return nil, closedParam(name, strings.Join(allowed, ", "))
		}
		if seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out, nil
}

func optionalID(raw, name string) (int64, *problem.Problem) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, ok := repr.ParseID(raw)
	if !ok || n <= 0 {
		p := problem.New(problem.CodeInvalidParameter, "", "", name+" must be a decimal catalog id.")
		p.Errors = []problem.FieldError{{Parameter: name, Reason: problem.ReasonInvalidFormat, Detail: raw}}
		return 0, p
	}
	return n, nil
}

func idList(raw, name string, max int) ([]int64, *problem.Problem) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > max {
		p := problem.New(problem.CodeInvalidParameter, "", "", name+" accepts at most "+strconv.Itoa(max)+" values.")
		p.Errors = []problem.FieldError{{Parameter: name, Reason: problem.ReasonOutOfRange, Detail: "maximum " + strconv.Itoa(max)}}
		return nil, p
	}
	var out []int64
	for _, p := range parts {
		n, err := optionalID(p, name)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, n)
		}
	}
	return out, nil
}

func dateOrdinal(raw, name string) (int64, *problem.Problem) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	t, perr := parse.Date(raw, name)
	if perr != nil {
		return 0, perr
	}
	return int64(t.Year())*10000 + int64(t.Month())*100 + int64(t.Day()), nil
}

func parseOLang(raw string) (catsvc.PublicOLang, *problem.Problem) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return catsvc.PublicOLang{All: true}, nil
	}
	if raw == "all" {
		return catsvc.PublicOLang{All: true}, nil
	}
	var vals []string
	seen := map[string]bool{}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" || seen[tok] {
			continue
		}
		seen[tok] = true
		vals = append(vals, tok)
	}
	return catsvc.PublicOLang{Values: vals}, nil
}
