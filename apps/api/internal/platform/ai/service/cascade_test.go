package service

import (
	"context"
	"testing"

	"api/internal/platform/ai/model"
	"api/internal/platform/ai/upstream"
	"api/internal/platform/settings"
	"api/internal/platform/settings/keys"
)

const omniModel = "omni-moderation-latest"

func TestCascadeOmniCleanBelowThreshold(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	settings.Override(t, keys.AINegativeSampleRate, 0)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        false,
		Categories:     map[string]bool{},
		CategoryScores: map[string]float64{"violence": 0.05, "harassment": 0.10},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": true}`, Channel: "deepseek-chat"}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "hi"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Flagged || res.Degraded {
		t.Fatalf("want clean not-degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != omniModel {
		t.Fatalf("channel = %q, want omni (%q)", res.Channel, omniModel)
	}
	if res.Score == nil || *res.Score < 0.09 || *res.Score > 0.11 {
		t.Fatalf("relevantScore should be 0.10 (harassment), got %v", res.Score)
	}
	if omni.calls != 1 {
		t.Fatalf("omni calls = %d, want 1", omni.calls)
	}
	if llm.calls != 0 {
		t.Fatalf("LLM must NOT dial below threshold without sampling, calls=%d", llm.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 metered row (omni), got %d", len(rows))
	}
	if rows[0].Status != model.StatusOK || rows[0].Channel != omniModel || rows[0].PromptTokens != 0 {
		t.Fatalf("omni row wrong: %+v", rows[0])
	}
}

func TestCascadeEscalatesToLLM(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        true,
		Categories:     map[string]bool{"harassment": true},
		CategoryScores: map[string]float64{"harassment": 0.92},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat", result: upstream.ChatResult{
		Content: `{"flagged": true, "categories": ["abuse"], "score": 0.88}`,
		Channel: "deepseek-chat", PromptTokens: 30, CompletionTokens: 6,
	}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "you idiot"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged || res.Degraded {
		t.Fatalf("want flagged not-degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != "deepseek-chat" {
		t.Fatalf("final channel = %q, want deepseek-chat (LLM is final)", res.Channel)
	}
	if len(res.Categories) != 1 || res.Categories[0] != "abuse" {
		t.Fatalf("final categories = %v, want [abuse] (from LLM)", res.Categories)
	}
	if omni.calls != 1 || llm.calls != 1 {
		t.Fatalf("want omni=1 llm=1, got omni=%d llm=%d", omni.calls, llm.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 2 {
		t.Fatalf("want 2 metered rows (omni + LLM), got %d", len(rows))
	}
	if rows[0].Channel != omniModel || rows[0].Status != model.StatusOK || rows[0].PromptTokens != 0 {
		t.Fatalf("omni row wrong: %+v", rows[0])
	}
	if rows[1].Channel != "deepseek-chat" || rows[1].Status != model.StatusOK || rows[1].PromptTokens != 30 {
		t.Fatalf("LLM row wrong: %+v", rows[1])
	}
}

func TestCascadeOmniConvictsAloneWhenNoLLM(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        true,
		Categories:     map[string]bool{"violence": true, "sexual": true},
		CategoryScores: map[string]float64{"violence": 0.77, "sexual": 0.99},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: false}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "graphic"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged {
		t.Fatalf("omni high score must convict, flagged=%v", res.Flagged)
	}
	if res.Degraded {
		t.Fatalf("omni conviction is a REAL verdict — must NOT be degraded")
	}
	if res.Channel != omniModel {
		t.Fatalf("channel = %q, want omni (%q)", res.Channel, omniModel)
	}
	if len(res.Categories) != 1 || res.Categories[0] != "violence" {
		t.Fatalf("categories = %v, want [violence] (sexual ignored)", res.Categories)
	}
	if res.Score == nil || *res.Score < 0.76 || *res.Score > 0.78 {
		t.Fatalf("relevantScore should be 0.77 (violence), got %v", res.Score)
	}
	if llm.calls != 0 {
		t.Fatalf("LLM unconfigured must not dial, calls=%d", llm.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 || rows[0].Channel != omniModel {
		t.Fatalf("want 1 omni row, got %+v", rows)
	}
}

func TestCascadeOmniErrorFallsThroughToLLM(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, err: context.DeadlineExceeded}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat", result: upstream.ChatResult{
		Content: `{"flagged": false, "score": 0.2}`, Channel: "deepseek-chat",
	}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "hello"})
	if err != nil {
		t.Fatalf("Moderate must never error (fail-open): %v", err)
	}
	if res.Degraded {
		t.Fatalf("LLM available → not degraded, got degraded=true")
	}
	if res.Channel != "deepseek-chat" {
		t.Fatalf("channel = %q, want deepseek-chat (LLM served)", res.Channel)
	}
	if omni.calls != 1 || llm.calls != 1 {
		t.Fatalf("want omni=1 llm=1, got omni=%d llm=%d", omni.calls, llm.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (omni error + LLM), got %d", len(rows))
	}
	if rows[0].Channel != omniModel || rows[0].Status != model.StatusUpstreamError {
		t.Fatalf("omni error row wrong: %+v (want channel=%q status=upstream_error)", rows[0], omniModel)
	}
	if rows[1].Channel != "deepseek-chat" || rows[1].Status != model.StatusOK {
		t.Fatalf("LLM row wrong: %+v", rows[1])
	}
}

func TestCascadeIgnoresSexualCategory(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	settings.Override(t, keys.AINegativeSampleRate, 0)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        true,
		Categories:     map[string]bool{"sexual": true},
		CategoryScores: map[string]float64{"sexual": 0.98, "violence": 0.01},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": true}`, Channel: "deepseek-chat"}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "adult"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Flagged {
		t.Fatalf("bare `sexual` must be IGNORED → not flagged")
	}
	if res.Degraded {
		t.Fatalf("clean verdict must not be degraded")
	}
	if res.Score == nil || *res.Score > 0.02 {
		t.Fatalf("relevantScore must exclude `sexual` (0.98), got %v", res.Score)
	}
	if llm.calls != 0 {
		t.Fatalf("ignored category must not escalate, LLM calls=%d", llm.calls)
	}
	if res.Channel != omniModel {
		t.Fatalf("channel = %q, want omni", res.Channel)
	}
}

