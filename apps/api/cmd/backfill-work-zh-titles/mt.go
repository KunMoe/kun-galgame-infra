package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// skipMarker is what the model answers when it cannot produce a reliable
// Chinese title — the same 无解保留 outcome the char-name lane adjudicated.
const skipMarker = "SKIP"

const translateSystemPrompt = `你是 galgame(视觉小说)领域的译名专家,负责给作品标题提出简体中文写法。
要求:
1. 只输出作品标题的简体中文写法本身,不要解释、标点或引号。
2. 标题中的品牌名、系列名、角色名若已有通行中文写法,使用它;没有通行写法的专有名词保留原文。
3. 罗马字母、数字、符号、副标题分隔符原样保留。
4. 同系列作品必须共用同一个系列名译法;下面若给出了同系列已有译名,必须沿用其写法。
5. 拿不准、查无通行写法、或该标题本身就该保留原文时,只输出 SKIP。宁可 SKIP,不要自创生僻写法。`

const verifySystemPrompt = `你是 galgame(视觉小说)领域的译名审校。判断给出的简体中文标题对于该作品原名是否正确、自然、符合通行译法,并且与同系列已有译名一致。
只输出一行:「通过」或「不通过」。拿不准就输出「不通过」。`

const refuteSystemPrompt = `你是挑剔的译名质检员。找出下面这个简体中文作品标题的问题:错译、自创生僻写法、残留假名、与同系列已有译名不一致、与官方或社区通行译法冲突等。
若确实挑不出问题,只输出「无误」;否则只输出一行最主要的问题。`

// rederiveSystemPrompt must NOT be the translate prompt verbatim: at
// temperature 0 an identical request would trivially re-emit the identical
// answer and this vote would always pass. Different phrasing + temperature 0.3
// turn agreement into a stability probe instead.
const rederiveSystemPrompt = `给出这个 galgame 作品原名最通行的简体中文标题。只输出标题本身,不要解释。
系列作品沿用系列的通行译法。没有把握就只输出 SKIP。`

// userMessage carries the batch context: the sibling titles already translated
// (Known) and the other members of the same batch, which is what makes a series
// come out with one name instead of one per work.
func userMessage(c mtCandidate, known, batchmates []string) string {
	var sb strings.Builder
	sb.WriteString("作品原名: ")
	sb.WriteString(cleanTitle(c.JaTitle))
	if len(known) > 0 {
		sb.WriteString("\n同系列已有译名:\n")
		for _, k := range known {
			sb.WriteString("  ")
			sb.WriteString(truncateRunes(k, 120))
			sb.WriteString("\n")
		}
	}
	if len(batchmates) > 0 {
		sb.WriteString("\n同批待译(仅供保持系列名一致,不要翻译它们):\n")
		for _, m := range batchmates {
			sb.WriteString("  ")
			sb.WriteString(truncateRunes(m, 120))
			sb.WriteString("\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func batchmatesOf(b mtBatch, self int64) []string {
	out := make([]string, 0, len(b.Members))
	for _, m := range b.Members {
		if m.WorkID == self {
			continue
		}
		out = append(out, cleanTitle(m.JaTitle))
	}
	return out
}

type translator interface {
	Translate(ctx context.Context, c mtCandidate, known, batchmates []string) (zh string, model string, err error)
	Chat(ctx context.Context, system, user string, temperature float64) (string, error)
}

type httpTranslator struct {
	baseURL   string
	token     string
	model     string
	maxTokens int
	http      *http.Client
}

func newHTTPTranslator(baseURL, token, model string, maxTokens int) *httpTranslator {
	return &httpTranslator{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		model:     model,
		maxTokens: maxTokens,
		http:      &http.Client{Timeout: 300 * time.Second},
	}
}

func (t *httpTranslator) Configured() bool { return t.baseURL != "" && t.token != "" }

func (t *httpTranslator) Translate(ctx context.Context, c mtCandidate, known, mates []string) (string, string, error) {
	content, model, err := t.chatModel(ctx, translateSystemPrompt, userMessage(c, known, mates), 0)
	if err != nil {
		return "", "", err
	}
	return cleanTitle(content), model, nil
}

func (t *httpTranslator) Chat(ctx context.Context, system, user string, temperature float64) (string, error) {
	content, _, err := t.chatModel(ctx, system, user, temperature)
	return content, err
}

var retryBackoff = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second, 90 * time.Second, 120 * time.Second}

// postChat retries throttled and transient gateway failures with backoff.
// Without it a 429 turns the pacing from inference latency into delay-only fast
// failures, and parallel shards then hammer the gateway exactly when it asked
// to slow down (the 08-14 intro-panel incident).
func (t *httpTranslator) postChat(ctx context.Context, raw []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, status, err := t.postOnce(ctx, raw)
		if err == nil {
			return data, nil
		}
		retryable := status == 0 || status == http.StatusTooManyRequests ||
			status == http.StatusRequestTimeout || status >= http.StatusInternalServerError
		if !retryable || attempt >= len(retryBackoff) {
			return nil, err
		}
		select {
		case <-time.After(retryBackoff[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (t *httpTranslator) postOnce(ctx context.Context, raw []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.token)
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("gateway http %d: %s", resp.StatusCode, truncateRunes(string(data), 300))
	}
	return data, resp.StatusCode, nil
}

func (t *httpTranslator) chatModel(ctx context.Context, system, user string, temperature float64) (string, string, error) {
	body := map[string]any{
		"model":       t.model,
		"max_tokens":  t.maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}
	// finish_reason="length" is a stochastic reasoning death-spiral, not a
	// too-small ceiling: a taller ceiling only multiplies what each spiral
	// burns, while a fresh roll clears it (temp 0 is not deterministic on this
	// gateway). Measured on the char-name lane, 08-18.
	var lastErr error
	for roll := 0; roll < 3; roll++ {
		data, err := t.postChat(ctx, raw)
		if err != nil {
			return "", "", err
		}
		var cr struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &cr); err != nil {
			return "", "", fmt.Errorf("decode chat response: %w", err)
		}
		if cr.Error != nil {
			return "", "", fmt.Errorf("gateway error: %s", cr.Error.Message)
		}
		if len(cr.Choices) == 0 {
			return "", "", fmt.Errorf("gateway returned no choices")
		}
		if fr := cr.Choices[0].FinishReason; fr != "" && fr != "stop" {
			lastErr = fmt.Errorf("generation finished with finish_reason=%q — refusing partial output", fr)
			continue
		}
		model := cr.Model
		if model == "" {
			model = t.model
		}
		return cr.Choices[0].Message.Content, model, nil
	}
	return "", "", lastErr
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(strings.TrimSpace(s), "「」。.\"'")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
