package handler

import (
	"strconv"

	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// Two cache tiers, split on one checkable question: does the editor face have a
// field that writes this block? Covers, screenshots, tags, roster, credits,
// releases, intros, series, links and engines all do, so they must not be
// fresher or staler than works/{id} itself. Ratings and relations have no edit
// field at all — their only writers are the nightly rating lanes and the VNDB
// relation importer — so a longer edge cache cannot hide anyone's edit.
const cacheWorkImported = "public, max-age=0, s-maxage=1800, stale-while-revalidate=600"

const workSubLimit = 100

func (h *PublicHandler) workSub(c fiber.Ctx, cache string, load func(id int64, limit, offset int) (any, bool, error)) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	limit, ok := limitPub(c.Query("limit"), workSubLimit, workSubLimit)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	offset := atoiOrPub(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	data, found, err := load(id, limit, offset)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Cache-Control", cache)
	return response.Success(c, data)
}

func (h *PublicHandler) WorkCovers(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkCovers(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkScreenshots(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkScreenshots(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkTags(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkTags(c.Context(), id, nsfwQuery(c), spoilersQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkCharacters(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkCharacters(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkCredits(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkCredits(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkReleases(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkReleases(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkIntros(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkIntros(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkRatings(c fiber.Ctx) error {
	return h.workSub(c, cacheWorkImported, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkRatings(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkRelations(c fiber.Ctx) error {
	return h.workSub(c, cacheWorkImported, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkRelations(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkSeries(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkSeries(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkLinks(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkLinks(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}

func (h *PublicHandler) WorkEngines(c fiber.Ctx) error {
	return h.workSub(c, cacheDetail, func(id int64, limit, offset int) (any, bool, error) {
		return h.svc.WorkEngines(c.Context(), id, nsfwQuery(c), limit, offset)
	})
}
