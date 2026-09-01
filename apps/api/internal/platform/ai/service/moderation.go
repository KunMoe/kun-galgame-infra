package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	"api/internal/platform/ai/model"
	"api/internal/platform/ai/route"
	"api/internal/platform/ai/upstream"

	"gorm.io/gorm"
)

type upstreamClient interface {
	Configured() bool
	Model() string
	ChatJSON(ctx context.Context, system, user string, maxTokens int) (upstream.ChatResult, error)
}

type omniClient interface {
	Configured() bool
	Model() string
	Moderate(ctx context.Context, input string) (upstream.OmniResult, error)
}

type ModerationService struct {
	db   *gorm.DB
	omni omniClient
	llm  upstreamClient

	escalateThreshold  float32
	negativeSampleRate float64
	rand               func() float64
	retryDelay         time.Duration
	forceEscalate      map[string]struct{}
}

type ModerationOptions struct {
	EscalateThreshold  float32
	NegativeSampleRate float64
	Rand               func() float64
	ForceEscalate      string
}

func NewModerationService(db *gorm.DB, omni omniClient, llm upstreamClient, opts ModerationOptions) *ModerationService {
	r := opts.Rand
	if r == nil {
		r = rand.Float64
	}
	return &ModerationService{
		db:                 db,
		omni:               omni,
		llm:                llm,
		escalateThreshold:  opts.EscalateThreshold,
		negativeSampleRate: opts.NegativeSampleRate,
		rand:               r,
		retryDelay:         moderateRetryDelay,
		forceEscalate:      parseForceEscalate(opts.ForceEscalate),
	}
}

func parseForceEscalate(raw string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.ToLower(strings.TrimSpace(pair))
		site, kind, ok := strings.Cut(pair, ":")
		if !ok || site == "" || kind == "" {
			continue
		}
		set[site+":"+kind] = struct{}{}
	}
	return set
}

func (s *ModerationService) forceEscalated(site, kind string) bool {
	if kind == "" {
		return false
	}
	_, ok := s.forceEscalate[strings.ToLower(site)+":"+strings.ToLower(kind)]
	return ok
}

type ModerateParams struct {
	Site        string
	Text        string
	AuthorID    *int64
	SubjectKind string
}

type ModerateResult struct {
	Route      string
	Flagged    bool
	Categories []string
	Score      *float32
	Channel    string
	Degraded   bool
}

const ModerateSystemPrompt = `You are a content-safety classifier for a community platform.
Judge the user message for policy violations (abuse/harassment, spam, illegal content, sexual content involving minors, or other clearly harmful content).
Respond with ONLY a JSON object, no prose, of the form:
{"flagged": <bool>, "categories": [<string>, ...], "score": <number between 0 and 1>}
"flagged" is true only when the message clearly violates policy; "score" is your confidence that it violates policy.`

var siteModerationContext = map[string]string{
	"kungal": `

Site context (kungal): the text is from a Chinese-language visual-novel (galgame) community forum. Discussion of adult (R18) games, requests for game resources/patches, and lewd banter about fictional game content are on-policy here — do not flag them as sexual content. The dominant real abuse is commercial solicitation spam: escort/prostitution classified ads (外送茶, 楼凤, 上门, 空降, 兼职 companions, prices with LINE / Telegram / WeChat contact handles), gambling or casino promotion, and other off-platform recruitment. Flag those as spam with high confidence even when the wording is not explicitly sexual.`,
}

func moderateSystemPrompt(site string) string {
	return ModerateSystemPrompt + siteModerationContext[site]
}

// moderateMaxTokens caps the reply. The verdict object itself is ~40 tokens, so
// this looks generous — but it is a CEILING, not a budget: a model that answers
// in 40 tokens is billed for 40 whichever ceiling is set, so raising it costs
// nothing on the replies that already fit and rescues the ones that did not.
//
// It was 256, which silently destroyed half of all escalated verdicts between
// 2026-07-22 and 2026-08-07. Reasoning-style models (the Tier2 channel was
// @cf/zai-org/glm-5.2) emit their working before the JSON; past 256 the object
// was cut mid-token and came back as "unexpected end of JSON input", which
// fail-open turned into an allow. The evidence was unambiguous once looked at:
// successful calls had a median of 132 completion tokens, failed calls a median
// of exactly 256, with 73% pinned to the ceiling.
//
// Tighten this only against measured completion_tokens in ai_usage, never by
// eyeballing the size of the verdict — the verdict is not what fills the budget.
const moderateMaxTokens = 1024

