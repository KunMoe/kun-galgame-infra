package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	newsmodel "api/internal/platform/news/model"

	"github.com/stretchr/testify/require"
)

type newsBodyView struct {
	Object     string   `json:"object"`
	ID         string   `json:"id"`
	Lane       string   `json:"lane"`
	Status     string   `json:"status"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	SourceURL  string   `json:"source_url"`
	BannerHash string   `json:"banner_hash"`
	WorkIDs    []string `json:"work_ids"`
	Source     struct {
		Name string `json:"name"`
	} `json:"source"`
}

func liveDoFull(t *testing.T, env *liveEnv, method, path, token, body string, extra map[string]string) (int, http.Header, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := env.app.Test(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header, raw
}

func liveProblem(t *testing.T, raw []byte) problem.Problem {
	t.Helper()
	var p problem.Problem
	require.NoError(t, json.Unmarshal(raw, &p), string(raw))
	return p
}

func liveNewsBody(t *testing.T, raw []byte) newsBodyView {
	t.Helper()
	var v newsBodyView
	require.NoError(t, json.Unmarshal(raw, &v), string(raw))
	return v
}

func liveSubmit(t *testing.T, env *liveEnv, title string) newsBodyView {
	t.Helper()
	status, _, raw := liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","title":"`+title+`","summary":"A lede","source_url":"https://example.test/x"}`)
	require.Equal(t, 201, status, string(raw))
	return liveNewsBody(t, raw)
}

func TestLiveNewsSourceIsTheGrant(t *testing.T) {
	env := liveCatalog(t)

	status, _, raw := liveDo(t, env, http.MethodGet, "/v2/me/news", liveAppKey, "")
	require.Equal(t, 401, status, string(raw))

	for _, source := range []string{"no_such_source", "ymgal"} {
		status, _, raw = liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
			`{"source":"`+source+`","title":"t","summary":"s","source_url":"https://example.test/x"}`)
		require.Equal(t, 403, status, source+": "+string(raw))
		require.Equal(t, problem.CodeSourceNotYours, liveProblem(t, raw).Code, source)
	}

	require.NoError(t, env.db.Exec(`
		INSERT INTO news_source (key, display_name, homepage_url, attribution, publisher_uid, column_url, active)
		VALUES ('dozing', 'Dozing', 'https://example.test', 'a', ?, '', false)
		ON CONFLICT (key) DO UPDATE SET publisher_uid = EXCLUDED.publisher_uid, active = false`, liveUID).Error)
	status, _, raw = liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"dozing","title":"t","summary":"s","source_url":"https://example.test/x"}`)
	require.Equal(t, 422, status, string(raw))
	require.Equal(t, problem.CodeSourceInactive, liveProblem(t, raw).Code)
}

func TestLiveNewsCreateLandsPending(t *testing.T) {
	env := liveCatalog(t)

	status, hdr, raw := liveDoFull(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","lane":"column","title":"Fresh","summary":"A lede",`+
			`"source_url":"https://example.test/fresh","published_at":"2026-08-25T12:00:00Z",`+
			`"work_ids":["`+idstr(env.fx.Work)+`"]}`, nil)
	require.Equal(t, 201, status, string(raw))
	rec := liveNewsBody(t, raw)
	require.Equal(t, "news_submission", rec.Object)
	require.Equal(t, "pending", rec.Status)
	require.Equal(t, "column", rec.Lane)
	require.Equal(t, liveNewsSource, rec.Source.Name)
	require.Equal(t, []string{idstr(env.fx.Work)}, rec.WorkIDs)
	require.Equal(t, "/v2/me/news/"+rec.ID, hdr.Get("Location"))
	require.NotEmpty(t, hdr.Get("ETag"))
	require.Equal(t, "private, no-store", hdr.Get("Cache-Control"))

	status, hdr, raw = liveDoFull(t, env, http.MethodGet, "/v2/me/news/"+rec.ID, liveUserToken, "", nil)
	require.Equal(t, 200, status, string(raw))
	require.NotEmpty(t, hdr.Get("ETag"))
	require.Equal(t, "private, no-store", hdr.Get("Cache-Control"))
	require.Equal(t, rec.ID, liveNewsBody(t, raw).ID)

	status, _, raw = liveDo(t, env, http.MethodGet, "/v2/me/news?limit=100", liveUserToken, "")
	require.Equal(t, 200, status, string(raw))
	var page struct {
		Items []newsBodyView `json:"items"`
	}
	require.NoError(t, json.Unmarshal(raw, &page))
	seen := false
	for _, it := range page.Items {
		if it.ID == rec.ID {
			seen = true
		}
	}
	require.True(t, seen, "a publisher must see their own pending item")
}

