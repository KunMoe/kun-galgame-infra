package handler

import (
	"context"
	"strconv"
	"strings"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/parse"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catmodel "api/internal/platform/catalog/model"
	catsearch "api/internal/platform/catalog/search"
	"api/internal/platform/catalog/search/spec"
)

var searchObjects = []string{"work", "character", "credit_name", "company", "tag"}

func (c *Catalog) Search(ctx context.Context, q collect.Query, object, query, locale string) (repr.List[repr.SearchHit], error) {
	if c == nil || c.Searcher == nil {
		return repr.List[repr.SearchHit]{}, problem.New(problem.CodeServiceUnavailable, "", "", "search is not bound.")
	}
	if q.Batch {
		return repr.List[repr.SearchHit]{}, feedNoBatch("search")
	}
	if q.Cursor != "" {
		p := problem.New(problem.CodeInvalidParameter, "", "", "search does not paginate with cursor.")
		p.Errors = []problem.FieldError{{Parameter: "cursor", Reason: problem.ReasonInvalidFormat, Detail: "omit cursor on this collection"}}
		return repr.List[repr.SearchHit]{}, p
	}
	rawObject := strings.TrimSpace(object)
	object, err := parse.Enum(rawObject, "object", searchObjects)
	if err != nil {
		if rawObject == "" {
			p := problem.New(problem.CodeInvalidParameter, "", "", "object is required.")
			p.Errors = []problem.FieldError{{Parameter: "object", Reason: problem.ReasonRequired, Detail: "one of work, character, credit_name, company, tag"}}
			return repr.List[repr.SearchHit]{}, p
		}
		return repr.List[repr.SearchHit]{}, err
	}
	uid, v1Type := searchIndex(object)
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	eq := spec.EntityQuery{
		Q:       query,
		Limit:   limit,
		Locales: catsearch.LocalesForUI(uid, locale),
	}
	if object == "work" && !q.NSFW {
		r18 := catmodel.ContentRatingR18
		eq.ContentRatingNot = &r18
	}
	res, serr := c.Searcher.SearchEntities(ctx, uid, eq)
	if serr != nil {
		return repr.List[repr.SearchHit]{}, serr
	}
	items := make([]repr.SearchHit, 0, len(res.Hits))
	ids := make([]int64, 0, len(res.Hits))
	for _, d := range res.Hits {
		id, ok := stripSearchID(d.ID)
		if !ok {
			continue
		}
		hit := repr.SearchHit{
			Object: "search_result", TargetObject: object, ID: repr.ID(id),
			DisplayName: d.Name(), Latin: optString(d.Latin), Localized: map[string]repr.LocalizedText{},
			Sources: d.Sources,
		}
		if hit.Sources == nil {
			hit.Sources = []string{}
		}
		if object == "work" && d.ContentRating != nil {
			if cr, ok := repr.ContentRating(*d.ContentRating); ok {
				hit.ContentRating = &cr
			}
		}
		if object == "tag" {
			if d.Tier != nil {
				if tier, ok := repr.TagTier(*d.Tier); ok {
					hit.Tier = &tier
				}
			}
			if d.Kind != nil {
				if k, ok := repr.TagKind(*d.Kind); ok {
					hit.TagKind = &k
				}
			}
		}
		items = append(items, hit)
		ids = append(ids, id)
	}
	if c.Public != nil {
		if fillErr := c.fillSearchLocalized(ctx, v1Type, ids, items); fillErr != nil {
			return repr.List[repr.SearchHit]{}, fillErr
		}
	}
	out := finishList(items, nil, res.Total, q, nil)
	return out, nil
}

func (c *Catalog) fillSearchLocalized(ctx context.Context, v1Type string, ids []int64, items []repr.SearchHit) error {
	switch v1Type {
	case "work":
		blocks, err := c.Public.WorkNamesByID(ctx, ids)
		if err != nil {
			return err
		}
		for i := range items {
			id, _ := repr.ParseID(items[i].ID)
			items[i].Localized = localizedFrom(blocks[id].Localized)
		}
	case "name", "character", "label":
		loc, err := c.Public.LocalizedForEntities(ctx, v1Type, ids)
		if err != nil {
			return err
		}
		for i := range items {
			id, _ := repr.ParseID(items[i].ID)
			items[i].Localized = localizedFrom(loc[id])
		}
	}
	return nil
}

func searchIndex(object string) (uid, v1Type string) {
	switch object {
	case "work":
		uid, _ = catsearch.IndexForType("works")
		return uid, "work"
	case "character":
		uid, _ = catsearch.IndexForType("characters")
		return uid, "character"
	case "credit_name":
		uid, _ = catsearch.IndexForType("names")
		return uid, "name"
	case "company":
		uid, _ = catsearch.IndexForType("labels")
		return uid, "label"
	case "tag":
		uid, _ = catsearch.IndexForType("tags")
		return uid, "tag"
	default:
		return "", ""
	}
}

func stripSearchID(id string) (int64, bool) {
	if len(id) < 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(id[1:], 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