// moderateRetryDelay waits out a 429 burst before the single retry. Every
// observed prod 429 landed at :35-:36 past the minute — a scheduled caller, not
// a person waiting on a form — so seconds here cost no user latency. It is 3s
// rather than 1s because two of the observed failures were one second apart:
// a 1s retry would have landed inside the same burst.
const moderateRetryDelay = 3 * time.Second

func (s *ModerationService) Moderate(ctx context.Context, p ModerateParams) (ModerateResult, error) {
	routeName := model.RouteModerateText
	spec, _ := route.Lookup(routeName)

	start := time.Now()
	if s.overBudget(ctx, routeName, p.Site) {
		slog.Warn("ai moderate-text over budget — fail-open allow", "site", p.Site, "route", routeName)
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: model.StatusBudgetDenied,
			LatencyMs: msSince(start),
		})
		return failOpen(routeName, "", spec), nil
	}

	if !s.omni.Configured() {
		return s.moderateViaLLM(ctx, p, routeName, spec)
	}

	omniStart := time.Now()
	ores, oerr := s.omni.Moderate(ctx, p.Text)
	if oerr != nil {
		slog.Warn("ai moderate-text omni error — falling through to LLM", "site", p.Site, "err", oerr)
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: model.StatusUpstreamError,
			Channel: s.omni.Model(), LatencyMs: msSince(omniStart),
		})
		return s.moderateViaLLM(ctx, p, routeName, spec)
	}
	s.meter(ctx, model.AIUsage{
		Site: p.Site, Route: routeName, Status: model.StatusOK,
		Channel: ores.Channel, LatencyMs: msSince(omniStart),
	})

	rel := relevantScore(ores.CategoryScores)

	if s.llm.Configured() && s.forceEscalated(p.Site, p.SubjectKind) {
		return s.moderateViaLLM(ctx, p, routeName, spec)
	}

	if rel >= s.escalateThreshold {
		if s.llm.Configured() {
			return s.moderateViaLLM(ctx, p, routeName, spec)
		}
		score := rel
		return ModerateResult{
			Route: routeName, Flagged: true, Categories: adoptedTrueCategories(ores.Categories),
			Score: &score, Channel: ores.Channel, Degraded: false,
		}, nil
	}

	if s.llm.Configured() && s.sampleHit() {
		return s.moderateViaLLM(ctx, p, routeName, spec)
	}
	score := rel
	return ModerateResult{
		Route: routeName, Flagged: false, Categories: nil,
		Score: &score, Channel: ores.Channel, Degraded: false,
	}, nil
}

