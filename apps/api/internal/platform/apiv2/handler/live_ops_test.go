package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func liveOpMethod(item *huma.PathItem, op *huma.Operation) string {
	if op.Method != "" {
		return op.Method
	}
	switch {
	case item.Get == op:
		return http.MethodGet
	case item.Put == op:
		return http.MethodPut
	case item.Post == op:
		return http.MethodPost
	case item.Delete == op:
		return http.MethodDelete
	case item.Patch == op:
		return http.MethodPatch
	default:
		return http.MethodGet
	}
}

func TestLiveReads200(t *testing.T) {
	env := liveCatalog(t)
	fx := env.fx
	gets := []string{
		"/v2/problems",
		"/v2/vocabularies",
		"/v2/vocabularies/medium",
		"/v2/problems/" + problem.CodeRateLimited,
		"/v2/problems/reasons",
		"/v2/catalog/stats",
		"/v2/catalog/schemas/work",
		"/v2/catalog/works",
		"/v2/catalog/works/" + idstr(fx.Work),
		"/v2/catalog/works/" + idstr(fx.Work) + "/covers",
		"/v2/catalog/works/" + idstr(fx.Work) + "/screenshots",
		"/v2/catalog/works/" + idstr(fx.Work) + "/tags",
		"/v2/catalog/works/" + idstr(fx.Work) + "/characters",
		"/v2/catalog/works/" + idstr(fx.Work) + "/credits",
		"/v2/catalog/works/" + idstr(fx.Work) + "/releases",
		"/v2/catalog/works/" + idstr(fx.Work) + "/intros",
		"/v2/catalog/works/" + idstr(fx.Work) + "/ratings",
		"/v2/catalog/works/" + idstr(fx.Work) + "/relations",
		"/v2/catalog/works/" + idstr(fx.Work) + "/series",
		"/v2/catalog/works/" + idstr(fx.Work) + "/links",
		"/v2/catalog/works/" + idstr(fx.Work) + "/engines",
		"/v2/catalog/companies",
		"/v2/catalog/companies/" + idstr(fx.Company),
		"/v2/catalog/companies/" + idstr(fx.Company) + "/graph",
		"/v2/catalog/tags",
		"/v2/catalog/tags/" + idstr(fx.Tag),
		"/v2/catalog/series",
		"/v2/catalog/series/" + idstr(fx.Series),
		"/v2/catalog/engines",
		"/v2/catalog/engines/" + idstr(fx.Engine),
		"/v2/catalog/releases",
		"/v2/catalog/releases/" + idstr(fx.Release),
		"/v2/catalog/characters",
		"/v2/catalog/characters/" + idstr(fx.Character),
		"/v2/catalog/characters/" + idstr(fx.Character) + "?view=full",
		"/v2/catalog/credit-names",
		"/v2/catalog/credit-names/" + idstr(fx.Credit),
		"/v2/catalog/credit-names/" + idstr(fx.Credit) + "/credits",
		"/v2/catalog/persons",
		"/v2/catalog/persons/" + idstr(fx.Person),
		"/v2/catalog/persons/" + idstr(fx.Person) + "/credit-names",
		"/v2/catalog/traits",
		"/v2/catalog/traits/" + idstr(fx.Trait),
		"/v2/catalog/calendar",
		"/v2/catalog/changes",
		"/v2/catalog/redirects",
		"/v2/me/playtimes",
		"/v2/me/cover-votes",
		"/v2/me/claims",
		"/v2/me/claims/" + idstr(fx.Pending),
		"/v2/me/proposals",
		"/v2/moderation/claims",
		"/v2/moderation/claims/" + idstr(fx.Pending),
		"/v2/moderation/proposals",
		"/v2/moderation/snapshots/work/" + idstr(fx.Work),
	}
	for _, path := range gets {
		status, ct, body := liveDo(t, env, http.MethodGet, path, liveAuthPath(path), "")
		if status != 200 {
			t.Errorf("GET %s -> %d %s", path, status, body)
			continue
		}
		if !strings.Contains(ct, "json") {
			t.Errorf("GET %s content-type %q", path, ct)
		}
	}
}

