package handler

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

type PlaytimeServer struct{ svc *service.UserPlaytimeService }

func SetupPlaytime(app *fiber.App, svc *service.UserPlaytimeService) huma.API {
	InstallErrorEnvelope()

	cfg := huma.DefaultConfig("KUN Playtime API", "1.0.0")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""

	api := humafiber.New(app, cfg)
	api.UseMiddleware(PlaytimeBridge)
	RegisterPlaytimeOps(api, svc)
	return api
}

func RegisterPlaytimeOps(api huma.API, svc *service.UserPlaytimeService) {
	s := &PlaytimeServer{svc: svc}
	tags := []string{"playtime"}

	huma.Register(api, huma.Operation{
		OperationID: "reportPlaytime", Method: http.MethodPut,
		Path:    PlaytimePrefix + "/works/{workID}",
		Summary: "Report the bearer token's own playtime on a work. The body carries the ABSOLUTE cumulative total in minutes, never a delta — re-sending the same number is a no-op, which makes the call safe to retry. Keyed by (user, work, client): a second app of the same user reports alongside, not over. Any app with a user access token may call this; playtime:write is not required.",
		Tags:    tags,
	}, s.report)

	huma.Register(api, huma.Operation{
		OperationID: "reportPlaytimeByRef", Method: http.MethodPut,
		Path:    PlaytimePrefix + "/by-ref/{source}/{externalID}",
		Summary: "Report playtime addressing the work by an external id the client already holds (vndb/dlsite/getchu/bangumi …) instead of a catalog work id. Only EXACT anchors resolve; the response echoes the resolved work_id, which the client should cache. 404 when nothing is anchored to that id. Any app with a user access token may call this; playtime:write is not required.",
		Tags:    tags,
	}, s.reportByRef)

	huma.Register(api, huma.Operation{
		OperationID: "reportPlaytimeBatch", Method: http.MethodPost,
		Path:    PlaytimePrefix + "/batch",
		Summary: "Report up to 200 works in one call — the first-login library sync. Each item is accepted or rejected on its own and the response reports per-item outcomes; a single bad item never fails the batch. Any app with a user access token may call this; playtime:write is not required.",
		Tags:    tags,
	}, s.reportBatch)

	huma.Register(api, huma.Operation{
		OperationID: "listOwnPlaytime", Method: http.MethodGet,
		Path:    PlaytimePrefix + "/mine",
		Summary: "Page the bearer token's own playtime rows in (updated_at) order — the sync-back leg for a second device. Hand `cursor` back as ?updated_since= to fetch only what changed. Any app with a user access token may call this; playtime:read is not required.",
		Tags:    tags,
	}, s.listMine)

	huma.Register(api, huma.Operation{
		OperationID: "getOwnPlaytimeForWork", Method: http.MethodGet,
		Path:    PlaytimePrefix + "/works/{workID}",
		Summary: "The bearer token's own playtime on ONE work, folded across their applications (MAX minutes — two apps watching one save file are not two playthroughs). `playtime` is null when the user has never reported here; that is a 200, not a 404. This is the call a rating form makes to offer 'you played 30h — attach it?'. Any app with a user access token may call this; playtime:read is not required.",
		Tags:    tags,
	}, s.getMine)
}

func playtimeErr(err error) error {
	switch {
	case stderrors.Is(err, service.ErrPlaytimeMinutesRange),
		stderrors.Is(err, service.ErrPlaytimeBadStatus),
		stderrors.Is(err, service.ErrPlaytimeUnknownSource):
		return apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam, err.Error())
	case stderrors.Is(err, service.ErrPlaytimeActorRequired),
		stderrors.Is(err, service.ErrPlaytimeClientRequired):
		return apiErrMsg(http.StatusForbidden, errors.ErrForbidden, err.Error())
	case stderrors.Is(err, service.ErrPlaytimeWorkUnavailable),
		stderrors.Is(err, service.ErrPlaytimeRefUnresolved):
		return apiErrMsg(http.StatusNotFound, errors.ErrNotFound, err.Error())
	}
	slog.Error("catalog playtime", "err", err)
	return apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
}

type playtimeReportInput struct {
	WorkID int64 `path:"workID" minimum:"1" doc:"Catalog work id"`
	Body   dto.PlaytimeReportBody
}

