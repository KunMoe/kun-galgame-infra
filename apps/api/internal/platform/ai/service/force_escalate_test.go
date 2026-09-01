package service

import (
	"context"
	"strings"
	"testing"

	"api/internal/platform/ai/model"
	"api/internal/platform/ai/upstream"
)

func TestForcedEscalationDialsLLMBelowThreshold(t *testing.T) {
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        false,
		Categories:     map[string]bool{},
		CategoryScores: map[string]float64{"violence": 0.01},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat", result: upstream.ChatResult{
		Content: `{"flagged": true, "categories": ["spam"], "score": 0.97}`,
		Channel: "deepseek-chat",
	}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{
		EscalateThreshold: 0.4,
		ForceEscalate:     "kungal:forum_topic",
	})

	res, err := s.Moderate(context.Background(), ModerateParams{
		Site: "kungal", Text: "classified ad", SubjectKind: "forum_topic",
	})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if !res.Flagged || res.Degraded {
		t.Fatalf("want flagged not-degraded, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != "deepseek-chat" {
		t.Fatalf("channel = %q, want deepseek-chat (LLM is final)", res.Channel)
	}
	if omni.calls != 1 {
		t.Fatalf("omni calls = %d, want 1", omni.calls)
	}
	if llm.calls != 1 {
		t.Fatalf("LLM calls = %d, want 1", llm.calls)
	}
	rows := usageRows(t, "kungal", model.RouteModerateText)
	if len(rows) != 2 {
		t.Fatalf("want 2 metered rows (omni + LLM), got %d", len(rows))
	}
	if rows[0].Channel != omniModel || rows[0].Status != model.StatusOK {
		t.Fatalf("omni row wrong: %+v", rows[0])
	}
	if rows[1].Channel != "deepseek-chat" || rows[1].Status != model.StatusOK {
		t.Fatalf("LLM row wrong: %+v", rows[1])
	}
	if !strings.Contains(llm.lastSystem, "Site context (kungal)") {
		t.Fatalf("LLM system prompt missing kungal site context, got %q", llm.lastSystem)
	}
}

func TestForcedEscalationIgnoresUnlistedPair(t *testing.T) {
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        false,
		Categories:     map[string]bool{},
		CategoryScores: map[string]float64{"violence": 0.01},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: true, model: "deepseek-chat",
		result: upstream.ChatResult{Content: `{"flagged": true, "categories": ["spam"], "score": 0.97}`, Channel: "deepseek-chat"}}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{
		EscalateThreshold:  0.4,
		NegativeSampleRate: 0,
		ForceEscalate:      "kungal:forum_topic",
	})

	res, err := s.Moderate(context.Background(), ModerateParams{
		Site: "letmoe", Text: "hi", SubjectKind: "forum_topic",
	})
	if err != nil {
		t.Fatalf("Moderate (letmoe): %v", err)
	}
	if res.Flagged || res.Degraded {
		t.Fatalf("unlisted pair must stay omni-terminal clean, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != omniModel {
		t.Fatalf("channel = %q, want omni", res.Channel)
	}
	if llm.calls != 0 {
		t.Fatalf("unlisted pair must NOT dial LLM, calls=%d", llm.calls)
	}

	res, err = s.Moderate(context.Background(), ModerateParams{
		Site: "kungal", Text: "hi", SubjectKind: "",
	})
	if err != nil {
		t.Fatalf("Moderate (empty kind): %v", err)
	}
	if res.Flagged || res.Degraded {
		t.Fatalf("empty kind must stay omni-terminal clean, got flagged=%v degraded=%v", res.Flagged, res.Degraded)
	}
	if res.Channel != omniModel {
		t.Fatalf("empty kind channel = %q, want omni", res.Channel)
	}
	if llm.calls != 0 {
		t.Fatalf("empty kind must NOT dial LLM, calls=%d", llm.calls)
	}
}

func TestForcedEscalationFallsBackWhenLLMUnconfigured(t *testing.T) {
	cleanTables(t)
	omni := &fakeOmni{configured: true, model: omniModel, result: upstream.OmniResult{
		Flagged:        false,
		Categories:     map[string]bool{},
		CategoryScores: map[string]float64{"violence": 0.01},
		Channel:        omniModel,
	}}
	llm := &fakeUpstream{configured: false}
	s := NewModerationService(testDB, omni, llm, ModerationOptions{
		EscalateThreshold: 0.4,
		ForceEscalate:     "kungal:forum_topic",
	})

	res, err := s.Moderate(context.Background(), ModerateParams{
		Site: "kungal", Text: "classified ad", SubjectKind: "forum_topic",
	})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if res.Flagged {
		t.Fatalf("omni-terminal clean must not flag, flagged=%v", res.Flagged)
	}
	if res.Degraded {
		t.Fatalf("omni-terminal fallback is a REAL verdict — must NOT be degraded")
	}
	if res.Channel != omniModel {
		t.Fatalf("channel = %q, want omni", res.Channel)
	}
	if llm.calls != 0 {
		t.Fatalf("LLM unconfigured must not dial, calls=%d", llm.calls)
	}
}

func TestParseForceEscalate(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"mixed case and spaces", " Kungal:Forum_Topic , kungal:forum_reply ", []string{"kungal:forum_topic", "kungal:forum_reply"}},
		{"no colon", "kungal", nil},
		{"empty site", ":x", nil},
		{"empty kind", "x:", nil},
		{"empty token", "kungal:forum_topic,,", []string{"kungal:forum_topic"}},
		{"empty string", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseForceEscalate(c.raw)
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want %d; set=%v", len(got), len(c.want), got)
			}
			for _, k := range c.want {
				if _, ok := got[k]; !ok {
					t.Fatalf("missing %q in %v", k, got)
				}
			}
		})
	}
}

func TestModerateSystemPromptSiteContext(t *testing.T) {
	kungal := moderateSystemPrompt("kungal")
	if !strings.HasPrefix(kungal, ModerateSystemPrompt) {
		t.Fatal("kungal prompt must start with ModerateSystemPrompt")
	}
	if !strings.Contains(kungal, "Site context (kungal)") {
		t.Fatalf("kungal prompt missing site context")
	}
	if got := moderateSystemPrompt("letmoe"); got != ModerateSystemPrompt {
		t.Fatalf("letmoe prompt must equal ModerateSystemPrompt exactly")
	}
}
