package search

import (
	"context"
	"strconv"

	"api/internal/platform/catalog/search/spec"

	"github.com/meilisearch/meilisearch-go"
)

func IndexForType(t string) (uid string, ok bool) {
	switch t {
	case "names":
		return IndexCreditNames, true
	case "characters":
		return IndexCharacters, true
	case "labels":
		return IndexLabels, true
	case "works":
		return IndexWorks, true
	case "tags":
		return IndexTags, true
	default:
		return "", false
	}
}

func LocalesForUI(uid, locale string) []string {
	if uid == IndexWorks {
		return nil
	}
	switch locale {
	case "zh":
		return []string{"cmn"}
	case "ja":
		return []string{"jpn"}
	default:
		return nil
	}
}

type SearchResult struct {
	Hits  []EntityDoc
	Total int64
}

func (e *meiliEngine) SearchEntities(ctx context.Context, uid string, q spec.EntityQuery) (SearchResult, error) {
	req := &meilisearch.SearchRequest{
		HitsPerPage:      int64(q.Limit),
		Locales:          q.Locales,
		MatchingStrategy: meilisearch.All,
	}
	if filter := meiliEntityFilter(q); filter != "" {
		req.Filter = filter
	}
	text := spec.SanitizeQuery(q.Q)
	if text == "" {
		req.Sort = []string{"popularity:desc"}
		req.MatchingStrategy = meilisearch.Last
	}
	resp, err := e.client.Index(uid).SearchWithContext(ctx, text, req)
	if err != nil {
		return SearchResult{}, err
	}
	hits := make([]EntityDoc, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		var d EntityDoc
		if err := h.DecodeInto(&d); err == nil {
			hits = append(hits, d)
		}
	}
	total := resp.TotalHits
	if total == 0 {
		total = resp.EstimatedTotalHits
	}
	return SearchResult{Hits: hits, Total: total}, nil
}

func meiliEntityFilter(q spec.EntityQuery) string {
	if q.ContentRatingNot != nil {
		return "content_rating != " + strconv.Itoa(int(*q.ContentRatingNot))
	}
	return ""
}

func (d EntityDoc) Name() string {
	switch {
	case d.NameJa != "":
		return d.NameJa
	case d.NameZh != "":
		return d.NameZh
	default:
		return d.NameOther
	}
}