func (s *ModerationService) moderateViaLLM(ctx context.Context, p ModerateParams, routeName string, spec route.Spec) (ModerateResult, error) {
	start := time.Now()

	if !s.llm.Configured() {
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: model.StatusDegraded,
			LatencyMs: msSince(start),
		})
		return failOpen(routeName, "", spec), nil
	}

	res, err := s.llm.ChatJSON(ctx, moderateSystemPrompt(p.Site), p.Text, moderateMaxTokens)
	if err != nil && upstream.IsRateLimited(err) {
		// The Cloudflare inference bucket is per-minute and shared with every
		// other caller on the account, so a 429 here is contention, not a
		// verdict — but the fail-open below turns it into an allow. In 2026-08 a
		// batch job on the same account made prod moderation fail open this way
		// dozens of times a day, each one silently admitting unscanned content.
		// One retry; the metered StatusRateLimited row is what makes it visible.
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: model.StatusRateLimited,
			Channel: s.llm.Model(), LatencyMs: msSince(start),
		})
		timer := time.NewTimer(s.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
			res, err = s.llm.ChatJSON(ctx, moderateSystemPrompt(p.Site), p.Text, moderateMaxTokens)
		}
	}
	if err != nil {
		slog.Warn("ai moderate-text upstream error — fail-open allow", "site", p.Site, "err", err)
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: model.StatusUpstreamError,
			Channel: s.llm.Model(), LatencyMs: msSince(start),
		})
		return failOpen(routeName, "", spec), nil
	}
	v, perr := parseModeration(res.Content)
	if perr != nil {
		status := model.StatusUpstreamError
		if res.FinishReason == "length" {
			status = model.StatusTruncated
			slog.Warn("ai moderate-text reply truncated at the token ceiling — fail-open allow; raise moderateMaxTokens",
				"site", p.Site, "channel", res.Channel, "max_tokens", moderateMaxTokens,
				"completion_tokens", res.CompletionTokens)
		} else {
			slog.Warn("ai moderate-text unparseable upstream reply — fail-open allow",
				"site", p.Site, "channel", res.Channel, "finish_reason", res.FinishReason, "err", perr)
		}
		s.meter(ctx, model.AIUsage{
			Site: p.Site, Route: routeName, Status: status,
			Channel: res.Channel, PromptTokens: res.PromptTokens, CompletionTokens: res.CompletionTokens,
			LatencyMs: msSince(start),
		})
		return failOpen(routeName, res.Channel, spec), nil
	}

	s.meter(ctx, model.AIUsage{
		Site: p.Site, Route: routeName, Status: model.StatusOK,
		Channel: res.Channel, PromptTokens: res.PromptTokens, CompletionTokens: res.CompletionTokens,
		LatencyMs: msSince(start),
	})
	return ModerateResult{
		Route: routeName, Flagged: v.Flagged, Categories: v.Categories, Score: v.Score,
		Channel: res.Channel, Degraded: false,
	}, nil
}

func (s *ModerationService) sampleHit() bool {
	if s.negativeSampleRate <= 0 {
		return false
	}
	return s.rand() < s.negativeSampleRate
}

func failOpen(routeName, channel string, _ route.Spec) ModerateResult {
	return ModerateResult{Route: routeName, Flagged: false, Channel: channel, Degraded: true}
}

func (s *ModerationService) overBudget(ctx context.Context, routeName, site string) bool {
	capMicro := s.resolveCap(ctx, routeName, site)
	if capMicro == nil {
		return false
	}
	var spent int64
	if err := s.db.WithContext(ctx).Model(&model.AIUsage{}).
		Where("route = ? AND site = ? AND created_at >= date_trunc('day', now())", routeName, site).
		Select("COALESCE(SUM(cost_micro), 0)").Scan(&spent).Error; err != nil {
		slog.Warn("ai budget spend query failed — fail-open (no block)", "site", site, "route", routeName, "err", err)
		return false
	}
	return spent >= *capMicro
}

func (s *ModerationService) resolveCap(ctx context.Context, routeName, site string) *int64 {
	scopes := []string{site}
	if site != "" {
		scopes = append(scopes, "")
	}
	for _, sc := range scopes {
		var b model.AIRouteBudget
		err := s.db.WithContext(ctx).Where("route = ? AND site = ?", routeName, sc).Take(&b).Error
		if err == nil {
			return b.DailyCostCapMicro
		}
	}
	return nil
}

func (s *ModerationService) meter(ctx context.Context, row model.AIUsage) {
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		slog.Warn("ai usage metering failed (non-fatal)", "site", row.Site, "route", row.Route, "status", row.Status, "err", err)
	}
}

func msSince(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}

type verdict struct {
	Flagged    bool
	Categories []string
	Score      *float32
}

func parseModeration(content string) (verdict, error) {
	raw := extractJSON(content)
	var v struct {
		Flagged    bool     `json:"flagged"`
		Categories []string `json:"categories"`
		Score      *float32 `json:"score"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return verdict{}, err
	}
	return verdict{Flagged: v.Flagged, Categories: v.Categories, Score: v.Score}, nil
}

func extractJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}
