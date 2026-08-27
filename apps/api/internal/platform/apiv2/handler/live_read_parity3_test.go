package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type liveWorkTagRow struct {
	DisplayName string `json:"display_name"`
	Spoiler     string `json:"spoiler"`
}

func liveWorkTagSpoilers(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	var rows []liveWorkTagRow
	require.NoError(t, json.Unmarshal(raw, &rows))
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.DisplayName] = r.Spoiler
	}
	return out
}

func TestLiveWorkDetailSpoilerCeiling(t *testing.T) {
	env := liveCatalog(t)
	base := "/v2/catalog/works/" + idstr(env.fx.Work) + "?include=tags"

	read := func(path string) map[string]string {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var detail struct {
			Tags json.RawMessage `json:"tags"`
		}
		require.NoError(t, json.Unmarshal(body, &detail))
		return liveWorkTagSpoilers(t, detail.Tags)
	}

	byName := read(base)
	require.Equal(t, map[string]string{"live-tag": "none"}, byName,
		"the default ceiling hides every spoiler>0 tag row")

	byName = read(base + "&spoiler=none")
	require.Equal(t, map[string]string{"live-tag": "none"}, byName)

	byName = read(base + "&spoiler=minor")
	require.Equal(t, map[string]string{
		"live-tag": "none", "live-tag-minor": "minor",
	}, byName, "minor admits <=1 and nothing above it")

	byName = read(base + "&spoiler=major")
	require.Equal(t, map[string]string{
		"live-tag": "none", "live-tag-minor": "minor", "live-tag-major": "major",
	}, byName)
}

func TestLiveWorkTagsSubFaceSpoilerCeiling(t *testing.T) {
	env := liveCatalog(t)
	base := "/v2/catalog/works/" + idstr(env.fx.Work) + "/tags"

	read := func(path string) map[string]string {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var page struct {
			Items json.RawMessage `json:"items"`
		}
		require.NoError(t, json.Unmarshal(body, &page))
		return liveWorkTagSpoilers(t, page.Items)
	}

	require.Equal(t, map[string]string{"live-tag": "none"}, read(base))
	require.Equal(t, map[string]string{
		"live-tag": "none", "live-tag-minor": "minor",
	}, read(base+"?spoiler=minor"))
	require.Equal(t, map[string]string{
		"live-tag": "none", "live-tag-minor": "minor", "live-tag-major": "major",
	}, read(base+"?spoiler=major"))
}

func TestLiveCharacterTraitsSpoilerCeiling(t *testing.T) {
	env := liveCatalog(t)
	base := "/v2/catalog/characters/" + idstr(env.fx.Character) + "?include=traits"

	read := func(path string) map[string]string {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var out struct {
			Traits *[]struct {
				ID      string `json:"id"`
				Spoiler string `json:"spoiler"`
			} `json:"traits"`
		}
		require.NoError(t, json.Unmarshal(body, &out))
		require.NotNil(t, out.Traits)
		got := map[string]string{}
		for _, tr := range *out.Traits {
			got[tr.ID] = tr.Spoiler
		}
		return got
	}

	require.Equal(t, map[string]string{idstr(env.fx.Trait): "none"}, read(base),
		"the default ceiling hides the level-1 trait")
	require.Equal(t, map[string]string{
		idstr(env.fx.Trait): "none", idstr(env.fx.TraitMinor): "minor",
	}, read(base+"&spoiler=minor"))
}