func TestCascadeAdoptsSexualMinors(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        true,
		Categories:     map[string]bool{"sexual/minors": true, "sexual": true},
		CategoryScores: map[string]float64{"sexual/minors": 0.95, "sexual": 0.99},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: false}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "illegal"})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged || res.Degraded {
		t.Fatalf("sexual/minors is the legal line — must convict non-degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if len(res.Categories) != 1 || res.Categories[0] != "sexual/minors" {
		t.Fatalf("categories = %v, want [sexual/minors] (bare sexual ignored)", res.Categories)
	}
	if res.Score == nil || *res.Score < 0.94 {
		t.Fatalf("relevantScore should be sexual/minors 0.95, got %v", res.Score)
	}
	if llm.calls != 0 {
		t.Fatalf("LLM unconfigured must not dial, calls=%d", llm.calls)
	}
}

func TestCascadeNegativeSamplingSeam(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	settings.Override(t, keys.AINegativeSampleRate, 0.05)
	build := func(randVal float64) (*fakeOmni, *fakeUpstream, *ModerationService) {
		omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
			Flagged:        false,
			CategoryScores: map[string]float64{"harassment": 0.10},
			Channel:        omniModel,
		}}
		llm := &fakeUpstream{configured: true, model: "deepseek-chat",
			result: upstream.ChatResult{Content: `{"flagged": true, "score": 0.7}`, Channel: "deepseek-chat"}}
		s := NewModerationService(testDB, omni, llm, ModerationOptions{
			Rand: func() float64 { return randVal },
		})
		return omni, llm, s
	}

	cleanTables(t)
	_, llmHit, sHit := build(0.01)
	resHit, err := sHit.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "x"})
	if err != nil {
		t.Fatalf("Moderate (hit): %v", err)
	}
	if llmHit.calls != 1 {
		t.Fatalf("sampling HIT must dial LLM, calls=%d", llmHit.calls)
	}
	if !resHit.Flagged || resHit.Channel != "deepseek-chat" {
		t.Fatalf("sampled verdict must be the LLM's (flagged, channel deepseek-chat), got flagged=%v channel=%q", resHit.Flagged, resHit.Channel)
	}
	if rows := usageRows(t, "letmoe", model.RouteModerateText); len(rows) != 2 {
		t.Fatalf("HIT want 2 rows (omni + LLM), got %d", len(rows))
	}

	cleanTables(t)
	_, llmMiss, sMiss := build(0.9)
	resMiss, err := sMiss.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "x"})
	if err != nil {
		t.Fatalf("Moderate (miss): %v", err)
	}
	if llmMiss.calls != 0 {
		t.Fatalf("sampling MISS must NOT dial LLM, calls=%d", llmMiss.calls)
	}
	if resMiss.Flagged || resMiss.Channel != omniModel {
		t.Fatalf("MISS → clean omni verdict, got flagged=%v channel=%q", resMiss.Flagged, resMiss.Channel)
	}
	if rows := usageRows(t, "letmoe", model.RouteModerateText); len(rows) != 1 {
		t.Fatalf("MISS want 1 row (omni), got %d", len(rows))
	}
}

func TestCascadeBothTiersUnconfiguredDegraded(t *testing.T) {
	settings.Override(t, keys.AIEscalateThreshold, 0.4)
	cleanTables(t)
	omni := &fakeOmni{configured: false}
	llm := &fakeUpstream{configured: false}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{})

	res, err := s.Moderate(context.Background(), ModerateParams{Site: "letmoe", Text: "x"})
	if err != nil {
		t.Fatalf("Moderate must never error: %v", err)
	}
	if res.Flagged || !res.Degraded {
		t.Fatalf("both tiers off → fail-open degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != "" {
		t.Fatalf("degraded channel must be empty, got %q", res.Channel)
	}
	if omni.calls != 0 || llm.calls != 0 {
		t.Fatalf("neither tier may dial: omni=%d llm=%d", omni.calls, llm.calls)
	}
	rows := usageRows(t, "letmoe", model.RouteModerateText)
	if len(rows) != 1 || rows[0].Status != model.StatusDegraded {
		t.Fatalf("want 1 degraded row, got %+v", rows)
	}
}
