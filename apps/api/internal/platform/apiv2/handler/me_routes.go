package handler

import (
	"context"
	"net/http"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"

	"github.com/danielgtaylor/huma/v2"
)

type listPlaytimesInput struct {
	collectionInput
	WorkIDs string `query:"work_ids" maxLength:"4096" doc:"Comma-separated work ids, max 100. Batch read, no pagination."`
}
type listPlaytimesOutput struct {
	Body repr.List[repr.UserPlaytime]
}
type getPlaytimeInput struct {
	WorkID string `path:"work_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
}
type getPlaytimeOutput struct {
	Body repr.UserPlaytime
}
type putPlaytimeInput struct {
	WorkID string `path:"work_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
	Body   struct {
		Minutes int `json:"minutes" minimum:"0" maximum:"60000" doc:"Absolute cumulative minutes."`
	}
}
type deletePlaytimeInput struct {
	WorkID string `path:"work_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog work id."`
}
type listCoverVotesOutput struct {
	Body repr.List[repr.CoverVote]
}
type putCoverVoteInput struct {
	CoverID string `path:"cover_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog cover row id."`
	Body    struct {
		Vote string `json:"vote" enum:"up" doc:"Only up is stored."`
	}
}
type deleteCoverVoteInput struct {
	CoverID string `path:"cover_id" minLength:"1" maxLength:"20" pattern:"^[0-9]+$" doc:"Catalog cover row id."`
}
type getCoverVoteOutput struct {
	Body repr.CoverVote
}

