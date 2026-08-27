package handler

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

func claimErr(err error) error {
	var (
		transition *service.ClaimTransitionError
		ownership  *service.ClaimOwnershipError
		notOwned   *service.ClaimNotOwnedError
	)
	switch {
	case stderrors.Is(err, gorm.ErrRecordNotFound):
		return apiErrMsg(http.StatusNotFound, errors.ErrNotFound, "work not found")
	case stderrors.As(err, &transition):
		return apiErrData(http.StatusConflict, errors.ErrOperationFailed, transition.Error(),
			dto.ClaimTransitionInfo{CurrentState: transition.Current, AllowedFrom: transition.Allowed})
	case stderrors.As(err, &ownership):
		return apiErrMsg(http.StatusForbidden, errors.ErrForbidden, ownership.Error())
	case stderrors.As(err, &notOwned):
		return apiErrMsg(http.StatusForbidden, errors.ErrForbidden, notOwned.Error())
	case stderrors.Is(err, service.ErrClaimReasonRequired),
		stderrors.Is(err, service.ErrClaimTargetRequired):
		return apiErrMsg(http.StatusUnprocessableEntity, errors.ErrValidationFailed, err.Error())
	}
	slog.Error("catalog claim action", "err", err)
	return apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
}

func (s *AdminServer) registerClaims(api huma.API) {
	tags := []string{"catalog-admin"}
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogPendingClaims", Method: http.MethodGet,
		Path:    "/api/v1/admin/catalog/claims/pending",
		Summary: "The claim review queue: submissions awaiting a decision, oldest submission first",
		Tags:    tags,
	}, s.listPendingClaims)
	huma.Register(api, huma.Operation{
		OperationID: "actOnCatalogClaimAsStaff", Method: http.MethodPost,
		Path:    "/api/v1/admin/catalog/claims/{id}/{action}",
		Summary: "Approve / decline / ban / unban a claim (decline and ban record the reason on the event)",
		Tags:    tags,
	}, s.actOnClaim)
}

type pendingClaimsInput struct {
	Site  string `query:"site" doc:"Restrict to one claiming site"`
	Limit int    `query:"limit" doc:"Items per page (max 200)"`
}

type pendingClaimsOutput struct {
	Body Envelope[dto.Page[service.PendingClaimItem]]
}

func (s *AdminServer) listPendingClaims(ctx context.Context, in *pendingClaimsInput) (*pendingClaimsOutput, error) {
	items, total, err := s.claims.PendingClaims(ctx, in.Site, in.Limit)
	if err != nil {
		slog.Error("catalog admin pending claims", "err", err)
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	return &pendingClaimsOutput{Body: okEnvelope(dto.Page[service.PendingClaimItem]{Items: items, Total: total})}, nil
}

type staffClaimActionInput struct {
	ID     int64  `path:"id" minimum:"1"`
	Action string `path:"action" doc:"approve | decline | ban | unban"`
	Body   dto.StaffClaimActionRequest
}

type staffClaimActionOutput struct {
	Body Envelope[service.ClaimActionResult]
}

func (s *AdminServer) actOnClaim(ctx context.Context, in *staffClaimActionInput) (*staffClaimActionOutput, error) {
	action := service.ClaimAction(in.Action)
	if !service.ReviewActions[action] {
		return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam,
			"unknown review action "+in.Action+" (approve, decline, ban, unban)")
	}
	res, err := s.claims.Act(ctx, service.ClaimActionParams{
		WorkID: in.ID, Action: action,
		ActorUID: adminIDFromCtx(ctx), Reason: in.Body.Reason,
	})
	if err != nil {
		return nil, claimErr(err)
	}
	return &staffClaimActionOutput{Body: okEnvelope(*res)}, nil
}
