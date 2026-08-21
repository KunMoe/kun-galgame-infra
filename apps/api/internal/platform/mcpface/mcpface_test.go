package mcpface

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var expectedTools = []string{
	"catalog_calendar",
	"catalog_calendar_pending",
	"catalog_calendar_tba",
	"catalog_changes",
	"catalog_character_get",
	"catalog_engine_get",
	"catalog_engines_list",
	"catalog_label_get",
	"catalog_label_relation_graph",
	"catalog_labels_list",
	"catalog_lookup_external",
	"catalog_name_get",
	"catalog_releases",
	"catalog_search",
	"catalog_series_get",
	"catalog_series_list",
	"catalog_stats",
	"catalog_tag_get",
	"catalog_tags_list",
	"catalog_work_characters",
	"catalog_work_covers",
	"catalog_work_credits",
	"catalog_work_engines",
	"catalog_work_get",
	"catalog_work_intros",
	"catalog_work_links",
	"catalog_work_ratings",
	"catalog_work_relations",
	"catalog_work_releases",
	"catalog_work_screenshots",
	"catalog_work_series",
	"catalog_work_tags",
	"catalog_works_list",
	"catalog_works_search",
	"news_get",
	"news_list",
	"news_sources",
}

func TestToolRegistry(t *testing.T) {
	ctx := context.Background()
	server := NewServer(NewUpstream("http://127.0.0.1:0"))

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	got := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
		if tool.InputSchema == nil {
			t.Errorf("tool %q has a nil input schema", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %q has an empty description", tool.Name)
		}
	}
	sort.Strings(got)

	if len(got) != len(expectedTools) {
		t.Fatalf("tool count = %d, want %d (%v)", len(got), len(expectedTools), got)
	}
	for i, name := range expectedTools {
		if got[i] != name {
			t.Errorf("tool[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestBearerTokenFormCheck(t *testing.T) {
	tests := []struct {
		name      string
		header    http.Header
		wantOK    bool
		wantToken string
	}{
		{"missing", http.Header{}, false, ""},
		{"nil header", nil, false, ""},
		{"non-bearer scheme", http.Header{"Authorization": {"Basic nm_live_x"}}, false, ""},
		{"bearer non-nm token (e.g. a JWT)", http.Header{"Authorization": {"Bearer eyJhbGci.foo.bar"}}, false, ""},
		{"bearer live key", http.Header{"Authorization": {"Bearer nm_live_abc123"}}, true, "nm_live_abc123"},
		{"bearer test key with padding", http.Header{"Authorization": {"Bearer   nm_test_xyz  "}}, true, "nm_test_xyz"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := bearerToken(tc.header)
			if ok != tc.wantOK || token != tc.wantToken {
				t.Errorf("bearerToken() = (%q, %v), want (%q, %v)", token, ok, tc.wantToken, tc.wantOK)
			}
		})
	}
}

func TestUpstreamGetPassthrough(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	up := NewUpstream(srv.URL)
	q := newQuery()
	q.Set("q", "スモーク")
	status, body, err := up.Get(context.Background(), "/v1/catalog/search", q, "Bearer nm_live_k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if string(body) != `{"data":{"ok":true}}` {
		t.Errorf("body = %s", body)
	}
	if gotPath != "/v1/catalog/search" {
		t.Errorf("upstream path = %q", gotPath)
	}
	if gotQuery != "q=%E3%82%B9%E3%83%A2%E3%83%BC%E3%82%AF" {
		t.Errorf("upstream query = %q", gotQuery)
	}
	if gotAuth != "Bearer nm_live_k" {
		t.Errorf("forwarded auth = %q", gotAuth)
	}
}

func TestMapUpstream(t *testing.T) {
	if r := mapUpstream(200, []byte(`{"x":1}`)); r.IsError {
		t.Error("200 must not be an error result")
	} else if textOf(t, r) != `{"x":1}` {
		t.Errorf("200 body = %q", textOf(t, r))
	}

	for _, status := range []int{401, 403, 404, 429, 500} {
		r := mapUpstream(status, []byte(`{"error":"nope"}`))
		if !r.IsError {
			t.Errorf("status %d must be an error result", status)
		}
		if txt := textOf(t, r); !contains(txt, "nope") {
			t.Errorf("status %d result should carry the upstream body, got %q", status, txt)
		}
	}
	if txt := textOf(t, mapUpstream(429, nil)); !contains(txt, devPortalURL) {
		t.Errorf("429 hint missing portal pointer: %q", txt)
	}
}

func TestUpstreamGetTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	up := NewUpstream(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := up.Get(ctx, "/v1/catalog/search", newQuery(), "Bearer nm_live_k"); err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}

func TestHandlerEndToEnd(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
	}))
	defer srv.Close()

	tl := &tools{up: NewUpstream(srv.URL)}
	ctx := context.Background()

	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{
		Header: http.Header{"Authorization": {"Bearer nm_test_smoke"}},
	}}
	res, _, err := tl.catalogSearch(ctx, req, catalogSearchInput{Type: "works", Q: "fate", Limit: 10})
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if res.IsError {
		t.Fatalf("handler returned an error result: %q", textOf(t, res))
	}
	if gotPath != "/v1/catalog/search" {
		t.Errorf("path = %q", gotPath)
	}
	if !contains(gotQuery, "q=fate") || !contains(gotQuery, "type=works") || !contains(gotQuery, "limit=10") {
		t.Errorf("query = %q (want type=works, q=fate and limit=10)", gotQuery)
	}

	noKey := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{}}}
	res2, _, err := tl.catalogWorkGet(ctx, noKey, catalogWorkGetInput{ID: 1})
	if err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if !res2.IsError {
		t.Fatal("missing key must yield an error result")
	}
	if !contains(textOf(t, res2), devPortalURL) {
		t.Errorf("auth error should point at the portal: %q", textOf(t, res2))
	}
}

