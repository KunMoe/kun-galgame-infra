package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"api/internal/platform/catalog/editspec"

	"gorm.io/gorm"
)

// skipMarker is what the model answers when it cannot produce a reliable
// Chinese rendering — the adjudicated 无解保留 outcome (refs/proj/178 §1).
const skipMarker = "SKIP"

const translateSystemPrompt = `你是 galgame(视觉小说)领域的译名专家,负责给角色名提出简体中文写法。
要求:
1. 只输出角色名的简体中文写法本身,不要解释、标点或引号。
2. 假名写法的名字:若存在官方或社区通行的汉字/中文写法,使用它;纯平假名的和风人名可以选用最常见的汉字写法。
3. 片假名的西式名字:使用通行的中文音译(如 アリス → 爱丽丝)。
4. 名字里的罗马字母、数字、符号原样保留。
5. 拿不准、查无通行写法、或该名字本身就该保留原文时,只输出 SKIP。宁可 SKIP,不要自创生僻写法。
6. 参考罗马字和出场作品来消歧。`

type mtResidueCandidate struct {
	ID          int64  `gorm:"column:id"`
	DisplayName string `gorm:"column:display_name"`
	Latin       string `gorm:"column:latin"`
	Uses        int64  `gorm:"column:uses"`
	Works       string `gorm:"column:works"`
}