type playtimeRecordOutput struct {
	Body Envelope[dto.PlaytimeRecordResponse]
}

func (s *PlaytimeServer) report(ctx context.Context, in *playtimeReportInput) (*playtimeRecordOutput, error) {
	uid, clientID, he := playtimeActor(ctx)
	if he != nil {
		return nil, he
	}
	rec, err := s.store(ctx, uid, clientID, in.WorkID, in.Body)
	if err != nil {
		return nil, err
	}
	return &playtimeRecordOutput{Body: okEnvelope(*rec)}, nil
}

type playtimeByRefInput struct {
	Source     string `path:"source" doc:"External source key (vndb, dlsite, getchu, bangumi, …)"`
	ExternalID string `path:"externalID" doc:"The id this game carries on that source (e.g. v17, RJ01234)"`
	Body       dto.PlaytimeReportBody
}

func (s *PlaytimeServer) reportByRef(ctx context.Context, in *playtimeByRefInput) (*playtimeRecordOutput, error) {
	uid, clientID, he := playtimeActor(ctx)
	if he != nil {
		return nil, he
	}
	workID, err := s.svc.ResolveRef(ctx, in.Source, in.ExternalID)
	if err != nil {
		return nil, playtimeErr(err)
	}
	rec, err := s.store(ctx, uid, clientID, workID, in.Body)
	if err != nil {
		return nil, err
	}
	rec.ResolvedFrom = in.Source + ":" + in.ExternalID
	return &playtimeRecordOutput{Body: okEnvelope(*rec)}, nil
}

func (s *PlaytimeServer) store(ctx context.Context, uid int64, clientID string,
	workID int64, body dto.PlaytimeReportBody) (*dto.PlaytimeRecordResponse, error) {

	status, lastPlayed, he := decodeReportBody(body)
	if he != nil {
		return nil, he
	}
	rec, err := s.svc.Report(ctx, service.PlaytimeReport{
		ActorUID: uid, WorkID: workID, ClientID: clientID,
		Minutes: body.Minutes, Status: status, LastPlayedAt: lastPlayed,
	})
	if err != nil {
		return nil, playtimeErr(err)
	}
	out := renderPlaytimeRecord(*rec)
	return &out, nil
}

func decodeReportBody(body dto.PlaytimeReportBody) (int16, *time.Time, *houseError) {
	status, ok := dto.PlaytimeStatusFromWord(orDefault(body.Status, dto.PlaytimeStatusWordPlaying))
	if !ok {
		return 0, nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam,
			"unknown playtime status "+body.Status)
	}
	var lastPlayed *time.Time
	if body.LastPlayedAt != nil && *body.LastPlayedAt != "" {
		t, err := time.Parse(time.RFC3339, *body.LastPlayedAt)
		if err != nil {
			return 0, nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam,
				"last_played_at must be an RFC 3339 timestamp")
		}
		lastPlayed = &t
	}
	return status, lastPlayed, nil
}

type playtimeBatchInput struct {
	Body dto.PlaytimeBatchBody
}

type playtimeBatchOutput struct {
	Body Envelope[dto.PlaytimeBatchResponse]
}

func (s *PlaytimeServer) reportBatch(ctx context.Context, in *playtimeBatchInput) (*playtimeBatchOutput, error) {
	uid, clientID, he := playtimeActor(ctx)
	if he != nil {
		return nil, he
	}

	out := dto.PlaytimeBatchResponse{Results: make([]dto.PlaytimeBatchResult, 0, len(in.Body.Items))}
	for i, item := range in.Body.Items {
		res := dto.PlaytimeBatchResult{Index: i}
		workID, err := s.resolveBatchTarget(ctx, item)
		if err != nil {
			res.Status, res.Error = classifyBatchErr(err), err.Error()
			out.Refused++
			out.Results = append(out.Results, res)
			continue
		}
		status, lastPlayed, he := decodeReportBody(dto.PlaytimeReportBody{
			Minutes: item.Minutes, Status: item.Status, LastPlayedAt: item.LastPlayedAt,
		})
		if he != nil {
			res.Status, res.Error, res.WorkID = "rejected", he.Message, workID
			out.Refused++
			out.Results = append(out.Results, res)
			continue
		}
		if _, err := s.svc.Report(ctx, service.PlaytimeReport{
			ActorUID: uid, WorkID: workID, ClientID: clientID,
			Minutes: item.Minutes, Status: status, LastPlayedAt: lastPlayed,
		}); err != nil {
			res.Status, res.Error, res.WorkID = classifyBatchErr(err), err.Error(), workID
			out.Refused++
			out.Results = append(out.Results, res)
			continue
		}
		res.Status, res.WorkID = "ok", workID
		out.Accepted++
		out.Results = append(out.Results, res)
	}
	return &playtimeBatchOutput{Body: okEnvelope(out)}, nil
}

