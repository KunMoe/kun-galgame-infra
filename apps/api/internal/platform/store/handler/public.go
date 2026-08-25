package handler

import (
	stderrors "errors"
	"time"

	"api/internal/platform/store/service"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const (
	msgBadProductID = "product_id must match ^(RJ|VJ)[0-9]{6,8}$"
	msgQuota        = "this application has minted the maximum number of purchase links"
	msgShortener    = "link shortener unavailable — no link was issued"
	msgOffline      = "store link service is not configured"
	msgBadRange     = "from/to must be YYYY-MM-DD JST days, from <= to, at most 92 days apart"

	// Every response is keyed to the calling application, so a shared cache
	// holding one site's short links and serving them to another would hand the
	// clicks to the wrong site. 00-workflow §1.3.
	cachePrivate = "private"
)

type PublicHandler struct {
	svc *service.Service
}

func NewPublicHandler(svc *service.Service) *PublicHandler {
	return &PublicHandler{svc: svc}
}

func (h *PublicHandler) offline(c fiber.Ctx) bool {
	if h.svc != nil && h.svc.Configured() {
		return false
	}
	_ = response.Error(c, fiber.StatusServiceUnavailable, errors.ErrStoreUnconfigured, msgOffline)
	return true
}

func (h *PublicHandler) PurchaseLinks(c fiber.Ctx) error {
	c.Set("Cache-Control", cachePrivate)
	if h.offline(c) {
		return nil
	}
	clientID, ok := clientFromCtx(c)
	if !ok {
		return response.Unauthorized(c, errors.ErrAuthUnauthorized)
	}

	data, err := h.svc.PurchaseLinks(c.Context(), clientID, c.Params("product_id"))
	switch {
	case err == nil:
		return response.Success(c, data)
	case stderrors.Is(err, service.ErrInvalidProductID):
		return response.Error(c, fiber.StatusUnprocessableEntity, errors.ErrStoreInvalidProduct, msgBadProductID)
	case stderrors.Is(err, service.ErrQuotaExceeded):
		return response.ForbiddenMsg(c, errors.ErrStoreQuotaExceeded, msgQuota)
	case stderrors.Is(err, service.ErrNotConfigured):
		return response.Error(c, fiber.StatusServiceUnavailable, errors.ErrStoreUnconfigured, msgOffline)
	case stderrors.Is(err, service.ErrShortenerDown):
		// Deliberately NOT a fallback to the bare aff URL: a raw aff link
		// bypasses the click counter entirely, which is the one thing the
		// deduplication promise to DLsite rests on.
		return response.Error(c, fiber.StatusBadGateway, errors.ErrStoreLinkUnavailable, msgShortener)
	default:
		return response.InternalError(c, errors.ErrInternalServer)
	}
}

func (h *PublicHandler) MyStats(c fiber.Ctx) error {
	c.Set("Cache-Control", cachePrivate)
	if h.svc == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, errors.ErrStoreUnconfigured, msgOffline)
	}
	clientID, ok := clientFromCtx(c)
	if !ok {
		return response.Unauthorized(c, errors.ErrAuthUnauthorized)
	}

	from, to, err := service.ResolveRange(time.Now(), c.Query("from"), c.Query("to"))
	if err != nil {
		return response.Error(c, fiber.StatusUnprocessableEntity, errors.ErrInvalidParam, msgBadRange)
	}
	data, err := h.svc.MyStats(c.Context(), clientID, from, to)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	return response.Success(c, data)
}
