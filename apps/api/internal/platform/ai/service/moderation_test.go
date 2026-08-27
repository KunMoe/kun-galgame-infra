package service

import (
	"context"
	"testing"
	"time"

	"api/internal/platform/ai/model"
	"api/internal/platform/ai/upstream"
)

func TestModerateNormal(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "deepseek-chat",
		result: upstream.ChatResult{
			Content:          `{"flagged": true, "categories": ["abuse"], "score": 0.91}`,
			Channel:          "deepseek-chat",
			PromptTokens:     42,
			CompletionTokens: 8,
		},
	}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "you idiot"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged || res.Degraded {
		t.Fatalf("want flagged+not-degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Route != model.RouteModerateText || res.Channel != "deepseek-chat" {
		t.Fatalf("route/channel = %q/%q", res.Route, res.Channel)
	}
	if res.Score == nil || *res.Score < 0.9 || len(res.Categories) != 1 || res.Categories[0] != "abuse" {
		t.Fatalf("verdict fields not parsed: score=%v categories=%v", res.Score, res.Categories)
	}
	if up.calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", up.calls)
	}

	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 {
		t.Fatalf("want 1 metered row, got %d", len(rows))
	}
	r := rows[0]
	if r.Status != model.StatusOK || r.Channel != "deepseek-chat" || r.PromptTokens != 42 || r.CompletionTokens != 8 {
		t.Fatalf("metered row wrong: status=%d channel=%q prompt=%d completion=%d", r.Status, r.Channel, r.PromptTokens, r.CompletionTokens)
	}
	if r.Site != "letmoe" {
		t.Fatalf("metered site = %q, want letmoe (derived, not on wire)", r.Site)
	}
}

func TestModerateUpstreamErrorFailOpen(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{configured: true, model: "deepseek-chat", err: context.DeadlineExceeded}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "hello"})
	if err != nil {
		t.Fatalf("Moderate must never error (fail-open): %v", err)
	}
	if res.Flagged || !res.Degraded {
		t.Fatalf("want fail-open (flagged=false degraded=true), got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if up.calls != 1 {
		t.Fatalf("upstream calls = %d, want 1 (it must have dialled)", up.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 || rows[0].Status != model.StatusUpstreamError {
		t.Fatalf("want 1 row status=upstream_error(1), got %+v", rows)
	}
}

func TestModerateDegradedEnvEmpty(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{configured: false}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "anything"})
	if err != nil {
		t.Fatalf("Moderate must never error: %v", err)
	}
	if res.Flagged || !res.Degraded {
		t.Fatalf("want fail-open degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != "" {
		t.Fatalf("degraded channel must be empty, got %q", res.Channel)
	}
	if up.calls != 0 {
		t.Fatalf("upstream calls = %d, want 0 (degraded must NOT dial)", up.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 || rows[0].Status != model.StatusDegraded {
		t.Fatalf("want 1 row status=degraded(4), got %+v", rows)
	}
}

func TestBudgetOverCapFailOpen(t *testing.T) {
	cleanTables(t)
	insertBudget(t, model.RouteModerateText, "letmoe", ptr[int64](100))
	insertUsageCost(t, "letmoe", model.RouteModerateText, 60)
	insertUsageCost(t, "letmoe", model.RouteModerateText, 50)

	up := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": true}`, Channel: "deepseek-chat"}}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "spend"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Flagged || !res.Degraded {
		t.Fatalf("over-budget must fail-open (flagged=false degraded=true), got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if up.calls != 0 {
		t.Fatalf("over-budget must NOT dial upstream, calls=%d", up.calls)
	}
	var denied int64
	if err := testDB.Model(&model.AIUsage{}).
		Where("site = ? AND route = ? AND status = ?", "letmoe", model.RouteModerateText, model.StatusBudgetDenied).
		Count(&denied).Error; err != nil {
		t.Fatalf("count denied: %v", err)
	}
	if denied != 1 {
		t.Fatalf("want 1 budget_denied row, got %d", denied)
	}
}

func TestBudgetUnderCapAllows(t *testing.T) {
	cleanTables(t)
	insertBudget(t, model.RouteModerateText, "letmoe", ptr[int64](1000))
	insertUsageCost(t, "letmoe", model.RouteModerateText, 10)

	up := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": false}`, Channel: "deepseek-chat"}}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "fine"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Degraded {
		t.Fatalf("under-cap must proceed normally, got degraded=true")
	}
	if up.calls != 1 {
		t.Fatalf("under-cap must dial upstream, calls=%d", up.calls)
	}
}

func TestBudgetNullCapNoBlock(t *testing.T) {
	cleanTables(t)
	insertBudget(t, model.RouteModerateText, "letmoe", nil)
	insertUsageCost(t, "letmoe", model.RouteModerateText, 999999)

	up := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": false}`, Channel: "deepseek-chat"}}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "fine"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Degraded || up.calls != 1 {
		t.Fatalf("NULL cap must not block: degraded=%v calls=%d", res.Degraded, up.calls)
	}
}

func TestBudgetRouteDefaultOverride(t *testing.T) {
	cleanTables(t)
	insertBudget(t, model.RouteModerateText, "", ptr[int64](100))
	insertUsageCost(t, "letmoe", model.RouteModerateText, 150)

	up := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": false}`, Channel: "deepseek-chat"}}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "spend"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Degraded || up.calls != 0 {
		t.Fatalf("route-default cap must apply to letmoe: degraded=%v calls=%d", res.Degraded, up.calls)
	}
}

