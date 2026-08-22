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
	Limit   int
	Offset  int
	Since   string  // RFC3339; keep only works whose elected zh intro changed since
	WorkIDs []int64 // keep only these works — the retry lane for a failed batch
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
	if len(o.WorkIDs) > 0 {
		want := make(map[int64]bool, len(o.WorkIDs))
		for _, id := range o.WorkIDs {
			want[id] = true
		}
		kept := works[:0:0]
		for _, w := range works {
			if want[w.WorkID] {
				kept = append(kept, w)
			}
		}
		works = kept
	}
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

// extraction is one work's answer: the model's name→passage map, or the error
// that kept it from arriving. A failed work writes nothing and stays a
// candidate, so the next run retries it.
type extraction struct {
	Found map[string]string
	Model string
	Err   error
}

// extractor answers a BATCH of works. Both gateways run the same batched
// envelope (batch.go); they differ only in how one prompt becomes one reply.
type extractor interface {
	ExtractBatch(ctx context.Context, batch []candidateWork) []extraction
}

type httpExtractor struct {
	baseURL   string
	token     string
	model     string
	effort    string
	maxTokens int
	http      *http.Client
}

func newHTTPExtractor(baseURL, token, model, effort string, maxTokens int) *httpExtractor {
	return &httpExtractor{
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		model:     model,
		effort:    effort,
		maxTokens: maxTokens,
		http:      &http.Client{Timeout: 600 * time.Second},
	}
}

func (t *httpExtractor) Configured() bool { return t.baseURL != "" && t.token != "" }

func (t *httpExtractor) ExtractBatch(ctx context.Context, batch []candidateWork) []extraction {
	return extractBatchVia(ctx, t, batch, 0)
}

func (t *httpExtractor) CompareBatch(ctx context.Context, batch []comparison) []comparisonResult {
	return compareBatchVia(ctx, t, batch, 0)
}

// callBatch sends one batched prompt as a chat completion: the rules are the
// system message, the JSON array is the user message.
func (t *httpExtractor) callBatch(ctx context.Context, b batchCall) (string, string, error) {
	body := map[string]any{
		"model":       t.model,
		"max_tokens":  t.maxTokens,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": b.Rules},
			{"role": "user", "content": b.Items},
		},
	}
	if t.effort != "" {
		body["reasoning_effort"] = t.effort
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}
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
	model := cr.Model
	if model == "" {
		model = t.model
	}
	// A cut-off reply keeps its text: the caller halves the batch and retries,
	// and the partial array would fail the integrity gate anyway.
	if fr := cr.Choices[0].FinishReason; fr != "" && fr != "stop" {
		return cr.Choices[0].Message.Content, model, fmt.Errorf("%w (finish_reason=%q)", errBatchOversize, fr)
	}
	return cr.Choices[0].Message.Content, model, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