func (s *PlaytimeServer) resolveBatchTarget(ctx context.Context, item dto.PlaytimeBatchItem) (int64, error) {
	if item.WorkID > 0 {
		return item.WorkID, nil
	}
	if item.Source == "" || item.ExternalID == "" {
		return 0, stderrors.New("each item needs work_id, or both source and external_id")
	}
	return s.svc.ResolveRef(ctx, item.Source, item.ExternalID)
}

func classifyBatchErr(err error) string {
	if stderrors.Is(err, service.ErrPlaytimeWorkUnavailable) ||
		stderrors.Is(err, service.ErrPlaytimeRefUnresolved) ||
		stderrors.Is(err, service.ErrPlaytimeUnknownSource) {
		return "not_found"
	}
	return "rejected"
}

type playtimeMineInput struct {
	UpdatedSince string `query:"updated_since" doc:"RFC 3339 timestamp; returns only rows changed AFTER it. Omit for a full first pull"`
	Limit        int    `query:"limit" minimum:"1" maximum:"200" default:"200" doc:"Page size"`
}

type playtimeMineOutput struct {
	Body Envelope[dto.PlaytimeMineResponse]
}

func (s *PlaytimeServer) listMine(ctx context.Context, in *playtimeMineInput) (*playtimeMineOutput, error) {
	uid := userIDFromCtx(ctx)
	var since time.Time
	if in.UpdatedSince != "" {
		t, err := time.Parse(time.RFC3339, in.UpdatedSince)
		if err != nil {
			return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam,
				"updated_since must be an RFC 3339 timestamp")
		}
		since = t
	}
	limit := in.Limit
	if limit <= 0 || limit > playtimeMinePageSize {
		limit = playtimeMinePageSize
	}
	rows, err := s.svc.ListMine(ctx, uid, since, limit)
	if err != nil {
		return nil, playtimeErr(err)
	}
	out := dto.PlaytimeMineResponse{Items: make([]dto.PlaytimeRecordResponse, 0, len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, renderPlaytimeRecord(r))
	}
	if n := len(rows); n > 0 {
		cursor := rows[n-1].UpdatedAt.UTC().Format(time.RFC3339Nano)
		out.Cursor = &cursor
	}
	return &playtimeMineOutput{Body: okEnvelope(out)}, nil
}

type playtimeSelfInput struct {
	WorkID int64 `path:"workID" minimum:"1" doc:"Catalog work id"`
}

type playtimeSelfOutput struct {
	Body Envelope[*dto.PlaytimeSelfResponse]
}

func (s *PlaytimeServer) getMine(ctx context.Context, in *playtimeSelfInput) (*playtimeSelfOutput, error) {
	uid := userIDFromCtx(ctx)
	got, err := s.svc.GetMine(ctx, uid, in.WorkID)
	if err != nil {
		return nil, playtimeErr(err)
	}
	if got == nil {
		return &playtimeSelfOutput{Body: okEnvelope[*dto.PlaytimeSelfResponse](nil)}, nil
	}
	return &playtimeSelfOutput{Body: okEnvelope(&dto.PlaytimeSelfResponse{
		WorkID:       got.WorkID,
		Minutes:      got.Minutes,
		Status:       dto.PlaytimeStatusWord(got.Status),
		LastPlayedAt: renderTimePtr(got.LastPlayedAt),
		Clients:      got.Clients,
	})}, nil
}

func renderPlaytimeRecord(r service.PlaytimeRecord) dto.PlaytimeRecordResponse {
	return dto.PlaytimeRecordResponse{
		WorkID:       r.WorkID,
		Minutes:      r.Minutes,
		Status:       dto.PlaytimeStatusWord(r.Status),
		LastPlayedAt: renderTimePtr(r.LastPlayedAt),
		ClientID:     r.ClientID,
		UpdatedAt:    r.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func renderTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

func orDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
