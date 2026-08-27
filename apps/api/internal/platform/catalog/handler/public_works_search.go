package handler

import (
	stderrors "errors"
	"strconv"
	"strings"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const (
	msgBadPage        = "page must be a positive integer"
	msgBadSearchSort  = "sort must be one of relevance, released_desc, released_asc, updated, popularity"
	msgBadSearchFacet = "facets must be a comma-separated subset of content_rating, olang, claimed, tag_id, label_id, engine_id, series_id, source"
)

func (h *PublicHandler) WorksSearch(c fiber.Ctx) error {
	f := service.WorksSearchFilter{
		SearchIntro: boolQueryPub(c.Query("search_intro")),
		Q:           c.Query("q"),
		NSFW:        nsfwQuery(c),
		OLang:       worksSearchOLang(c.Query("olang")),
		Include:     service.ParseWorksListInclude(c.Query("include")),
		Fields:      fieldsQuery(c),
	}

	f.Sort = strings.TrimSpace(c.Query("sort"))
	if _, ok := service.WorksSearchSortRule(f.Sort); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadSearchSort)
	}
	if raw := strings.TrimSpace(c.Query("facets")); raw != "" {
		for _, tok := range strings.Split(raw, ",") {
			tok = strings.TrimSpace(tok)
			if !service.IsWorksSearchFacet(tok) {
				return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadSearchFacet)
			}
			f.Facets = append(f.Facets, tok)
		}
	}
	if cr := c.Query("content_rating"); cr != "" {
		v, ok := contentRatingFromKey(cr)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "content_rating must be all_ages|sensitive|r18")
		}
		if v == model.ContentRatingR18 && !f.NSFW {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "content_rating=r18 requires nsfw=1")
		}
		f.ContentRating = &v
	}
	if cl := c.Query("claimed"); cl != "" {
		switch strings.ToLower(strings.TrimSpace(cl)) {
		case "1", "true", "yes":
			t := true
			f.Claimed = &t
		case "0", "false", "no":
			v := false
			f.Claimed = &v
		default:
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "claimed must be true|false")
		}
	}
	var ok bool
	if f.ClaimStates, ok = claimStatesPub(c.Query("claim_state")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadClaimState)
	}
	if f.DisplayLimits, ok = displayLimitsPub(c.Query("content_limit")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadDisplayLimit)
	}
	if f.LabelID, ok = posIntQueryPub(c.Query("label_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "label_id must be a positive integer")
	}
	if f.TagIDs, ok = posIntListQueryPub(c.Query("tag_id"), maxTagIDFilters); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadTagIDs)
	}
	if f.SeriesID, ok = posIntQueryPub(c.Query("series_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "series_id must be a positive integer")
	}
	if f.EngineID, ok = posIntQueryPub(c.Query("engine_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "engine_id must be a positive integer")
	}
	if f.ReleasedAfter, ok = datePub(c.Query("released_after")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "released_after must be YYYY-MM-DD")
	}
	if f.ReleasedBefore, ok = datePub(c.Query("released_before")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "released_before must be YYYY-MM-DD")
	}
	if f.Page, ok = pageNumPub(c.Query("page")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadPage)
	}
	if f.Limit, ok = limitPub(c.Query("limit"), 20, 100); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}

	data, err := h.svc.WorksSearch(c.Context(), f)
	if err != nil {
		if stderrors.Is(err, service.ErrSearchUnavailable) {
			return response.InternalError(c, errors.ErrInternalServer)
		}
		return response.InternalError(c, errors.ErrInternalServer)
	}
	c.Set("Cache-Control", cacheSearch)
	return successProjected(c, data, f.Fields, f.Fields.ProjectItems)
}

func pageNumPub(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 1, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