func TestLiveWrites200(t *testing.T) {
	env := liveCatalog(t)
	fx := env.fx

	status, _, body := liveDo(t, env, http.MethodPut, "/v2/me/playtimes/"+idstr(fx.Work), liveUserToken, `{"minutes":12}`)
	require.Equal(t, 200, status, string(body))
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/playtimes/"+idstr(fx.Work), liveUserToken, "")
	require.Equal(t, 200, status, string(body))
	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/playtimes", liveUserToken,
		`{"items":[{"work_id":"`+idstr(fx.Work)+`","minutes":15}]}`)
	require.Equal(t, 207, status, string(body))
	status, _, _ = liveDo(t, env, http.MethodDelete, "/v2/me/playtimes/"+idstr(fx.Work), liveUserToken, "")
	require.Equal(t, 204, status)

	status, _, body = liveDo(t, env, http.MethodPut, "/v2/me/cover-votes/"+idstr(fx.Cover), liveUserToken, `{"vote":"up"}`)
	require.Equal(t, 200, status, string(body))
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/cover-votes", liveUserToken, "")
	require.Equal(t, 200, status, string(body))
	status, _, _ = liveDo(t, env, http.MethodDelete, "/v2/me/cover-votes/"+idstr(fx.Cover), liveUserToken, "")
	require.Equal(t, 204, status)

	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", liveUserToken,
		`{"work_id":"`+idstr(fx.Claimable)+`","site_work_id":"20001"}`)
	require.Equal(t, 201, status, string(body))

	empty := datatypes.JSON([]byte("{}"))
	mintTarget := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "Mint Target",
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	require.NoError(t, env.db.Create(mintTarget).Error)
	require.NoError(t, env.db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: mintTarget.ID, SourceID: 2,
		ExternalID: "v77777", LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error)
	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", liveUserToken,
		`{"refs":[{"source":"vndb","external_id":"v77777"}],"site_work_id":"20002"}`)
	require.Equal(t, 201, status, string(body))

	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", liveUserToken,
		`{"refs":[{"source":"vndb","external_id":"v123456"}],"display_name":"Minted From Refs","site_work_id":"20003"}`)
	require.Equal(t, 201, status, string(body))

	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/claims?limit=1", liveUserToken, "")
	require.Equal(t, 200, status, string(body))
	var page struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor *string           `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(body, &page))
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.NextCursor)
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/claims?limit=1&cursor="+*page.NextCursor, liveUserToken, "")
	require.Equal(t, 200, status, string(body))

	status, _, body = liveDo(t, env, http.MethodGet, "/v2/moderation/claims/"+idstr(fx.Pending), liveUserToken, "")
	require.Equal(t, 200, status, string(body))

	etag := `"c` + idstr(fx.Pending) + `"`
	status, _, body = liveDo(t, env, http.MethodPost, "/v2/moderation/claims/"+idstr(fx.Pending)+"/decisions", liveUserToken,
		`{"decision":"approve","note":"ok"}`)
	// missing If-Match
	require.Equal(t, 428, status, string(body))
	reqBody := `{"decision":"approve","note":"ok"}`
	status, _, body = liveDoHeader(t, env, http.MethodPost, "/v2/moderation/claims/"+idstr(fx.Pending)+"/decisions", liveUserToken, reqBody, map[string]string{"If-Match": etag})
	require.Equal(t, 201, status, string(body))

	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/proposals", liveUserToken, fmt.Sprintf(
		`{"entity_type":%q,"entity_id":%q,"patch":{%q:"Renamed Live Work"}}`,
		editspec.TypeWork, idstr(fx.Work), editspec.FieldWorkDisplayName))
	if status != 201 && status != 403 {
		t.Fatalf("create proposal %d %s", status, body)
	}
}

func TestLiveSpecWalk(t *testing.T) {
	env := liveCatalog(t)
	specApp := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	doc := SetupWith(specApp, Options{Catalog: env.cat}).OpenAPI()
	require.NotEmpty(t, doc.Paths)

	for path, item := range doc.Paths {
		for _, op := range pathOps(item) {
			method := liveOpMethod(item, op)
			url := liveSubstitute(path, env.fx)
			if strings.Contains(url, "{") {
				t.Fatalf("unsubstituted path param in %s", url)
			}
			token := liveAuthPath(path)
			var body string
			switch method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				body = "{}"
			}
			status, ct, raw := liveDo(t, env, method, url, token, body)
			declared := make([]string, 0, len(op.Responses))
			for code := range op.Responses {
				declared = append(declared, code)
			}
			if _, ok := op.Responses[itoa(status)]; !ok {
				t.Errorf("%s %s returned %d not in %v body=%s", method, path, status, declared, raw)
			}
			if status >= 400 && !strings.Contains(ct, "application/problem+json") {
				t.Errorf("%s %s error content-type %q", method, path, ct)
			}
		}
	}
}