func registerMe(api huma.API, cat *Catalog) {
	me := []string{"me"}
	errs := collectionErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable)
	huma.Register(api, huma.Operation{
		OperationID: "listMyPlaytimes", Method: http.MethodGet, Path: "/v2/me/playtimes",
		Summary: "List my playtimes", Description: "The bearer user's playtime rows. work_ids= is a batch read. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, listMyPlaytimes(cat))
	huma.Register(api, huma.Operation{
		OperationID: "getMyPlaytime", Method: http.MethodGet, Path: "/v2/me/playtimes/{work_id}",
		Summary: "Get my playtime on one work", Description: "404 when the user has never reported. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, getMyPlaytime(cat))
	huma.Register(api, huma.Operation{
		OperationID: "putMyPlaytime", Method: http.MethodPut, Path: "/v2/me/playtimes/{work_id}",
		Summary: "Replace my playtime on one work", Description: "Absolute minutes. Naturally idempotent. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, putMyPlaytime(cat))
	huma.Register(api, huma.Operation{
		OperationID: "deleteMyPlaytime", Method: http.MethodDelete, Path: "/v2/me/playtimes/{work_id}",
		Summary: "Delete my playtime on one work", Description: "204 with no body. Requires a user access token.",
		Tags: me, Errors: errs, DefaultStatus: http.StatusNoContent, SkipValidateParams: true,
	}, deleteMyPlaytime(cat))
	huma.Register(api, huma.Operation{
		OperationID: "listMyCoverVotes", Method: http.MethodGet, Path: "/v2/me/cover-votes",
		Summary: "List my cover votes", Description: "Every cover the bearer has voted up. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, listMyCoverVotes(cat))
	huma.Register(api, huma.Operation{
		OperationID: "putMyCoverVote", Method: http.MethodPut, Path: "/v2/me/cover-votes/{cover_id}",
		Summary: "Cast a cover vote", Description: "Only vote=up is stored. One ballot per work. Requires a user access token.",
		Tags: me, Errors: errs, SkipValidateParams: true,
	}, putMyCoverVote(cat))
	huma.Register(api, huma.Operation{
		OperationID: "deleteMyCoverVote", Method: http.MethodDelete, Path: "/v2/me/cover-votes/{cover_id}",
		Summary: "Withdraw a cover vote", Description: "204 with no body. Requires a user access token.",
		Tags: me, Errors: errs, DefaultStatus: http.StatusNoContent, SkipValidateParams: true,
	}, deleteMyCoverVote(cat))
}

func listMyPlaytimes(cat *Catalog) func(context.Context, *listPlaytimesInput) (*listPlaytimesOutput, error) {
	return func(ctx context.Context, in *listPlaytimesInput) (*listPlaytimesOutput, error) {
		if in == nil {
			in = &listPlaytimesInput{}
		}
		q, err := parseCatalogList(ctx, &in.collectionInput, collect.PlaytimeSpec())
		if err != nil {
			return nil, err
		}
		page, lerr := cat.ListPlaytimes(ctx, q, splitWorkIDs(in.WorkIDs))
		if lerr != nil {
			return nil, catalogErr(ctx, lerr)
		}
		return &listPlaytimesOutput{Body: page}, nil
	}
}

func getMyPlaytime(cat *Catalog) func(context.Context, *getPlaytimeInput) (*getPlaytimeOutput, error) {
	return func(ctx context.Context, in *getPlaytimeInput) (*getPlaytimeOutput, error) {
		if in == nil {
			in = &getPlaytimeInput{}
		}
		id, ok := repr.ParseID(in.WorkID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.WorkID))
		}
		rec, err := cat.GetPlaytime(ctx, id)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getPlaytimeOutput{Body: rec}, nil
	}
}

func putMyPlaytime(cat *Catalog) func(context.Context, *putPlaytimeInput) (*getPlaytimeOutput, error) {
	return func(ctx context.Context, in *putPlaytimeInput) (*getPlaytimeOutput, error) {
		if in == nil {
			in = &putPlaytimeInput{}
		}
		id, ok := repr.ParseID(in.WorkID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.WorkID))
		}
		rec, err := cat.PutPlaytime(ctx, id, in.Body.Minutes)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getPlaytimeOutput{Body: rec}, nil
	}
}

func deleteMyPlaytime(cat *Catalog) func(context.Context, *deletePlaytimeInput) (*struct{}, error) {
	return func(ctx context.Context, in *deletePlaytimeInput) (*struct{}, error) {
		if in == nil {
			in = &deletePlaytimeInput{}
		}
		id, ok := repr.ParseID(in.WorkID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.WorkID))
		}
		if err := cat.DeletePlaytime(ctx, id); err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &struct{}{}, nil
	}
}

func listMyCoverVotes(cat *Catalog) func(context.Context, *struct{}) (*listCoverVotesOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*listCoverVotesOutput, error) {
		page, err := cat.ListCoverVotes(ctx)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &listCoverVotesOutput{Body: page}, nil
	}
}

func putMyCoverVote(cat *Catalog) func(context.Context, *putCoverVoteInput) (*getCoverVoteOutput, error) {
	return func(ctx context.Context, in *putCoverVoteInput) (*getCoverVoteOutput, error) {
		if in == nil {
			in = &putCoverVoteInput{}
		}
		id, ok := repr.ParseID(in.CoverID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.CoverID))
		}
		rec, err := cat.PutCoverVote(ctx, id, in.Body.Vote)
		if err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &getCoverVoteOutput{Body: rec}, nil
	}
}

func deleteMyCoverVote(cat *Catalog) func(context.Context, *deleteCoverVoteInput) (*struct{}, error) {
	return func(ctx context.Context, in *deleteCoverVoteInput) (*struct{}, error) {
		if in == nil {
			in = &deleteCoverVoteInput{}
		}
		id, ok := repr.ParseID(in.CoverID)
		if !ok {
			return nil, catalogErr(ctx, problemInvalidID(in.CoverID))
		}
		if err := cat.DeleteCoverVote(ctx, id); err != nil {
			return nil, catalogErr(ctx, err)
		}
		return &struct{}{}, nil
	}
}

func problemInvalidID(raw string) error {
	p := problem.New(problem.CodeInvalidParameter, "", "", "id must be a positive decimal catalog id.")
	p.Errors = []problem.FieldError{{Parameter: "id", Reason: problem.ReasonInvalidFormat, Detail: raw}}
	return p
}
