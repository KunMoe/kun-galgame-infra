package handler

import (
	"context"
	"strconv"

	"api/internal/platform/store/service"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// OwnerApps hands the portal panel the applications one signed-in owner holds.
// It is a function rather than a devapi dependency so the store domain never
// reads the developer-platform tables itself.
type OwnerApps func(ctx context.Context, ownerUserID uint) ([]service.OwnerApp, error)

type DevHandler struct {
	svc  *service.Service
	apps OwnerApps
}

func NewDevHandler(svc *service.Service, apps OwnerApps) *DevHandler {
	return &DevHandler{svc: svc, apps: apps}
}

func (h *DevHandler) Register(r fiber.Router) {
	r.Get("/store/usage", h.Usage)
}

func (h *DevHandler) Usage(c fiber.Ctx) error {
	ownerID, ok := c.Locals("user_id").(uint)
	if !ok || ownerID == 0 {
		return response.Unauthorized(c, errors.ErrAuthUnauthorized)
	}
	apps, err := h.apps(c.Context(), ownerID)
	if err != nil {
		return response.InternalError(c, errors.ErrOperationFailed)
	}
	days, _ := strconv.Atoi(c.Query("days"))
	summary, err := h.svc.OwnerUsage(c.Context(), apps, days)
	if err != nil {
		return response.InternalError(c, errors.ErrOperationFailed)
	}
	return response.Success(c, summary)
}