func TestLiveSpoilerUnknownValueIsAProblem(t *testing.T) {
	env := liveCatalog(t)
	for _, path := range []string{
		"/v2/catalog/works/" + idstr(env.fx.Work) + "?spoiler=bogus",
		"/v2/catalog/works/" + idstr(env.fx.Work) + "/tags?spoiler=bogus",
		"/v2/catalog/characters/" + idstr(env.fx.Character) + "?spoiler=bogus",
	} {
		status, ct, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 400, status, path)
		require.Contains(t, ct, "application/problem+json", path)
		var p struct {
			Code   string `json:"code"`
			Errors []struct {
				Parameter string `json:"parameter"`
				Reason    string `json:"reason"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(body, &p))
		require.Equal(t, "UNKNOWN_ENUM_VALUE", p.Code, string(body))
		require.Len(t, p.Errors, 1)
		require.Equal(t, "spoiler", p.Errors[0].Parameter)
	}
}

func TestLiveCharacterIncludeAliasesIntrosRefs(t *testing.T) {
	env := liveCatalog(t)
	status, _, body := liveDo(t, env, http.MethodGet,
		"/v2/catalog/characters/"+idstr(env.fx.Character)+"?include=aliases,intros,refs", liveAppKey, "")
	require.Equal(t, 200, status, string(body))
	var out struct {
		Aliases *[]struct {
			Value     string `json:"value"`
			Lang      string `json:"lang"`
			AliasKind string `json:"alias_kind"`
		} `json:"aliases"`
		Intros *[]struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"intros"`
		Refs *[]struct {
			Source     string `json:"source"`
			ExternalID string `json:"external_id"`
		} `json:"refs"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.NotNil(t, out.Aliases)
	require.Len(t, *out.Aliases, 1)
	require.Equal(t, "Live Char Alias", (*out.Aliases)[0].Value)
	require.Equal(t, "translation", (*out.Aliases)[0].AliasKind)
	require.NotNil(t, out.Intros)
	require.Len(t, *out.Intros, 1)
	require.Equal(t, "角色简介", (*out.Intros)[0].Value)
	require.NotNil(t, out.Refs)
	require.Len(t, *out.Refs, 1)
	require.Equal(t, "c-live-1", (*out.Refs)[0].ExternalID)

	status, _, body = liveDo(t, env, http.MethodGet,
		"/v2/catalog/characters/"+idstr(env.fx.Character), liveAppKey, "")
	require.Equal(t, 200, status)
	var bare map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &bare))
	for _, k := range []string{"aliases", "intros", "refs"} {
		_, ok := bare[k]
		require.False(t, ok, "unrequested block %s must be absent", k)
	}
}

func TestLiveCompanyGraphIncludeLogo(t *testing.T) {
	env := liveCatalog(t)
	base := "/v2/catalog/companies/" + idstr(env.fx.Company) + "/graph"

	read := func(path string) map[string]*struct {
		URL  string `json:"url"`
		Hash string `json:"hash"`
	} {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var out struct {
			Nodes []struct {
				ID   string `json:"id"`
				Logo *struct {
					URL  string `json:"url"`
					Hash string `json:"hash"`
				} `json:"logo"`
			} `json:"nodes"`
			Edges []struct {
				FromID   string `json:"from_id"`
				ToID     string `json:"to_id"`
				Relation string `json:"relation"`
			} `json:"edges"`
		}
		require.NoError(t, json.Unmarshal(body, &out))
		require.Len(t, out.Nodes, 2, string(body))
		require.Len(t, out.Edges, 1)
		require.Equal(t, "parent", out.Edges[0].Relation)
		got := map[string]*struct {
			URL  string `json:"url"`
			Hash string `json:"hash"`
		}{}
		for _, n := range out.Nodes {
			got[n.ID] = n.Logo
		}
		return got
	}

	bare := read(base)
	require.Nil(t, bare[idstr(env.fx.CompanyLogo)], "no logo without include=logo")
	require.Nil(t, bare[idstr(env.fx.Company)])

	withLogo := read(base + "?include=logo")
	node := withLogo[idstr(env.fx.CompanyLogo)]
	require.NotNil(t, node)
	require.Equal(t, liveLogoHash, node.Hash)
	require.Contains(t, node.URL, liveCDNBase)
	require.Nil(t, withLogo[idstr(env.fx.Company)], "a logo-less node stays without the key")

	// The graph deliberately keeps the company token set: narrowing it would 400
	// requests that answer 200 today.
	status, _, _ := liveDo(t, env, http.MethodGet, base+"?include=aliases,intros,links", liveAppKey, "")
	require.Equal(t, 200, status)
	status, _, _ = liveDo(t, env, http.MethodGet, base+"?include=nope", liveAppKey, "")
	require.Equal(t, 400, status)
}

func TestLiveEntityLangOnEveryLane(t *testing.T) {
	env := liveCatalog(t)

	langOf := func(path string) (*string, bool) {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var row map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(body, &row))
		raw, present := row["lang"]
		if !present {
			return nil, false
		}
		var v *string
		require.NoError(t, json.Unmarshal(raw, &v))
		return v, true
	}

	langsInList := func(path string) map[string]*string {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
		require.Equal(t, 200, status, string(body))
		var page struct {
			Items []struct {
				ID   string  `json:"id"`
				Lang *string `json:"lang"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(body, &page))
		out := map[string]*string{}
		for _, it := range page.Items {
			out[it.ID] = it.Lang
		}
		return out
	}

	for _, tc := range []struct {
		detail string
		want   string
	}{
		{"/v2/catalog/companies/" + idstr(env.fx.Company), "ja"},
		{"/v2/catalog/credit-names/" + idstr(env.fx.Credit), "ja"},
		{"/v2/catalog/characters/" + idstr(env.fx.Character), "ja"},
	} {
		got, present := langOf(tc.detail)
		require.True(t, present, "lang is always-present on %s", tc.detail)
		require.NotNil(t, got, tc.detail)
		require.Equal(t, tc.want, *got, tc.detail)
	}

	got, present := langOf("/v2/catalog/companies/" + idstr(env.fx.CompanyEmpty))
	require.True(t, present)
	require.Nil(t, got, "an unrecorded lang is null, not absent")

	got, present = langOf("/v2/catalog/characters/" + idstr(env.fx.CharacterNoLang))
	require.True(t, present)
	require.Nil(t, got)

	companies := langsInList("/v2/catalog/companies")
	require.Equal(t, "ja", *companies[idstr(env.fx.Company)])
	require.Nil(t, companies[idstr(env.fx.CompanyEmpty)])

	characters := langsInList("/v2/catalog/characters")
	require.Equal(t, "ja", *characters[idstr(env.fx.Character)])
	require.Nil(t, characters[idstr(env.fx.CharacterNoLang)])

	names := langsInList("/v2/catalog/credit-names")
	require.Equal(t, "ja", *names[idstr(env.fx.Credit)])
	require.Nil(t, names[idstr(env.fx.Sibling)], "the sibling row has no recorded lang")

	batch := langsInList("/v2/catalog/companies?ids=" + idstr(env.fx.Company) + "&include=links")
	require.Equal(t, "ja", *batch[idstr(env.fx.Company)], "the batch lane upgrades to detail reads")

	// Embedded lanes: the same shape reached through another entity.
	status, _, body := liveDo(t, env, http.MethodGet,
		"/v2/catalog/credit-names/"+idstr(env.fx.Credit)+"?include=siblings", liveAppKey, "")
	require.Equal(t, 200, status, string(body))
	var withSiblings struct {
		Siblings *[]struct {
			ID   string  `json:"id"`
			Lang *string `json:"lang"`
		} `json:"siblings"`
	}
	require.NoError(t, json.Unmarshal(body, &withSiblings))
	require.NotNil(t, withSiblings.Siblings)
	require.Len(t, *withSiblings.Siblings, 1)
	require.Nil(t, (*withSiblings.Siblings)[0].Lang)

	status, _, body = liveDo(t, env, http.MethodGet,
		"/v2/catalog/persons/"+idstr(env.fx.Person)+"/credit-names", liveAppKey, "")
	require.Equal(t, 200, status, string(body))
	var personNames struct {
		Items []struct {
			ID   string  `json:"id"`
			Lang *string `json:"lang"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(body, &personNames))
	byID := map[string]*string{}
	for _, it := range personNames.Items {
		byID[it.ID] = it.Lang
	}
	require.Equal(t, "ja", *byID[idstr(env.fx.Credit)])
	require.Nil(t, byID[idstr(env.fx.Sibling)])
}