func TestNewsToolsHitTheNewsFace(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
	}))
	defer srv.Close()

	tl := &tools{up: NewUpstream(srv.URL)}
	ctx := context.Background()
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{
		Header: http.Header{"Authorization": {"Bearer nm_test_smoke"}},
	}}

	if _, _, err := tl.newsList(ctx, req, newsListInput{Source: "ymgal", Lane: "column", Limit: 5}); err != nil {
		t.Fatalf("news_list: %v", err)
	}
	if gotPath != "/v1/news" {
		t.Errorf("news_list path = %q", gotPath)
	}
	if !contains(gotQuery, "source=ymgal") || !contains(gotQuery, "lane=column") || !contains(gotQuery, "limit=5") {
		t.Errorf("news_list query = %q", gotQuery)
	}

	if _, _, err := tl.newsSources(ctx, req, newsSourcesInput{}); err != nil {
		t.Fatalf("news_sources: %v", err)
	}
	if gotPath != "/v1/news/sources" || gotQuery != "" {
		t.Errorf("news_sources = %q?%q", gotPath, gotQuery)
	}

	if _, _, err := tl.newsGet(ctx, req, newsGetInput{ID: 42}); err != nil {
		t.Fatalf("news_get: %v", err)
	}
	if gotPath != "/v1/news/42" {
		t.Errorf("news_get path = %q", gotPath)
	}
}

func TestWorkSubresourceToolsHitTheirBlock(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
	}))
	defer srv.Close()

	tl := &tools{up: NewUpstream(srv.URL)}
	ctx := context.Background()
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{
		Header: http.Header{"Authorization": {"Bearer nm_test_smoke"}},
	}}
	in := workSubresourceInput{ID: 7, Limit: 5, Offset: 10, Nsfw: true}

	blocks := map[string]func() error{
		"covers":      func() error { _, _, err := tl.catalogWorkCovers(ctx, req, in); return err },
		"screenshots": func() error { _, _, err := tl.catalogWorkScreenshots(ctx, req, in); return err },
		"characters":  func() error { _, _, err := tl.catalogWorkCharacters(ctx, req, in); return err },
		"credits":     func() error { _, _, err := tl.catalogWorkCredits(ctx, req, in); return err },
		"releases":    func() error { _, _, err := tl.catalogWorkReleases(ctx, req, in); return err },
		"intros":      func() error { _, _, err := tl.catalogWorkIntros(ctx, req, in); return err },
		"ratings":     func() error { _, _, err := tl.catalogWorkRatings(ctx, req, in); return err },
		"relations":   func() error { _, _, err := tl.catalogWorkRelations(ctx, req, in); return err },
		"series":      func() error { _, _, err := tl.catalogWorkSeries(ctx, req, in); return err },
		"links":       func() error { _, _, err := tl.catalogWorkLinks(ctx, req, in); return err },
		"engines":     func() error { _, _, err := tl.catalogWorkEngines(ctx, req, in); return err },
	}
	for block, call := range blocks {
		if err := call(); err != nil {
			t.Fatalf("%s: %v", block, err)
		}
		if want := "/v1/catalog/works/7/" + block; gotPath != want {
			t.Errorf("%s path = %q, want %q", block, gotPath, want)
		}
		if !contains(gotQuery, "limit=5") || !contains(gotQuery, "offset=10") || !contains(gotQuery, "nsfw=1") {
			t.Errorf("%s query = %q", block, gotQuery)
		}
		if contains(gotQuery, "spoilers") {
			t.Errorf("%s must not send spoilers, query = %q", block, gotQuery)
		}
	}

	if _, _, err := tl.catalogWorkTags(ctx, req, workTagsInput{ID: 7, Limit: 5, Spoilers: 2}); err != nil {
		t.Fatalf("tags: %v", err)
	}
	if gotPath != "/v1/catalog/works/7/tags" {
		t.Errorf("tags path = %q", gotPath)
	}
	if !contains(gotQuery, "spoilers=2") {
		t.Errorf("tags query = %q, want spoilers=2", gotQuery)
	}
}

func TestNewsToolDescriptionsStateTheGrant(t *testing.T) {
	for name, desc := range map[string]string{
		"news_list":    descNewsList,
		"news_sources": descNewsSources,
		"news_get":     descNewsGet,
	} {
		if !contains(desc, "news:read") {
			t.Errorf("%s description must name the news:read scope", name)
		}
		if !contains(desc, "GRANTED BY THE PLATFORM") {
			t.Errorf("%s description must say the scope is not self-service", name)
		}
	}
}

func textOf(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", r.Content[0])
	}
	return tc.Text
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
