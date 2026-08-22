package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Cursor lane. Cursor exposes no chat/completions route — api.cursor.com is
// Admin/Analytics/Cloud-Agents only — so the one inference face is an agent
// run: POST /v1/agents, then poll the run until it reaches a terminal status
// and read its result text. Every run pays ~14k input tokens of fixed
// overhead, so the BATCH is the economic lever, not the concurrency: wave 210
// measured $0.00032 per item at batch 200 against $0.0008 at batch 50.
const cursorBase = "https://api.cursor.com"

type cursorClient struct {
	base    string
	token   string
	model   string
	effort  string
	http    *http.Client
	timeout time.Duration
	poll    time.Duration
}

func newCursorClient(token, model, effort string, timeout time.Duration) *cursorClient {
	return &cursorClient{
		base:    cursorBase,
		token:   token,
		model:   model,
		effort:  effort,
		http:    &http.Client{Timeout: 120 * time.Second},
		timeout: timeout,
		poll:    5 * time.Second,
	}
}

func (c *cursorClient) Configured() bool { return c.token != "" }

// runAgent creates one agent run and blocks until it reports a terminal status.
func (c *cursorClient) runAgent(ctx context.Context, prompt, label string) (string, error) {
	body := map[string]any{
		"prompt": map[string]any{"text": prompt},
		"model": map[string]any{
			"id": c.model,
			"params": []map[string]any{
				{"id": "effort", "value": c.effort},
				{"id": "fast", "value": "true"},
			},
		},
		"name": label,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	created, err := c.request(ctx, http.MethodPost, "/v1/agents", raw)
	if err != nil {
		return "", err
	}
	agentID, runID := cursorIDs(created)
	if agentID == "" || runID == "" {
		return "", fmt.Errorf("create agent: no run in reply: %s", truncate(string(created), 200))
	}
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-time.After(c.poll):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		data, err := c.request(ctx, http.MethodGet, "/v1/agents/"+agentID+"/runs/"+runID, nil)
		if err != nil {
			return "", err
		}
		var r struct {
			Run *struct {
				Status string `json:"status"`
				Result string `json:"result"`
			} `json:"run"`
			Status string `json:"status"`
			Result string `json:"result"`
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return "", fmt.Errorf("decode run: %w", err)
		}
		status, result := r.Status, r.Result
		if r.Run != nil {
			status, result = r.Run.Status, r.Run.Result
		}
		switch status {
		case "FINISHED":
			return result, nil
		case "ERROR", "CANCELLED", "EXPIRED":
			return "", fmt.Errorf("run %s: %s", status, truncate(result, 200))
		}
	}
	return "", fmt.Errorf("run %s exceeded %s", runID, c.timeout)
}

func cursorIDs(created []byte) (agentID, runID string) {
	var resp struct {
		ID    string `json:"id"`
		Agent *struct {
			ID string `json:"id"`
		} `json:"agent"`
		Run *struct {
			ID string `json:"id"`
		} `json:"run"`
	}
	if err := json.Unmarshal(created, &resp); err != nil {
		return "", ""
	}
	agentID = resp.ID
	if resp.Agent != nil && resp.Agent.ID != "" {
		agentID = resp.Agent.ID
	}
	if resp.Run != nil {
		runID = resp.Run.ID
	}
	return agentID, runID
}

func (c *cursorClient) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, status, err := c.requestOnce(ctx, method, path, body)
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

func (c *cursorClient) requestOnce(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("cursor http %d: %s", resp.StatusCode, truncate(string(data), 300))
	}
	return data, resp.StatusCode, nil
}

const batchEnvelopeRules = `下面是一个 JSON 数组,每项是一个待处理条目,` + "`i`" + ` 是它的序号。
逐项独立处理,输出 JSON 数组 %s。
必须满足:输出项数与输入完全一致;每项的 ` + "`i`" + ` 原样回显;顺序与输入一致。
把 JSON 数组直接作为你的最终回复正文输出:不要解释、不要代码围栏、不要写入任何文件、不要使用任何工具。`

