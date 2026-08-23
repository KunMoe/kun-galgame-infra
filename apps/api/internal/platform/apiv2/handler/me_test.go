package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"

	"github.com/stretchr/testify/require"
)

func TestMePlaytimesRequireUserToken(t *testing.T) {
	app := testApp(t)
	status, ct, body := do(t, app, http.MethodGet, "/v2/me/playtimes")
	require.Equal(t, 401, status)
	require.Contains(t, ct, "application/problem+json")
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p), string(body))
	require.Equal(t, problem.CodeMissingCredential, p.Code)

	req := httptest.NewRequest(http.MethodGet, "/v2/me/playtimes", nil)
	req.Header.Set("Authorization", "Bearer nm_live_notauser")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
}

func TestPlaytimeUnboundWithUser(t *testing.T) {
	ctx := contextWithUser(t.Context(), 7, "client-a")
	_, err := (*Catalog)(nil).ListPlaytimes(ctx, collect.Query{}, nil)
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
	_, err = (&Catalog{}).GetPlaytime(ctx, 1)
	if p, ok = err.(*problem.Problem); !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}

func TestClaimsUnbound(t *testing.T) {
	ctx := contextWithUser(t.Context(), 7, "client-a")
	_, err := (*Catalog)(nil).ListMyClaims(ctx, collect.Query{})
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}

func TestCoverVoteUnbound(t *testing.T) {
	ctx := contextWithUser(t.Context(), 7, "client-a")
	_, err := (*Catalog)(nil).ListCoverVotes(ctx)
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}

func contextWithUser(ctx context.Context, uid int64, client string) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, uid)
	return context.WithValue(ctx, ctxClientID, client)
}