// TestLiveNewsPendingIsInvisibleOnThePublicFace is the same obligation the read
// face already enforces, asserted from the write side: a POST is not a publish.
func TestLiveNewsPendingIsInvisibleOnThePublicFace(t *testing.T) {
	env := liveCatalog(t)
	rec := liveSubmit(t, env, "Still pending")

	status, _, raw := liveDo(t, env, http.MethodGet, "/v2/news/"+rec.ID, "", "")
	require.Equal(t, 404, status, string(raw))

	status, _, raw = liveDo(t, env, http.MethodGet, "/v2/news?limit=100", "", "")
	require.Equal(t, 200, status, string(raw))
	require.NotContains(t, string(raw), `"id":"`+rec.ID+`"`)
	require.Contains(t, string(raw), `"id":"`+idstr(env.fx.NewsItem)+`"`,
		"control: the seeded published item is on the public face")
}

func TestLiveNewsValidationIsFieldLevel(t *testing.T) {
	env := liveCatalog(t)

	status, _, raw := liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","title":"t","summary":"`+strings.Repeat("a", newsmodel.PreviewMaxRunes+1)+`","source_url":"https://example.test/x"}`)
	require.Equal(t, 422, status, string(raw))
	p := liveProblem(t, raw)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.NotEmpty(t, p.Errors)
	require.Equal(t, "/summary", p.Errors[0].Pointer)
	require.Equal(t, problem.ReasonTooLong, p.Errors[0].Reason)

	status, _, raw = liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","title":"t","summary":"`+strings.Repeat("a", newsmodel.PreviewMaxRunes)+`","source_url":"https://example.test/x"}`)
	require.Equal(t, 201, status, "exactly 200 runes is the ceiling, not one past it: "+string(raw))

	status, _, raw = liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","title":"t","summary":"s","source_url":"not a url","work_ids":["abc","`+idstr(env.fx.Work)+`"]}`)
	require.Equal(t, 422, status, string(raw))
	p = liveProblem(t, raw)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	pointers := map[string]string{}
	for _, e := range p.Errors {
		pointers[e.Pointer] = e.Reason
	}
	require.Equal(t, problem.ReasonInvalidFormat, pointers["/source_url"])
	require.Equal(t, problem.ReasonInvalidFormat, pointers["/work_ids/0"], "every failure at once, not one round trip each")

	// banner_hash is refused by the generated schema before the handler runs, so
	// it is asserted on its own: mixing it into the case above hides every other
	// field's error behind huma's single schema failure.
	status, _, raw = liveDo(t, env, http.MethodPost, "/v2/me/news", liveUserToken,
		`{"source":"`+liveNewsSource+`","title":"t","summary":"s","source_url":"https://example.test/x","banner_hash":"zz"}`)
	require.Equal(t, 422, status, string(raw))
	p = liveProblem(t, raw)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.NotEmpty(t, p.Errors)
	require.Equal(t, "/banner_hash", p.Errors[0].Pointer)
}

func TestLiveNewsIdempotencyKeyReplays(t *testing.T) {
	env := liveCatalog(t)
	body := `{"source":"` + liveNewsSource + `","title":"Once","summary":"A lede","source_url":"https://example.test/once"}`
	key := map[string]string{"Idempotency-Key": "01K3QY8Z9T7M4A2BXWQ7RN2C4H"}

	status, hdr, raw := liveDoFull(t, env, http.MethodPost, "/v2/me/news", liveUserToken, body, key)
	require.Equal(t, 201, status, string(raw))
	require.Empty(t, hdr.Get("Idempotency-Replayed"))
	first := liveNewsBody(t, raw).ID

	status, hdr, raw = liveDoFull(t, env, http.MethodPost, "/v2/me/news", liveUserToken, body, key)
	require.Equal(t, 201, status, string(raw))
	require.Equal(t, "true", hdr.Get("Idempotency-Replayed"))
	require.Equal(t, first, liveNewsBody(t, raw).ID, "a replay must not mint a second item")
}

