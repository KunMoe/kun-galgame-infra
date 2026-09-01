// works_query.go — the Meilisearch half of the works PRODUCT search face
// (A2-1d, refs/proj/126 D5): page-based pagination, an opt-in facet
// distribution, five sort lanes and the relevance floor.
//
// It deliberately returns only ids + counts. The wire items are re-hydrated
// from Postgres into the works-list item shape, so a Meilisearch document field
// never becomes a public field (裁定 4) and the search row is byte-identical to
// the browse row a consumer already renders.
//
// ── why the whole filter set is pushed down here ────────────────────────────
//
// The deprecated wiki search filtered `content_limit` in SQL while leaving it
// out of the Meilisearch filter, so `total` counted rows the caller could never
// receive and sfw pagination silently lost rows. Every filter of this face is
// compiled into ONE Meilisearch expression instead, which is what makes
// `total`, the facet distribution and the page share a single gate by
// construction rather than by discipline.
package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"api/internal/platform/catalog/search/spec"

	"github.com/meilisearch/meilisearch-go"
)

const worksSearchScoreThreshold = 0.4

type WorksResult struct {
	IDs    []int64
	Total  int64
	Facets map[string]map[string]int64
}

func (e *meiliEngine) SearchWorks(ctx context.Context, q spec.WorksQuery) (WorksResult, error) {
	page, limit := q.Page, q.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	text := spec.SanitizeQuery(q.Q)

	req := &meilisearch.SearchRequest{
		Page:             int64(page),
		HitsPerPage:      int64(limit),
		MatchingStrategy: meilisearch.All,
	}
	if filter := MeiliFilter(q.Filter); filter != "" {
		req.Filter = filter
	}
	if !q.SearchIntro {
		req.AttributesToSearchOn = WorksTitleSearchable
	}
	if len(q.FacetAttrs) > 0 {
		req.Facets = q.FacetAttrs
	}
	switch {
	case text != "":
		req.RankingScoreThreshold = worksSearchScoreThreshold
		if q.SortLane != "" {
			req.Sort = []string{q.SortLane}
		}
	case q.SortLane != "":
		req.Sort = []string{q.SortLane}
		req.MatchingStrategy = meilisearch.Last
	default:
		req.Sort = []string{"popularity:desc"}
		req.MatchingStrategy = meilisearch.Last
	}

	resp, err := e.client.Index(IndexWorks).SearchWithContext(ctx, text, req)
	if err != nil {
		return WorksResult{}, err
	}

	out := WorksResult{IDs: make([]int64, 0, len(resp.Hits))}
	for _, h := range resp.Hits {
		var d struct {
			ID string `json:"id"`
		}
		if err := h.DecodeInto(&d); err != nil {
			slog.Warn("works search: undecodable hit dropped",
				"raw_id", string(h["id"]), "err", err)
			continue
		}
		id, ok := WorkDocIDToWorkID(d.ID)
		if !ok {
			slog.Warn("works search: hit with unparseable doc id dropped", "doc_id", d.ID)
			continue
		}
		out.IDs = append(out.IDs, id)
	}
	out.Total = resp.TotalHits
	if out.Total == 0 {
		out.Total = resp.EstimatedTotalHits
	}
	if len(resp.FacetDistribution) > 0 {
		var dist map[string]map[string]int64
		if err := json.Unmarshal(resp.FacetDistribution, &dist); err == nil {
			out.Facets = dist
		}
	}
	return out, nil
}

func MeiliFilter(f spec.WorksFilter) string {
	var clauses []string
	if f.DocID != "" {
		clauses = append(clauses, "id = '"+EscapeFilterValue(f.DocID)+"'")
	}
	if f.ContentRatingNot != nil {
		clauses = append(clauses, "content_rating != "+strconv.Itoa(int(*f.ContentRatingNot)))
	}
	if f.ContentRating != nil {
		clauses = append(clauses, "content_rating = "+strconv.Itoa(int(*f.ContentRating)))
	}
	if f.Claimed != nil {
		clauses = append(clauses, "claimed = "+strconv.FormatBool(*f.Claimed))
	}
	if len(f.ClaimStates) > 0 {
		or := make([]string, 0, len(f.ClaimStates))
		for _, st := range f.ClaimStates {
			or = append(or, "claim_state = '"+EscapeFilterValue(st)+"'")
		}
		clauses = append(clauses, "("+strings.Join(or, " OR ")+")")
	}
	if f.ClaimStateNot != "" {
		clauses = append(clauses, "claim_state != '"+f.ClaimStateNot+"'")
	}
	if len(f.DisplayLimits) > 0 {
		or := make([]string, 0, len(f.DisplayLimits))
		for _, lim := range f.DisplayLimits {
			or = append(or, "content_limit = '"+EscapeFilterValue(lim)+"'")
		}
		clauses = append(clauses, "("+strings.Join(or, " OR ")+")")
	}
	for _, tagID := range f.TagIDs {
		clauses = append(clauses, "tag_ids = "+strconv.FormatInt(tagID, 10))
	}
	for _, e := range []struct {
		attr string
		id   int64
	}{
		{"label_ids", f.LabelID}, {"engine_ids", f.EngineID}, {"series_ids", f.SeriesID},
	} {
		if e.id > 0 {
			clauses = append(clauses, e.attr+" = "+strconv.FormatInt(e.id, 10))
		}
	}
	if f.ReleasedAfter > 0 {
		clauses = append(clauses, "released_ord >= "+strconv.FormatInt(f.ReleasedAfter, 10))
	}
	if f.ReleasedBefore > 0 {
		clauses = append(clauses, "released_ord <= "+strconv.FormatInt(f.ReleasedBefore, 10))
	}
	if olang := meiliOLang(f.OLang); olang != "" {
		clauses = append(clauses, olang)
	}
	return strings.Join(clauses, " AND ")
}

func meiliOLang(o spec.OLang) string {
	switch {
	case o.All:
		return ""
	case len(o.Values) > 0:
		quoted := make([]string, len(o.Values))
		for i, v := range o.Values {
			quoted[i] = "'" + EscapeFilterValue(v) + "'"
		}
		return "olang IN [" + strings.Join(quoted, ", ") + "]"
	default:
		return "(olang = 'ja' OR olang STARTS WITH 'zh')"
	}
}

func WorkDocID(workID int64) string { return "w" + strconv.FormatInt(workID, 10) }

func WorkDocIDToWorkID(docID string) (int64, bool) {
	if !strings.HasPrefix(docID, "w") {
		return 0, false
	}
	id, err := strconv.ParseInt(docID[1:], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func EscapeFilterValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `\'`)
}
