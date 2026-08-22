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
	msgBadLabelKind = "kind must be one of game_brand, bunko, publisher, anime_studio, doujin_circle, group"
	msgBadTagTier   = "tier must be one of core, longtail, hidden"
	msgBadTagKind   = "kind must be one of content, meta"
	msgBadCursor    = "malformed cursor"
)

func (h *PublicHandler) LabelsList(c fiber.Ctx) error {
	f := service.LabelsListFilter{NSFW: nsfwQuery(c), HasWorks: boolQueryPub(c.Query("has_works"))}
	if raw := c.Query("kind"); raw != "" {
		v, ok := labelKindFromKey(raw)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLabelKind)
		}
		f.Kind = &v
	}
	limit, ok := limitPub(c.Query("limit"), 20, 100)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	data, err := h.svc.LabelsList(c.Context(), f, c.Query("cursor"), limit)
	if err != nil {
		return taxonomyListError(c, err)
	}
	c.Set("Cache-Control", cacheSearch)
	return response.Success(c, data)
}

func (h *PublicHandler) TagsList(c fiber.Ctx) error {
	f := service.TagsListFilter{NSFW: nsfwQuery(c), HasWorks: boolQueryPub(c.Query("has_works"))}
	if raw := c.Query("tier"); raw != "" {
		v, ok := tagTierFromKey(raw)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadTagTier)
		}
		f.Tier = &v
	}
	if raw := c.Query("kind"); raw != "" {
		v, ok := tagKindFromKey(raw)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadTagKind)
		}
		f.Kind = &v
	}
	if raw := strings.TrimSpace(c.Query("ids")); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > 100 {
			return response.BadRequestMsg(c, errors.ErrValidationFailed, "at most 100 ids")
		}
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil || id <= 0 {
				return response.BadRequestMsg(c, errors.ErrInvalidParam, "ids must be positive integers")
			}
			f.IDs = append(f.IDs, id)
		}
	}
	limit, ok := limitPub(c.Query("limit"), 20, 100)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	data, err := h.svc.TagsList(c.Context(), f, c.Query("cursor"), limit)
	if err != nil {
		return taxonomyListError(c, err)
	}
	c.Set("Cache-Control", cacheSearch)
	return response.Success(c, data)
}

func (h *PublicHandler) EnginesList(c fiber.Ctx) error {
	limit, ok := limitPub(c.Query("limit"), 20, 100)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	data, err := h.svc.EnginesList(c.Context(), service.EnginesListFilter{NSFW: nsfwQuery(c)}, c.Query("cursor"), limit)
	if err != nil {
		return taxonomyListError(c, err)
	}
	c.Set("Cache-Control", cacheSearch)
	return response.Success(c, data)
}

func (h *PublicHandler) EngineDetail(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	rec, found, err := h.svc.EngineDetail(c.Context(), id, nsfwQuery(c))
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, rec)
}

func taxonomyListError(c fiber.Ctx, err error) error {
	if stderrors.Is(err, service.ErrBadCursor) {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadCursor)
	}
	return response.InternalError(c, errors.ErrInternalServer)
}

func labelKindFromKey(k string) (int16, bool) {
	switch k {
	case "game_brand":
		return model.LabelKindGameBrand, true
	case "bunko":
		return model.LabelKindBunko, true
	case "publisher":
		return model.LabelKindPublisher, true
	case "anime_studio":
		return model.LabelKindAnimeStudio, true
	case "doujin_circle":
		return model.LabelKindDoujinCircle, true
	case "group":
		return model.LabelKindGroup, true
	default:
		return 0, false
	}
}

func tagTierFromKey(k string) (int16, bool) {
	switch k {
	case "core":
		return model.TagTierCore, true
	case "longtail":
		return model.TagTierLongtail, true
	case "hidden":
		return model.TagTierHidden, true
	default:
		return 0, false
	}
}

func tagKindFromKey(k string) (int16, bool) {
	switch k {
	case "content":
		return model.TagKindContent, true
	case "meta":
		return model.TagKindMeta, true
	default:
		return 0, false
	}
}
