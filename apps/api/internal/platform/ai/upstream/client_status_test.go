package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatJSONRateLimitIsTyped(t *testing.T) {
	const body = `{"errors":[{"message":"AiError: AiError: rate limiting: inference request per min rate reached (8700d6e7)","code":3021}],"success":false,"result":{},"messages":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "m")
	_, err := c.ChatJSON(context.Background(), "sys", "user", 64)
	if err == nil {
		t.Fatal("want an error")
	}
	if !IsRateLimited(err) {
		t.Fatalf("Cloudflare sends its per-minute limit as a real 429; IsRateLimited said no for: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "upstream http 429: ") {
		t.Fatalf("error text changed, existing log greps break: %q", err.Error())
	}
}

func TestChatJSONOtherStatusIsNotRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"message":"unauthorized"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "m")
	_, err := c.ChatJSON(context.Background(), "sys", "user", 64)
	if err == nil {
		t.Fatal("want an error")
	}
	// Cloudflare has been seen answering 401 when merely overloaded. Retrying
	// that would mask a genuine credential failure, so it must not be retried.
	if IsRateLimited(err) {
		t.Fatalf("401 must not count as rate limited: %v", err)
	}
}
