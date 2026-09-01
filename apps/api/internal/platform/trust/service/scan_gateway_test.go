package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayModerateSendsSubjectKind(t *testing.T) {
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastBody = b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"route":"moderate-text","flagged":true,"categories":["spam"],"score":0.9,"channel":"x","degraded":false}}`))
	}))
	defer srv.Close()

	c := NewAIGatewayClient(srv.URL, "id", "secret")

	v, err := c.Moderate(context.Background(), "hello", "forum_topic", nil)
	if err != nil {
		t.Fatalf("Moderate(forum_topic): %v", err)
	}
	if !strings.Contains(string(lastBody), `"subject_kind":"forum_topic"`) {
		t.Fatalf("body missing subject_kind forum_topic: %s", lastBody)
	}
	if !v.Flagged || v.Degraded {
		t.Fatalf("want flagged not-degraded, got flagged=%v degraded=%v", v.Flagged, v.Degraded)
	}
	if v.Channel != "x" {
		t.Fatalf("channel = %q, want x", v.Channel)
	}
	if len(v.Categories) != 1 || v.Categories[0] != "spam" {
		t.Fatalf("categories = %v, want [spam]", v.Categories)
	}
	if v.Score == nil || *v.Score < 0.89 || *v.Score > 0.91 {
		t.Fatalf("score = %v, want ~0.9", v.Score)
	}

	v, err = c.Moderate(context.Background(), "hello", "", nil)
	if err != nil {
		t.Fatalf("Moderate(empty kind): %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(lastBody, &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, lastBody)
	}
	if _, ok := body["subject_kind"]; ok {
		t.Fatalf("empty subjectKind must omit the key, body=%s", lastBody)
	}
	if !v.Flagged || v.Degraded || v.Channel != "x" {
		t.Fatalf("empty-kind verdict wrong: flagged=%v degraded=%v channel=%q", v.Flagged, v.Degraded, v.Channel)
	}
}
