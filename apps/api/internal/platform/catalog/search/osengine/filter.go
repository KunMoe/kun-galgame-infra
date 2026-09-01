package osengine

import "api/internal/platform/catalog/search/spec"

func filterClauses(f spec.WorksFilter) []any {
	var clauses []any
	if f.DocID != "" {
		clauses = append(clauses, termClause("id", f.DocID))
	}
	if f.ContentRatingNot != nil {
		clauses = append(clauses, mustNotTerm("content_rating", *f.ContentRatingNot))
	}
	if f.ContentRating != nil {
		clauses = append(clauses, termClause("content_rating", *f.ContentRating))
	}
	if f.Claimed != nil {
		clauses = append(clauses, termClause("claimed", *f.Claimed))
	}
	if len(f.ClaimStates) > 0 {
		clauses = append(clauses, termsClause("claim_state", f.ClaimStates))
	}
	if f.ClaimStateNot != "" {
		clauses = append(clauses, mustNotTerm("claim_state", f.ClaimStateNot))
	}
	if len(f.DisplayLimits) > 0 {
		clauses = append(clauses, termsClause("content_limit", f.DisplayLimits))
	}
	for _, id := range f.TagIDs {
		clauses = append(clauses, termClause("tag_ids", id))
	}
	if f.LabelID > 0 {
		clauses = append(clauses, termClause("label_ids", f.LabelID))
	}
	if f.EngineID > 0 {
		clauses = append(clauses, termClause("engine_ids", f.EngineID))
	}
	if f.SeriesID > 0 {
		clauses = append(clauses, termClause("series_ids", f.SeriesID))
	}
	if f.ReleasedAfter > 0 || f.ReleasedBefore > 0 {
		rng := map[string]any{}
		if f.ReleasedAfter > 0 {
			rng["gte"] = f.ReleasedAfter
		}
		if f.ReleasedBefore > 0 {
			rng["lte"] = f.ReleasedBefore
		}
		clauses = append(clauses, map[string]any{
			"range": map[string]any{"released_ord": rng},
		})
	}
	if len(f.OLangs) > 0 {
		clauses = append(clauses, termsClause("olang", f.OLangs))
	}
	return clauses
}

func termClause(field string, value any) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

func termsClause(field string, values any) map[string]any {
	return map[string]any{"terms": map[string]any{field: values}}
}

func mustNotTerm(field string, value any) map[string]any {
	return map[string]any{
		"bool": map[string]any{
			"must_not": map[string]any{
				"term": map[string]any{field: value},
			},
		},
	}
}
