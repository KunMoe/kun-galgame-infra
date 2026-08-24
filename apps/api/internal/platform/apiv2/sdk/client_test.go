package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGets(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","items":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "nmk_live_testkeytestkeytestkeytestkey")
	status, body, err := c.Problems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("status %d body %s", status, body)
	}
	if gotPath != "/v2/problems" {
		t.Errorf("path %q", gotPath)
	}
	if gotAuth == "" {
		t.Error("missing Authorization")
	}
	status, _, err = c.Vocabulary(context.Background(), "medium")
	if err != nil || status != 200 {
		t.Fatalf("vocab %d %v", status, err)
	}
	if gotPath != "/v2/vocabularies/medium" {
		t.Errorf("vocab path %q", gotPath)
	}
	status, _, err = c.Work(context.Background(), "1")
	if err != nil || status != 200 {
		t.Fatalf("work %d %v", status, err)
	}
	if gotPath != "/v2/catalog/works/1" {
		t.Errorf("work path %q", gotPath)
	}
}