// loadMTResidue lists the live characters with no zh-family name whose display
// name is NOT pure Han (the passthrough owns those), most-visible first so a
// small reviewed batch covers the names readers actually meet. The work titles
// ride along as prompt/review context.
func loadMTResidue(ctx context.Context, db *gorm.DB, limit int) ([]mtResidueCandidate, error) {
	q := `
	WITH cand AS (
		SELECT c.id, c.display_name, COALESCE(c.latin, '') AS latin,
		       (SELECT count(*) FROM catalog_work_character wc
		         JOIN catalog_work w ON w.id = wc.work_id AND w.deleted_at IS NULL
		        WHERE wc.character_id = c.id) AS uses
		FROM catalog_character c
		WHERE c.deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM catalog_character_alias a
		    WHERE a.character_id = c.id AND a.lang IN ('zh-Hans','zh','zh-Hant'))
	)
	SELECT cand.*, COALESCE((
		SELECT string_agg(t.title, ' / ')
		FROM (
			SELECT DISTINCT ON (wc.work_id) wt.title
			FROM catalog_work_character wc
			JOIN catalog_work_title wt ON wt.work_id = wc.work_id AND wt.kind IN (0,1)
			WHERE wc.character_id = cand.id AND ` + editspec.NotSuppressedWorkTitleSQL("wt") + `
			  AND ` + editspec.NotSuppressedRosterSQL("wc") + `
			ORDER BY wc.work_id, wt.kind, wt.id
			LIMIT 3
		) t), '') AS works
	FROM cand
	WHERE uses > 0
	ORDER BY uses DESC, id`
	var rows []mtResidueCandidate
	if err := db.WithContext(ctx).Raw(q).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load mt residue: %w", err)
	}
	out := rows[:0]
	for _, r := range rows {
		if pureHan(r.DisplayName) {
			continue
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func userMessage(c mtResidueCandidate) string {
	var sb strings.Builder
	sb.WriteString("角色名: ")
	sb.WriteString(c.DisplayName)
	if c.Latin != "" {
		sb.WriteString("\n罗马字: ")
		sb.WriteString(c.Latin)
	}
	if c.Works != "" {
		sb.WriteString("\n出场作品: ")
		sb.WriteString(truncateRunes(c.Works, 200))
	}
	return sb.String()
}

func runMT(ctx context.Context, db *gorm.DB, tr translator, out string, limit int, delay time.Duration) error {
	cands, err := loadMTResidue(ctx, db, limit)
	if err != nil {
		return err
	}
	fmt.Printf("\n=== backfill-char-zh-names MT (residue this run: %d) ===\n", len(cands))
	rows := make([]reviewRow, 0, len(cands))
	var errs, skips int
	usedModel := ""
	for i, c := range cands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i > 0 && delay > 0 {
			time.Sleep(delay)
		}
		zh, m, err := tr.Translate(ctx, c)
		if err != nil {
			errs++
			fmt.Fprintf(os.Stderr, "WARN translate failed character=%d %q: %v\n", c.ID, c.DisplayName, err)
			zh = ""
		}
		if m != "" {
			usedModel = m
		}
		if strings.EqualFold(zh, skipMarker) {
			skips++
			zh = ""
		}
		rows = append(rows, reviewRow{
			CharacterID: c.ID, Name: c.DisplayName, Latin: c.Latin,
			Works: truncateRunes(c.Works, 120), Uses: c.Uses, ProposedZh: zh, Model: usedModel,
		})
		if (i+1)%50 == 0 {
			fmt.Fprintf(os.Stderr, "progress %d/%d errors=%d skips=%d\n", i+1, len(cands), errs, skips)
		}
	}
	if err := writeReviewCSV(out, rows); err != nil {
		return err
	}
	fmt.Printf("proposed=%d skipped_by_model=%d errors=%d → %s\nreview it, then: --apply-csv %s\n",
		len(rows)-errs-skips, skips, errs, out, out)
	if errs > 0 {
		os.Exit(1)
	}
	return nil
}

type translator interface {
	Translate(ctx context.Context, c mtResidueCandidate) (zh string, model string, err error)
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

func (t *httpTranslator) Translate(ctx context.Context, c mtResidueCandidate) (string, string, error) {
	content, model, err := t.chatModel(ctx, translateSystemPrompt, userMessage(c), 0)
	if err != nil {
		return "", "", err
	}
	return cleanProposal(content), model, nil
}

func (t *httpTranslator) chat(ctx context.Context, system, user string, temperature float64) (string, error) {
	content, _, err := t.chatModel(ctx, system, user, temperature)
	return content, err
}

var retryBackoff = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second, 90 * time.Second, 120 * time.Second}

// postChat retries throttled and transient gateway failures with backoff.
// Without it a 429 turns the pacing from inference latency into delay-only fast
// failures, and parallel shards then hammer the gateway at thousands of
// requests per minute exactly when it asked to slow down (the 08-14 intro-panel
// incident; this tool was itself starved twice by a concurrent big chain).
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
	// finish_reason="length" here is a stochastic reasoning death-spiral, not a
	// too-small ceiling: raising max_tokens 4096→16384 left the ramp's length
	// rate unchanged (25%→28%), and a replay probe saw the same name burn all
	// 16384 tokens of reasoning and then pass on a fresh call with 519. A fresh
	// roll is the cure (temp 0 is not deterministic on this gateway); a taller
	// ceiling only multiplies what each spiral burns.
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

func cleanProposal(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, "译名:"))
	s = strings.TrimSpace(strings.TrimPrefix(s, "译名："))
	// The katakana middle dots ・(U+30FB) / ･(U+FF65) sit inside the kana code
	// blocks, so a fully translated name that reuses the source's separator
	// ("双叶・莉莉・拉姆塞斯") was killed by the kana-left pre-gate in rehearsal.
	// Normalize to the zh alias convention · before any gate sees the string.
	s = strings.NewReplacer("・", "·", "･", "·").Replace(s)
	return strings.Trim(s, " \t\"'“”「」。.")
}

var csvHeader = []string{"character_id", "name", "latin", "works", "uses", "proposed_zh", "model"}

type reviewRow struct {
	CharacterID int64
	Name        string
	Latin       string
	Works       string
	Uses        int64
	ProposedZh  string
	Model       string
}

func writeReviewCSV(path string, rows []reviewRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			strconv.FormatInt(r.CharacterID, 10), r.Name, r.Latin, r.Works,
			strconv.FormatInt(r.Uses, 10), r.ProposedZh, r.Model,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
