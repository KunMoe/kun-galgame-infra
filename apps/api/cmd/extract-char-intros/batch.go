package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// The batched lane, shared by both gateways. On Cursor every agent run re-reads
// ~14k tokens of fixed preamble; on the OpenAI-compatible gateway the account
// serialises requests (2026-08-22: four concurrent judge calls came back as a
// perfect serial staircase 6.8/13.9/20.0/27.1s, eight returned a 429
// "Concurrency limit exceeded for account"). So on BOTH gateways the batch is
// the throughput lever and concurrency is not: batching the judge measured
// 0.147 -> 3.09 comparisons per second.
type batchCall struct {
	Rules string // system half: the task rules plus the envelope contract
	Items string // user half: a JSON array, one object per item
	Label string // names the Cursor agent run; unused by the chat face
}

type promptCaller interface {
	// callBatch answers one batched prompt with the reply text and the model
	// that actually served it.
	callBatch(ctx context.Context, c batchCall) (text, model string, err error)
}

// errBatchOversize marks a call that a SMALLER call might have survived: the
// reply was cut off at max_tokens, or the gateway ran out of patience with one
// that took too long. Only these reach the split retry; every other failure
// fails its batch outright.
var errBatchOversize = errors.New("batch too big for one call")

const batchEnvelopeRules = `下面是一个 JSON 数组,每项是一个待处理条目,` + "`i`" + ` 是它的序号。
逐项独立处理,输出 JSON 数组 %s。
必须满足:输出项数与输入完全一致;每项的 ` + "`i`" + ` 原样回显;顺序与输入一致。
把 JSON 数组直接作为你的最终回复正文输出:不要解释、不要代码围栏、不要写入任何文件、不要使用任何工具。`

func batchCallOf(rules, outShape, label string, items any) (batchCall, error) {
	raw, err := json.Marshal(items)
	if err != nil {
		return batchCall{}, err
	}
	return batchCall{
		Rules: rules + "\n\n" + fmt.Sprintf(batchEnvelopeRules, outShape),
		Items: string(raw),
		Label: label,
	}, nil
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

// splitRetries is how many times a batch that fails the integrity gate is cut
// in half and retried. One level is what wave 210 settled on: it rescues the
// batch that was merely too big without letting a systematically bad item turn
// one run into a binary search.
const splitRetries = 1

func splittable(depth, size int) bool { return depth < splitRetries && size > 1 }

// --- extraction ---

type batchExtractItem struct {
	I      int      `json:"i"`
	Roster []string `json:"角色名单"`
	Intro  string   `json:"作品简介"`
}

type batchExtractReply struct {
	I     int               `json:"i"`
	Found map[string]string `json:"摘录"`
}

const extractOutShape = `[{"i":0,"摘录":{"角色名":"摘录到的介绍文本"}}]`

const extractBatchRules = extractSystemPrompt + `
5. ` + "`摘录`" + ` 的键必须逐字使用「角色名单」里给出的写法(带中文名括注的,只用括注前的名字);没有可摘录的角色介绍时该项输出空对象 {}。
6. 摘录值必须是从简介正文里**原样复制**的连续文本:一个字都不能改、不能顺句、不能换标点、不能把相隔的句子拼起来。程序会逐字比对,改写过的一律丢弃——与其润色不如原样照抄。`

func extractBatchVia(ctx context.Context, c promptCaller, batch []candidateWork, depth int) []extraction {
	out := make([]extraction, len(batch))
	items := make([]batchExtractItem, len(batch))
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
		items[i] = batchExtractItem{I: i, Roster: names, Intro: w.Intro}
	}
	call, err := batchCallOf(extractBatchRules, extractOutShape, fmt.Sprintf("p3-extract-%d", batch[0].WorkID), items)
	if err != nil {
		return failAll(out, err)
	}
	text, model, err := c.callBatch(ctx, call)
	if err != nil && !errors.Is(err, errBatchOversize) {
		return failAll(out, err)
	}
	replies, gateErr := parseBatchArray[batchExtractReply](text, len(batch))
	if gateErr == nil {
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
			out[r.I] = extraction{Found: found, Model: model}
		}
		return out
	}
	if err == nil {
		err = gateErr
	}
	if splittable(depth, len(batch)) {
		slog.Warn("extraction batch failed the integrity gate — splitting", "works", len(batch), "err", err)
		half := len(batch) / 2
		return append(extractBatchVia(ctx, c, batch[:half], depth+1), extractBatchVia(ctx, c, batch[half:], depth+1)...)
	}
	return failAll(out, err)
}

func failAll(out []extraction, err error) []extraction {
	for i := range out {
		out[i] = extraction{Err: err}
	}
	return out
}

// --- panel judging ---

type batchJudgeItem struct {
	I    int    `json:"i"`
	Name string `json:"角色"`
	A    string `json:"A"`
	B    string `json:"B"`
}

type batchJudgeReply struct {
	I      int    `json:"i"`
	Winner string `json:"winner"`
}

const judgeOutShape = `[{"i":0,"winner":"A"}]`

func compareBatchVia(ctx context.Context, c promptCaller, batch []comparison, depth int) []comparisonResult {
	out := make([]comparisonResult, len(batch))
	items := make([]batchJudgeItem, len(batch))
	for i, cmp := range batch {
		first, second := cmp.Incumbent, cmp.Challenger
		if cmp.ChallengerFirst {
			first, second = cmp.Challenger, cmp.Incumbent
		}
		items[i] = batchJudgeItem{I: i, Name: cmp.Name, A: first, B: second}
	}
	call, err := batchCallOf(judgeSystemPrompt, judgeOutShape, fmt.Sprintf("p3-judge-%d", len(batch)), items)
	if err != nil {
		return failAllVotes(out, err)
	}
	text, _, err := c.callBatch(ctx, call)
	if err != nil && !errors.Is(err, errBatchOversize) {
		return failAllVotes(out, err)
	}
	replies, gateErr := parseBatchArray[batchJudgeReply](text, len(batch))
	if gateErr == nil {
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
	if err == nil {
		err = gateErr
	}
	if splittable(depth, len(batch)) {
		slog.Warn("judge batch failed the integrity gate — splitting", "votes", len(batch), "err", err)
		half := len(batch) / 2
		return append(compareBatchVia(ctx, c, batch[:half], depth+1), compareBatchVia(ctx, c, batch[half:], depth+1)...)
	}
	return failAllVotes(out, err)
}

func failAllVotes(out []comparisonResult, err error) []comparisonResult {
	for i := range out {
		out[i] = comparisonResult{Err: err}
	}
	return out
}
