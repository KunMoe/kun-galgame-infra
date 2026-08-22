package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api/internal/platform/catalog/editspec"

	"gorm.io/gorm"
)

// candidateWork is one live work whose zh-Hans intro may embed introductions
// for roster characters that still have no zh-Hans intro row.
type candidateWork struct {
	WorkID int64
	Intro  string
	// Roster is every nameable target: only characters MISSING a zh intro.
	// Characters that already have one are not even offered to the model —
	// the panel bucket is a different wave.
	Roster []rosterChar
}

type rosterChar struct {
	CharacterID int64
	Name        string // display name — the key the model must answer with
	ZhName      string // best zh alias, shown as an aid, may equal Name
	Incumbent   string // panel bucket only: the elected translated machine intro
	DerivedID   int64  // refresh bucket only: the stale derived row to rewrite
	DerivedHash string // refresh bucket only: its src_hash, the update guard
}

// candidateOpts is the window every bucket's loader applies identically.
type candidateOpts struct {
	Limit  int
	Offset int
	Since  string // RFC3339; keep only works whose elected zh intro changed since
}

// sinceClause filters on the ELECTED intro's updated_at — the row the zhi CTE
// already picked — so it narrows the work list without changing which intro
// each work contributes.
func sinceClause(since string) string {
	if since == "" {
		return ""
	}
	return `
	  AND zhi.updated_at >= ?`
}

func sinceArgs(since string) []any {
	if since == "" {
		return nil
	}
	return []any{since}
}

func window(works []candidateWork, o candidateOpts) []candidateWork {
	if o.Offset > 0 {
		if o.Offset >= len(works) {
			return nil
		}
		works = works[o.Offset:]
	}
	if o.Limit > 0 && len(works) > o.Limit {
		works = works[:o.Limit]
	}
	return works
}

var candidateWorksSQL = `
	WITH zhi AS (
		SELECT DISTINCT ON (work_id) work_id, intro, updated_at
		FROM catalog_work_intro
		WHERE lang = 'zh-Hans' AND length(intro) >= 200
		ORDER BY work_id, source_id
	)
	SELECT w.id AS work_id, zhi.intro,
	       wc.character_id, c.display_name,
	       COALESCE((
	         SELECT a.name FROM catalog_character_alias a
	         WHERE a.character_id = c.id AND a.lang IN ('zh-Hans','zh','zh-Hant') AND a.kind IN (0,1)
	         ORDER BY (NOT a.is_primary_for_locale), (a.lang <> 'zh-Hans'), a.id LIMIT 1
	       ), '') AS zh_name
	FROM catalog_work w
	JOIN zhi ON zhi.work_id = w.id
	JOIN catalog_work_character wc ON wc.work_id = w.id
	JOIN catalog_character c ON c.id = wc.character_id AND c.deleted_at IS NULL
	WHERE w.deleted_at IS NULL
	  AND ` + editspec.NotSuppressedRosterSQL("wc") + `
	  AND NOT EXISTS (
	    SELECT 1 FROM catalog_character_intro ci
	    WHERE ci.character_id = wc.character_id AND ci.lang = 'zh-Hans')`

func loadCandidateWorks(ctx context.Context, db *gorm.DB, o candidateOpts) ([]candidateWork, error) {
	sql := candidateWorksSQL + sinceClause(o.Since) + `
	ORDER BY w.id, wc.character_id`
	var rows []struct {
		WorkID      int64  `gorm:"column:work_id"`
		Intro       string `gorm:"column:intro"`
		CharacterID int64  `gorm:"column:character_id"`
		DisplayName string `gorm:"column:display_name"`
		ZhName      string `gorm:"column:zh_name"`
	}
	if err := db.WithContext(ctx).Raw(sql, sinceArgs(o.Since)...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load candidate works: %w", err)
	}
	var out []candidateWork
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].WorkID != r.WorkID {
			out = append(out, candidateWork{WorkID: r.WorkID, Intro: r.Intro})
		}
		cur := &out[len(out)-1]
		cur.Roster = append(cur.Roster, rosterChar{
			CharacterID: r.CharacterID, Name: r.DisplayName, ZhName: r.ZhName,
		})
	}
	return window(out, o), nil
}