func TestLiveNewsPendingEditThenWithdraw(t *testing.T) {
	env := liveCatalog(t)
	rec := liveSubmit(t, env, "Draft title")

	status, _, raw := liveDo(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken, `{}`)
	require.Equal(t, 422, status, "an empty patch changes nothing: "+string(raw))

	status, hdr, raw := liveDoFull(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken,
		`{"title":"Edited title","work_ids":[]}`, nil)
	require.Equal(t, 200, status, string(raw))
	require.Equal(t, "Edited title", liveNewsBody(t, raw).Title)
	require.Equal(t, "pending", liveNewsBody(t, raw).Status)
	require.Equal(t, "private, no-store", hdr.Get("Cache-Control"))

	status, _, raw = liveDo(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken, `{"status":"withdrawn"}`)
	require.Equal(t, 428, status, "If-Match is mandatory on the withdrawal: "+string(raw))
	require.Equal(t, problem.CodePreconditionRequired, liveProblem(t, raw).Code)

	id, ok := repr.ParseID(rec.ID)
	require.True(t, ok)
	require.NoError(t, env.db.Exec(`UPDATE news_item SET status = ? WHERE id = ?`, newsmodel.StatusPublished, id).Error)

	status, hdr, raw = liveDoFull(t, env, http.MethodGet, "/v2/me/news/"+rec.ID, liveUserToken, "", nil)
	require.Equal(t, 200, status, string(raw))
	etag := hdr.Get("ETag")
	require.NotEmpty(t, etag)

	status, _, raw = liveDoFull(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken,
		`{"title":"Too late"}`, nil)
	require.Equal(t, 409, status, "text is editable only while pending: "+string(raw))
	require.Equal(t, problem.CodeInvalidStateTransition, liveProblem(t, raw).Code)

	status, _, raw = liveDoFull(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken,
		`{"status":"withdrawn"}`, map[string]string{"If-Match": `"n` + rec.ID + `.0"`})
	require.Equal(t, 412, status, string(raw))
	require.Equal(t, problem.CodePreconditionFailed, liveProblem(t, raw).Code)

	status, _, raw = liveDoFull(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken,
		`{"status":"withdrawn"}`, map[string]string{"If-Match": etag})
	require.Equal(t, 200, status, string(raw))
	require.Equal(t, "withdrawn", liveNewsBody(t, raw).Status)

	status, _, raw = liveDo(t, env, http.MethodGet, "/v2/news?limit=100", "", "")
	require.Equal(t, 200, status)
	require.NotContains(t, string(raw), `"id":"`+rec.ID+`"`, "a withdrawn item leaves the public list")
}

func TestLiveNewsRejectedIsTerminal(t *testing.T) {
	env := liveCatalog(t)
	rec := liveSubmit(t, env, "Doomed")
	id, ok := repr.ParseID(rec.ID)
	require.True(t, ok)
	require.NoError(t, env.db.Exec(`UPDATE news_item SET status = ? WHERE id = ?`, newsmodel.StatusRejected, id).Error)

	status, hdr, raw := liveDoFull(t, env, http.MethodGet, "/v2/me/news/"+rec.ID, liveUserToken, "", nil)
	require.Equal(t, 200, status, string(raw))
	require.Equal(t, "rejected", liveNewsBody(t, raw).Status)
	etag := hdr.Get("ETag")

	for _, body := range []string{`{"title":"Reborn"}`, `{"status":"withdrawn"}`} {
		status, _, raw = liveDoFull(t, env, http.MethodPatch, "/v2/me/news/"+rec.ID, liveUserToken, body,
			map[string]string{"If-Match": etag})
		require.Equal(t, 409, status, body+": "+string(raw))
		require.Equal(t, problem.CodeInvalidStateTransition, liveProblem(t, raw).Code, body)
	}
}

func TestLiveNewsIsFencedToTheCaller(t *testing.T) {
	env := liveCatalog(t)
	require.NoError(t, env.db.Exec(`
		INSERT INTO news_source (key, display_name, homepage_url, attribution, publisher_uid, column_url, active)
		VALUES ('stranger', 'Stranger', 'https://example.test', 'a', 987654, '', true)
		ON CONFLICT (key) DO UPDATE SET publisher_uid = EXCLUDED.publisher_uid, active = true`).Error)
	require.NoError(t, env.db.Exec(`
		INSERT INTO news_item (source_key, lane, upstream_category, external_id, title, preview,
			source_url, banner_hash, banner_origin_url, published_at, status)
		VALUES ('stranger', 'news', '', 'stranger-1', 't', 'p', 'https://example.test/s', '', '', now(), ?)`,
		newsmodel.StatusPublished).Error)
	var strangerID int64
	require.NoError(t, env.db.Raw(`SELECT id FROM news_item WHERE external_id = 'stranger-1'`).Scan(&strangerID).Error)

	status, _, raw := liveDo(t, env, http.MethodGet, "/v2/me/news/"+idstr(strangerID), liveUserToken, "")
	require.Equal(t, 404, status, string(raw))
	require.Equal(t, problem.CodeNotFound, liveProblem(t, raw).Code)

	status, _, raw = liveDo(t, env, http.MethodPatch, "/v2/me/news/"+idstr(strangerID), liveUserToken, `{"title":"mine now"}`)
	require.Equal(t, 404, status, string(raw))

	status, _, raw = liveDo(t, env, http.MethodGet, "/v2/news/"+idstr(strangerID), "", "")
	require.Equal(t, 200, status, "control: the item exists and is public, it is just not the caller's: "+string(raw))
}
