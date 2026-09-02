package settings

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	apperrors "api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type SiteLookup func(ctx context.Context, id uint) (bool, error)

type SiteOf func(c fiber.Ctx) *uint

type Handler struct {
	svc      *Service
	canWrite func(roles []string) bool
	sites    SiteLookup
	siteOf   SiteOf
}

func NewHandler(svc *Service, canWrite func(roles []string) bool, sites SiteLookup) *Handler {
	return &Handler{svc: svc, canWrite: canWrite, sites: sites}
}

func (h *Handler) Register(admin fiber.Router, view fiber.Handler, write fiber.Handler) {
	g := admin.Group("/settings", view)
	g.Get("", h.Overview)
	g.Get("/audit", h.Audit)
	g.Put("/:key", write, h.Set)
	g.Delete("/:key", write, h.Reset)

	slog.Info("settings console registered under /api/v1/admin/settings/*")
}

func (h *Handler) RegisterS2S(v1 fiber.Router, clientAuth fiber.Handler, siteOf SiteOf) {
	h.siteOf = siteOf
	v1.Get("/settings", clientAuth, h.Effective)
	slog.Info("settings read face registered at /api/v1/settings")
}

func callerFrom(c fiber.Ctx) (userID uint, roles []string) {
	roles, _ = c.Locals("user_roles").([]string)
	userID, _ = c.Locals("user_id").(uint)
	return userID, roles
}

func (h *Handler) scopeFrom(c fiber.Ctx) (Scope, error) {
	s := c.Query("site")
	if s == "" {
		return PlatformScope, nil
	}
	id, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return Scope{}, response.BadRequestMsg(c, apperrors.ErrInvalidParam, "invalid site")
	}
	exists, err := h.sites(c.Context(), uint(id))
	if err != nil {
		slog.Error("settings: site lookup failed", "err", err)
		return Scope{}, response.InternalError(c, apperrors.ErrOperationFailed)
	}
	if !exists {
		return Scope{}, response.NotFoundMsg(c, apperrors.ErrNotFound, "unknown site")
	}
	return SiteScope(uint(id)), nil
}

func (h *Handler) Overview(c fiber.Ctx) error {
	scope, err := h.scopeFrom(c)
	if err != nil {
		return err
	}
	_, roles := callerFrom(c)
	ov, err := h.svc.Overview(c.Context(), h.canWrite(roles), scope)
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
	scope, err := h.scopeFrom(c)
	if err != nil {
		return err
	}
	var req setRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, apperrors.ErrBadRequest)
	}
	userID, _ := callerFrom(c)
	key := c.Params("key")
	view, err := h.svc.Set(c.Context(), userID, scope, key, req.Value, req.Note, req.Version)
	if err != nil {
		return h.writeErr(c, err, key)
	}
	return response.Success(c, view)
}

func (h *Handler) Reset(c fiber.Ctx) error {
	scope, err := h.scopeFrom(c)
	if err != nil {
		return err
	}
	userID, _ := callerFrom(c)
	key := c.Params("key")
	view, err := h.svc.Reset(c.Context(), userID, scope, key, c.Query("note"))
	if err != nil {
		return h.writeErr(c, err, key)
	}
	return response.Success(c, view)
}

func (h *Handler) Effective(c fiber.Ctx) error {
	view, err := h.svc.Effective(c.Context(), h.siteOf(c))
	if err != nil {
		slog.Error("settings: effective read failed", "err", err)
		return response.InternalError(c, apperrors.ErrOperationFailed)
	}
	c.Set("ETag", view.ETag)
	c.Set("Cache-Control", "no-cache")
	if etagMatches(c.Get("If-None-Match"), view.ETag) {
		c.Status(fiber.StatusNotModified)
		return nil
	}
	return response.Success(c, view)
}

func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

func (h *Handler) writeErr(c fiber.Ctx, err error, key string) error {
	switch {
	case errors.Is(err, ErrUnknownKey):
		return response.NotFoundMsg(c, apperrors.ErrNotFound, "unknown setting key")
	case errors.Is(err, ErrNotSiteScoped):
		return response.BadRequestMsg(c, apperrors.ErrInvalidParam, "该配置不支持按站点覆盖")
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
