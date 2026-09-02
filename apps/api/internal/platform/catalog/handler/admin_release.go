package handler

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"
)

type releaseWorkInput struct {
	Body struct {
		WorkID int64  `json:"work_id" minimum:"1" doc:"The quarantined work to release to live"`
		Note   string `json:"note,omitempty"`
	}
}

type releaseWorkData struct {
	WorkID int64 `json:"work_id"`
	Status int16 `json:"status"`
}

type releaseWorkOutput struct {
	Body Envelope[releaseWorkData]
}

func (s *AdminServer) releaseWork(ctx context.Context, in *releaseWorkInput) (*releaseWorkOutput, error) {
	err := s.queues.ReleaseWork(ctx, in.Body.WorkID, adminIDFromCtx(ctx), in.Body.Note)
	if err != nil {
		switch {
		case stderrors.Is(err, service.ErrNotFound):
			return nil, apiErr(http.StatusNotFound, errors.ErrNotFound)
		case stderrors.Is(err, service.ErrProposalState):
			return nil, apiErrMsg(http.StatusConflict, errors.ErrOperationFailed, err.Error())
		}
		slog.Error("catalog admin release work", "err", err)
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	return &releaseWorkOutput{Body: okEnvelope(releaseWorkData{
		WorkID: in.Body.WorkID, Status: model.WorkStatusLive,
	})}, nil
}
