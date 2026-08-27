package devapi

import (
	goerrors "errors"

	devapiPerm "api/internal/platform/devapi/perm"
	apperr "api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type policyMatrixView struct {
	Editable bool         `json:"editable"`
	Policies []PolicyView `json:"policies"`
}

type setPolicyRequest struct {
	Mode string `json:"mode"`
}

func (h *AdminHandler) ListPolicies(c fiber.Ctx) error {
	policies, err := h.svc.Policies(c.Context())
	if err != nil {
		return response.InternalError(c, apperr.ErrOperationFailed)
	}
	roles, _ := c.Locals("user_roles").([]string)
	return response.Success(c, policyMatrixView{
		Editable: devapiPerm.Resolver.Can(roles, devapiPerm.PolicyManage),
		Policies: policies,
	})
}

func (h *AdminHandler) SetPolicy(c fiber.Ctx) error {
	var req setPolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, apperr.ErrBadRequest)
	}
	actor, _ := c.Locals("user_id").(uint)
	err := h.svc.SetPolicy(c.Context(), c.Params("capability"), req.Mode, actor)
	if msg, bad := policyBadRequest(err); bad {
		return response.BadRequestMsg(c, apperr.ErrValidationFailed, msg)
	}
	if err != nil {
		return response.InternalError(c, apperr.ErrOperationFailed)
	}
	return h.ListPolicies(c)
}

func (h *AdminHandler) ResetPolicy(c fiber.Ctx) error {
	actor, _ := c.Locals("user_id").(uint)
	err := h.svc.ResetPolicy(c.Context(), c.Params("capability"), actor)
	if msg, bad := policyBadRequest(err); bad {
		return response.BadRequestMsg(c, apperr.ErrValidationFailed, msg)
	}
	if err != nil {
		return response.InternalError(c, apperr.ErrOperationFailed)
	}
	return h.ListPolicies(c)
}

func policyBadRequest(err error) (string, bool) {
	switch {
	case goerrors.Is(err, ErrUnknownCapability):
		return "unknown capability", true
	case goerrors.Is(err, ErrInvalidPolicyMode):
		return "mode not allowed for this capability", true
	default:
		return "", false
	}
}
