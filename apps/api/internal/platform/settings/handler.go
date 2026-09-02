package settings

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	apperrors "api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type Handler struct {
	svc      *Service
	canWrite func(roles []string) bool
}

func NewHandler(svc *Service, canWrite func(roles []string) bool) *Handler {
	return &Handler{svc: svc, canWrite: canWrite}
}

func (h *Handler) Register(admin fiber.Router, view fiber.Handler, write fiber.Handler) {
	g := admin.Group("/settings", view)
	g.Get("", h.Overview)
	g.Get("/audit", h.Audit)
	g.Put("/:key", write, h.Set)
	g.Delete("/:key", write, h.Reset)

	slog.Info("settings console registered under /api/v1/admin/settings/*")
}

func callerFrom(c fiber.Ctx) (userID uint, roles []string) {
	roles, _ = c.Locals("user_roles").([]string)
	userID, _ = c.Locals("user_id").(uint)
	return userID, roles
}

func (h *Handler) Overview(c fiber.Ctx) error {
	_, roles := callerFrom(c)
	ov, err := h.svc.Overview(c.Context(), h.canWrite(roles))
	if err != nil {
		slog.Error("settings: overview failed", "err", err)
		return response.InternalError(c, apperrors.ErrOperationFailed)
	}
	return response.Success(c, ov)
}

func (h *Handler) Audit(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	entries, err := h.svc.Audit(c.Context(), limit)
	if err != nil {
		slog.Error("settings: audit read failed", "err", err)
		return response.InternalError(c, apperrors.ErrOperationFailed)
	}
	return response.Success(c, entries)
}

type setRequest struct {
	Value   json.RawMessage `json:"value"`
	Note    string          `json:"note"`
	Version *int64          `json:"version"`
}

func (h *Handler) Set(c fiber.Ctx) error {
	var req setRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, apperrors.ErrBadRequest)
	}
	userID, _ := callerFrom(c)
	key := c.Params("key")
	view, err := h.svc.Set(c.Context(), userID, key, req.Value, req.Note, req.Version)
	if err != nil {
		return h.writeErr(c, err, key)
	}
	return response.Success(c, view)
}

func (h *Handler) Reset(c fiber.Ctx) error {
	userID, _ := callerFrom(c)
	key := c.Params("key")
	view, err := h.svc.Reset(c.Context(), userID, key, c.Query("note"))
	if err != nil {
		return h.writeErr(c, err, key)
	}
	return response.Success(c, view)
}

func (h *Handler) writeErr(c fiber.Ctx, err error, key string) error {
	switch {
	case errors.Is(err, ErrUnknownKey):
		return response.NotFoundMsg(c, apperrors.ErrNotFound, "unknown setting key")
	case errors.Is(err, ErrInvalidValue), errors.Is(err, ErrNoteTooLong):
		return response.BadRequestMsg(c, apperrors.ErrInvalidParam, err.Error())
	case errors.Is(err, ErrVersionConflict):
		return response.Error(c, fiber.StatusConflict, apperrors.ErrOperationFailed, "该配置已被其他人修改,请刷新后重试")
	case errors.Is(err, ErrNoOverride):
		return response.BadRequestMsg(c, apperrors.ErrOperationFailed, "该配置没有覆盖值")
	default:
		slog.Error("settings: write failed", "key", key, "err", err)
		return response.InternalError(c, apperrors.ErrOperationFailed)
	}
}