func TestModerateTruncatedReplyIsNotUpstreamError(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "glm-5.2",
		result: upstream.ChatResult{
			Content:          `{"flagged": true, "categories": ["abu`,
			Channel:          "glm-5.2",
			FinishReason:     "length",
			PromptTokens:     42,
			CompletionTokens: 1024,
		},
	}
	s := newLLMOnly(up)

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "you idiot"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Flagged || !res.Degraded {
		t.Fatalf("want fail-open degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}

	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 {
		t.Fatalf("want 1 metered row, got %d", len(rows))
	}
	if rows[0].Status != model.StatusTruncated {
		t.Fatalf("status = %d, want truncated(%d) — truncation is hiding inside upstream_error again",
			rows[0].Status, model.StatusTruncated)
	}
	if rows[0].CompletionTokens != 1024 {
		t.Fatalf("completion_tokens = %d, want 1024 — the evidence that names the ceiling must survive",
			rows[0].CompletionTokens)
	}
}

func TestModerateUnparseableWithoutTruncation(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "glm-5.2",
		result: upstream.ChatResult{
			Content:      `I'm sorry, I can't help with that.`,
			Channel:      "glm-5.2",
			FinishReason: "stop",
		},
	}
	s := newLLMOnly(up)

	if _, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "hi"}); err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 || rows[0].Status != model.StatusUpstreamError {
		t.Fatalf("want a single upstream_error row, got %d rows status=%v", len(rows), rows)
	}
}

// The prod 429 body verbatim, so this fails if IsRateLimited ever stops
// matching what Cloudflare actually sends.
var cfRateLimit = &upstream.StatusError{
	Code: 429,
	Body: `{"errors":[{"message":"AiError: AiError: rate limiting: inference request per min rate reached (8700d6e7)","code":3021}],"success":false,"result":{}}`,
}

func TestModerateRetriesOnceOnRateLimit(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "deepseek-chat",
		errSeq:     []error{cfRateLimit},
		result: upstream.ChatResult{
			Content: `{"flagged": true, "categories": ["abuse"], "score": 0.95}`,
			Channel: "deepseek-chat",
		},
	}
	s := newLLMOnly(up)
	s.retryDelay = time.Millisecond

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "you idiot"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged || res.Degraded {
		t.Fatalf("a retried 429 must produce a real verdict, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if up.calls != 2 {
		t.Fatalf("upstream calls = %d, want 2 (one 429 + one retry)", up.calls)
	}

	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 2 {
		t.Fatalf("want 2 metered rows (rate-limited + ok), got %d", len(rows))
	}
	if rows[0].Status != model.StatusRateLimited {
		t.Fatalf("first row status = %d, want StatusRateLimited — the retry must stay visible", rows[0].Status)
	}
	if rows[1].Status != model.StatusOK {
		t.Fatalf("second row status = %d, want StatusOK", rows[1].Status)
	}
}

func TestModerateFailsOpenAfterSecondRateLimit(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "deepseek-chat",
		errSeq:     []error{cfRateLimit, cfRateLimit},
	}
	s := newLLMOnly(up)
	s.retryDelay = time.Millisecond

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "you idiot"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("two 429s must still fail open as degraded")
	}
	if up.calls != 2 {
		t.Fatalf("upstream calls = %d, want exactly 2 — one retry, not a loop", up.calls)
	}
}

func TestModerateDoesNotRetryNonRateLimitError(t *testing.T) {
	cleanTables(t)
	up := &fakeUpstream{
		configured: true,
		model:      "deepseek-chat",
		err:        &upstream.StatusError{Code: 400, Body: `{"errors":[{"message":"bad request"}]}`},
	}
	s := newLLMOnly(up)
	s.retryDelay = time.Millisecond

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "hi"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Degraded {
		t.Fatalf("a 400 still fails open")
	}
	if up.calls != 1 {
		t.Fatalf("upstream calls = %d, want 1 — only a 429 is worth retrying", up.calls)
	}
}