func batchPrompt(rules, outShape string, items any) (string, error) {
	raw, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return rules + "\n\n" + fmt.Sprintf(batchEnvelopeRules, outShape) + "\n\n" + string(raw), nil
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

// parseBatchArray is the integrity gate: the reply must carry exactly one
// object per wanted index, no more, no fewer. A model that silently drops the
// tail (observed at batch 1000: 418 of 1000 returned) fails here instead of
// quietly shifting every downstream passage onto the wrong work.
func parseBatchArray[T any](text string, want int) ([]T, error) {
	m := jsonArrayRe.FindString(text)
	if m == "" {
		return nil, fmt.Errorf("no JSON array in reply: %s", truncate(strings.TrimSpace(text), 200))
	}
	var arr []struct {
		I *int `json:"i"`
	}
	if err := json.Unmarshal([]byte(m), &arr); err != nil {
		return nil, fmt.Errorf("reply is not a JSON array of objects: %w", err)
	}
	if len(arr) != want {
		return nil, fmt.Errorf("count mismatch: got %d want %d", len(arr), want)
	}
	var items []T
	if err := json.Unmarshal([]byte(m), &items); err != nil {
		return nil, fmt.Errorf("reply items do not match the requested shape: %w", err)
	}
	seen := make(map[int]bool, want)
	for _, a := range arr {
		if a.I == nil {
			return nil, fmt.Errorf("item missing index key")
		}
		seen[*a.I] = true
	}
	for i := range want {
		if !seen[i] {
			return nil, fmt.Errorf("index set mismatch: %d missing", i)
		}
	}
	return items, nil
}

// --- extraction ---

type cursorExtractItem struct {
	I      int      `json:"i"`
	Roster []string `json:"角色名单"`
	Intro  string   `json:"作品简介"`
}

type cursorExtractReply struct {
	I     int               `json:"i"`
	Found map[string]string `json:"摘录"`
}

const extractOutShape = `[{"i":0,"摘录":{"角色名":"摘录到的介绍文本"}}]`

// splitRetries is how many times a batch that fails the integrity gate is cut
// in half and retried. One level is what wave 210 settled on: it rescues the
// batch that was merely too big without letting a systematically bad item turn
// one run into a binary search.
const splitRetries = 1

func (c *cursorClient) ExtractBatch(ctx context.Context, batch []candidateWork) []extraction {
	return c.extractBatch(ctx, batch, 0)
}

func (c *cursorClient) extractBatch(ctx context.Context, batch []candidateWork, depth int) []extraction {
	out := make([]extraction, len(batch))
	items := make([]cursorExtractItem, len(batch))
	plain := make([]map[string]string, len(batch))
	for i, w := range batch {
		names := make([]string, 0, len(w.Roster))
		plain[i] = make(map[string]string, len(w.Roster))
		for _, r := range w.Roster {
			n := r.Name
			if r.ZhName != "" && r.ZhName != r.Name {
				n += "(中文名: " + r.ZhName + ")"
			}
			names = append(names, n)
			plain[i][n] = r.Name
		}
		items[i] = cursorExtractItem{I: i, Roster: names, Intro: w.Intro}
	}
	rules := extractSystemPrompt + `
5. ` + "`摘录`" + ` 的键必须逐字使用「角色名单」里给出的写法(带中文名括注的,只用括注前的名字);没有可摘录的角色介绍时该项输出空对象 {}。
6. 摘录值必须是从简介正文里**原样复制**的连续文本:一个字都不能改、不能顺句、不能换标点、不能把相隔的句子拼起来。程序会逐字比对,改写过的一律丢弃——与其润色不如原样照抄。`
	prompt, err := batchPrompt(rules, extractOutShape, items)
	if err != nil {
		return failAll(out, err)
	}
	text, err := c.runAgent(ctx, prompt, fmt.Sprintf("p3-extract-%d", batch[0].WorkID))
	if err != nil {
		return failAll(out, err)
	}
	replies, err := parseBatchArray[cursorExtractReply](text, len(batch))
	if err != nil {
		if depth < splitRetries && len(batch) > 1 {
			slog.Warn("extraction batch failed the integrity gate — splitting", "works", len(batch), "err", err)
			half := len(batch) / 2
			return append(c.extractBatch(ctx, batch[:half], depth+1), c.extractBatch(ctx, batch[half:], depth+1)...)
		}
		return failAll(out, err)
	}
	for _, r := range replies {
		if r.I < 0 || r.I >= len(out) {
			continue
		}
		// The roster is shown as 「名字(中文名: …)」, and a model that echoes the
		// whole label back would land in the unmatched-name bucket; map it home.
		found := make(map[string]string, len(r.Found))
		for name, passage := range r.Found {
			if p, ok := plain[r.I][strings.TrimSpace(name)]; ok {
				name = p
			}
			found[name] = passage
		}
		out[r.I] = extraction{Found: found, Model: c.model}
	}
	return out
}

func failAll(out []extraction, err error) []extraction {
	for i := range out {
		out[i] = extraction{Err: err}
	}
	return out
}

// --- panel judging ---

type cursorJudgeItem struct {
	I    int    `json:"i"`
	Name string `json:"角色"`
	A    string `json:"A"`
	B    string `json:"B"`
}

type cursorJudgeReply struct {
	I      int    `json:"i"`
	Winner string `json:"winner"`
}

const judgeOutShape = `[{"i":0,"winner":"A"}]`

func (c *cursorClient) CompareBatch(ctx context.Context, batch []comparison) []comparisonResult {
	return c.compareBatch(ctx, batch, 0)
}

func (c *cursorClient) compareBatch(ctx context.Context, batch []comparison, depth int) []comparisonResult {
	out := make([]comparisonResult, len(batch))
	items := make([]cursorJudgeItem, len(batch))
	for i, cmp := range batch {
		first, second := cmp.Incumbent, cmp.Challenger
		if cmp.ChallengerFirst {
			first, second = cmp.Challenger, cmp.Incumbent
		}
		items[i] = cursorJudgeItem{I: i, Name: cmp.Name, A: first, B: second}
	}
	prompt, err := batchPrompt(judgeSystemPrompt, judgeOutShape, items)
	if err != nil {
		return failAllVotes(out, err)
	}
	text, err := c.runAgent(ctx, prompt, fmt.Sprintf("p3-judge-%d", len(batch)))
	if err != nil {
		return failAllVotes(out, err)
	}
	replies, err := parseBatchArray[cursorJudgeReply](text, len(batch))
	if err != nil {
		if depth < splitRetries && len(batch) > 1 {
			slog.Warn("judge batch failed the integrity gate — splitting", "votes", len(batch), "err", err)
			half := len(batch) / 2
			return append(c.compareBatch(ctx, batch[:half], depth+1), c.compareBatch(ctx, batch[half:], depth+1)...)
		}
		return failAllVotes(out, err)
	}
	for _, r := range replies {
		if r.I < 0 || r.I >= len(out) {
			continue
		}
		out[r.I] = comparisonResult{Vote: voteOf(r.Winner, batch[r.I].ChallengerFirst)}
		if out[r.I].Vote == voteEqual && r.Winner != "equal" {
			out[r.I] = comparisonResult{Err: fmt.Errorf("judge answered %q", r.Winner)}
		}
	}
	return out
}

func failAllVotes(out []comparisonResult, err error) []comparisonResult {
	for i := range out {
		out[i] = comparisonResult{Err: err}
	}
	return out
}