const extractSystemPrompt = `你从视觉小说(galgame)的作品简介中摘录角色介绍。给你一段简介正文和一份角色名单。
要求:
1. 只摘录,不写作:输出的每一段都必须是简介正文中逐字存在的句子(可以去掉行首的名字标头、括号注音、项目符号,可以合并同一角色相邻的句子,但不得改写、缩写、补写任何内容)。
2. 只收录名单中的角色,且只在简介确实包含对该角色的介绍性描述时收录。介绍性描述=说明该角色的身份、性格、外貌或与他人的关系;叙述剧情事件时顺带出现角色名的句子不是介绍,即使那是该角色在简介中唯一出现的地方,也不收。宁缺毋滥,拿不准就不收。
   只有当简介正文中确实出现该角色的名字(或给出的中文名)时才可收录该角色;不要凭姓氏或身份推断"这段说的是他"。
3. 输出严格的 JSON 对象:键 = 名单中给出的角色名(逐字使用名单写法),值 = 摘录的介绍文本。没有任何可摘录的角色介绍时输出 {}。
4. 只输出 JSON,不要输出解释或代码块围栏。`

func extractUserMessage(c candidateWork) string {
	var sb strings.Builder
	sb.WriteString("角色名单:\n")
	for _, r := range c.Roster {
		sb.WriteString("- ")
		sb.WriteString(r.Name)
		if r.ZhName != "" && r.ZhName != r.Name {
			sb.WriteString("(中文名: ")
			sb.WriteString(r.ZhName)
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n作品简介:\n")
	sb.WriteString(c.Intro)
	return sb.String()
}

// extraction is one work's answer: the model's name→passage map, or the error
// that kept it from arriving. A failed work writes nothing and stays a
// candidate, so the next run retries it.
type extraction struct {
	Found map[string]string
	Model string
	Err   error
}

// extractor answers a BATCH of works. The OpenAI-compatible gateway has no
// batch face and its adapter just loops; Cursor's only face is a batch, and
// there the batch is what makes the lane affordable.
type extractor interface {
	ExtractBatch(ctx context.Context, batch []candidateWork) []extraction
}

func (t *httpExtractor) ExtractBatch(ctx context.Context, batch []candidateWork) []extraction {
	out := make([]extraction, len(batch))
	for i, c := range batch {
		found, model, err := t.Extract(ctx, c)
		out[i] = extraction{Found: found, Model: model, Err: err}
	}
	return out
}

type httpExtractor struct {
	baseURL   string
	token     string
	model     string
	maxTokens int
	http      *http.Client
}

func newHTTPExtractor(baseURL, token, model string, maxTokens int) *httpExtractor {
	return &httpExtractor{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		model:     model,
		maxTokens: maxTokens,
		http:      &http.Client{Timeout: 300 * time.Second},
	}
}

func (t *httpExtractor) Configured() bool { return t.baseURL != "" && t.token != "" }

func (t *httpExtractor) Extract(ctx context.Context, c candidateWork) (map[string]string, string, error) {
	body := map[string]any{
		"model":       t.model,
		"max_tokens":  t.maxTokens,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": extractSystemPrompt},
			{"role": "user", "content": extractUserMessage(c)},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	data, err := t.postChat(ctx, raw)
	if err != nil {
		return nil, "", err
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
		return nil, "", fmt.Errorf("decode chat response: %w", err)
	}
	if cr.Error != nil {
		return nil, "", fmt.Errorf("gateway error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return nil, "", fmt.Errorf("gateway returned no choices")
	}
	if fr := cr.Choices[0].FinishReason; fr != "" && fr != "stop" {
		return nil, "", fmt.Errorf("generation finished with finish_reason=%q — refusing partial output", fr)
	}
	found, err := parseExtraction(cr.Choices[0].Message.Content)
	if err != nil {
		return nil, "", err
	}
	model := cr.Model
	if model == "" {
		model = t.model
	}
	return found, model, nil
}

// parseExtraction decodes the model's JSON object, tolerating a stray code
// fence. Anything that is not a flat string→string object is an error.
func parseExtraction(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty extraction output")
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("extraction is not a flat JSON object: %w", err)
	}
	return out, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
