package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBatchArrayIntegrityGate(t *testing.T) {
	type reply struct {
		I      int    `json:"i"`
		Winner string `json:"winner"`
	}
	cases := []struct {
		name    string
		text    string
		want    int
		wantErr string
	}{
		{"clean", `[{"i":0,"winner":"A"},{"i":1,"winner":"B"}]`, 2, ""},
		{"tolerates surrounding chatter", "好的:\n[{\"i\":0,\"winner\":\"A\"}]\n", 1, ""},
		{"dropped tail", `[{"i":0,"winner":"A"}]`, 2, "count mismatch"},
		{"duplicate index hides a gap", `[{"i":0,"winner":"A"},{"i":0,"winner":"B"}]`, 2, "index set mismatch"},
		{"missing index key", `[{"winner":"A"},{"i":1,"winner":"B"}]`, 2, "item missing index key"},
		{"no array at all", `抱歉,我无法完成。`, 1, "no JSON array"},
	}
	for _, c := range cases {
		got, err := parseBatchArray[reply](c.text, c.want)
		if c.wantErr != "" {
			require.Error(t, err, c.name)
			assert.Contains(t, err.Error(), c.wantErr, c.name)
			continue
		}
		require.NoError(t, err, c.name)
		assert.Len(t, got, c.want, c.name)
	}
}

func TestBatchCallCarriesTheIntegrityRules(t *testing.T) {
	c, err := batchCallOf("规则", `[{"i":0}]`, "label", []batchExtractItem{{I: 0, Intro: "简介"}})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(c.Rules, "规则"))
	assert.Contains(t, c.Rules, "输出项数与输入完全一致")
	assert.Contains(t, c.Items, `"作品简介":"简介"`)
	assert.Equal(t, "label", c.Label)
}

// chatReq is the request shape the batched lane must send: the rules as the
// system message, the JSON array as the user message.
type chatReq struct {
	Model           string `json:"model"`
	MaxTokens       int    `json:"max_tokens"`
	ReasoningEffort string `json:"reasoning_effort"`
	Messages        []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type chatProbe struct {
	ex     *httpExtractor
	reqs   []chatReq
	status func(n int) int // HTTP status for the nth call; nil means 200
}

func fakeChat(t *testing.T, answer func(req chatReq, n int) (content, finish string)) *chatProbe {
	t.Helper()
	p := &chatProbe{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		require.NoError(t, json.Unmarshal(body, &req))
		p.reqs = append(p.reqs, req)
		if p.status != nil {
			if code := p.status(len(p.reqs)); code != http.StatusOK {
				http.Error(w, "error code: "+http.StatusText(code), code)
				return
			}
		}
		content, finish := answer(req, len(p.reqs))
		out, _ := json.Marshal(map[string]any{
			"model": "grok-4.6",
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}, "finish_reason": finish},
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}))
	t.Cleanup(srv.Close)
	p.ex = newHTTPExtractor(srv.URL+"/v1", "test-key", "grok-4.6", "low", 4096)
	return p
}

func TestHTTPCompareBatchSendsOneCallAndKeepsEachSide(t *testing.T) {
	p := fakeChat(t, func(chatReq, int) (string, string) {
		return `[{"i":0,"winner":"B"},{"i":1,"winner":"A"}]`, "stop"
	})
	batch := []comparison{
		{Name: "沙耶", Incumbent: "既有甲", Challenger: "提取甲", ChallengerFirst: false},
		{Name: "玲", Incumbent: "既有乙", Challenger: "提取乙", ChallengerFirst: true},
	}

	out := p.ex.CompareBatch(context.Background(), batch)
	require.Len(t, out, 2)
	require.Len(t, p.reqs, 1, "a batch of two comparisons is one call")
	for i, r := range out {
		require.NoError(t, r.Err, "vote %d", i)
		assert.Equal(t, voteChallenger, r.Vote, "vote %d answered for the challenger whichever side it sat on", i)
	}
	msgs := p.reqs[0].Messages
	require.Len(t, msgs, 2)
	assert.Equal(t, "system", msgs[0].Role)
	assert.Contains(t, msgs[0].Content, "输出项数与输入完全一致")
	assert.Equal(t, "user", msgs[1].Role)
	assert.Contains(t, msgs[1].Content, `"角色":"沙耶"`)
	assert.Equal(t, "low", p.reqs[0].ReasoningEffort)
}

func TestHTTPExtractBatchMapsAnswersBackToTheirWork(t *testing.T) {
	p := fakeChat(t, func(chatReq, int) (string, string) {
		return `[{"i":0,"摘录":{"沙耶":"甲的介绍"}},{"i":1,"摘录":{}}]`, "stop"
	})

	out := p.ex.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	require.NoError(t, out[0].Err)
	assert.Equal(t, map[string]string{"沙耶": "甲的介绍"}, out[0].Found)
	assert.Equal(t, "grok-4.6", out[0].Model, "the served model is recorded, not the requested one")
	require.NoError(t, out[1].Err)
	assert.Empty(t, out[1].Found)
	require.Len(t, p.reqs, 1)
}

// A reply the gateway cut off at max_tokens is the one call failure halving can
// fix, so it must reach the split retry instead of failing both works outright.
func TestHTTPBatchSplitsAReplyCutOffAtMaxTokens(t *testing.T) {
	p := fakeChat(t, func(_ chatReq, n int) (string, string) {
		if n == 1 {
			return `[{"i":0,"摘录":{"沙耶":"甲的介绍"`, "length"
		}
		return `[{"i":0,"摘录":{}}]`, "stop"
	})

	out := p.ex.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	assert.Len(t, p.reqs, 3, "the cut-off batch plus its two halves")
	for i, e := range out {
		require.NoError(t, e.Err, "work %d", i)
	}
}

func TestHTTPBatchFailsEveryItemWhenSplittingCannotHelp(t *testing.T) {
	p := fakeChat(t, func(chatReq, int) (string, string) {
		return "抱歉,我无法完成这个任务。", "stop"
	})

	out := p.ex.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	assert.Len(t, p.reqs, 3)
	for i, e := range out {
		require.Error(t, e.Err, "work %d", i)
		assert.Contains(t, e.Err.Error(), "no JSON array")
	}
}

func TestHTTPBatchOmitsEffortWhenUnset(t *testing.T) {
	p := fakeChat(t, func(chatReq, int) (string, string) { return `[{"i":0,"摘录":{}}]`, "stop" })
	p.ex.effort = ""

	out := p.ex.ExtractBatch(context.Background(), twoWorks()[:1])
	require.NoError(t, out[0].Err)
	assert.Empty(t, p.reqs[0].ReasoningEffort, fmt.Sprintf("req: %+v", p.reqs[0]))
}

// Cloudflare answers 524 once the origin passes its 100s cap, so retrying the
// same call just spends another 100s reaching the same cap. The 2026-08-23
// panel run lost 42 minutes to one 25-work batch doing exactly that, and then
// got both halves on the first try.
func TestHTTPBatchSplitsOnA524WithoutRetrying(t *testing.T) {
	orig := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond}
	t.Cleanup(func() { retryBackoff = orig })

	p := fakeChat(t, func(chatReq, int) (string, string) { return `[{"i":0,"摘录":{}}]`, "stop" })
	// 524 is what a call that runs long gets, so the probe answers it by size:
	// the whole batch times out, either half gets through.
	p.status = func(n int) int {
		if strings.Count(p.reqs[n-1].Messages[1].Content, `"i":`) > 1 {
			return 524
		}
		return http.StatusOK
	}

	out := p.ex.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	for i, e := range out {
		require.NoError(t, e.Err, "work %d survived on a half-sized call", i)
	}
	require.Len(t, p.reqs, 3, "the oversized call must go straight to the split, not up the ladder")
}

func fourWorks() []candidateWork {
	return []candidateWork{
		{WorkID: 11, Intro: "甲作品简介", Roster: []rosterChar{{CharacterID: 1, Name: "沙耶"}}},
		{WorkID: 22, Intro: "乙作品简介", Roster: []rosterChar{{CharacterID: 2, Name: "玲"}}},
		{WorkID: 33, Intro: "丙作品简介", Roster: []rosterChar{{CharacterID: 3, Name: "葵"}}},
		{WorkID: 44, Intro: "丁作品简介", Roster: []rosterChar{{CharacterID: 4, Name: "樒"}}},
	}
}

// oversizeOnlyOK answers 524 to any call carrying more than one item, so a
// batch only survives once it has been cut all the way down to 1.
func oversizeOnlyOK(p *chatProbe) func(int) int {
	return func(n int) int {
		if strings.Count(p.reqs[n-1].Messages[1].Content, `"i":`) > 1 {
			return 524
		}
		return http.StatusOK
	}
}

// One cut is not always enough. The 2026-08-23 panel run measured both ends of
// this: at --batch 10 the single allowed cut landed on 5 and cleared the 100s
// cap, but at --batch 25 it landed on 12, still over the cap, and the batch died
// one cut short of working. Oversize is the failure halving converges on, so it
// keeps halving: 4 -> 2 -> 1 is 1 + 2 + 4 = 7 calls.
func TestHTTPBatchKeepsHalvingWhileTheCauseIsOversize(t *testing.T) {
	orig := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond}
	t.Cleanup(func() { retryBackoff = orig })

	p := fakeChat(t, func(chatReq, int) (string, string) { return `[{"i":0,"摘录":{}}]`, "stop" })
	p.status = oversizeOnlyOK(p)

	out := p.ex.ExtractBatch(context.Background(), fourWorks())
	require.Len(t, out, 4)
	for i, e := range out {
		require.NoError(t, e.Err, "work %d survived once the batch reached size 1", i)
	}
	assert.Len(t, p.reqs, 7, "4 -> two 2s -> four 1s")
}

// The negative control for the rule above: a reply of the wrong shape means one
// item is systematically bad, and deeper cuts would only binary-search for it.
// That cause still stops after a single level — 4 plus its two halves.
func TestHTTPBatchStopsAtOneCutWhenTheShapeIsWrong(t *testing.T) {
	p := fakeChat(t, func(chatReq, int) (string, string) {
		return "抱歉,我无法完成这个任务。", "stop"
	})

	out := p.ex.ExtractBatch(context.Background(), fourWorks())
	require.Len(t, out, 4)
	for i, e := range out {
		require.Error(t, e.Err, "work %d", i)
	}
	assert.Len(t, p.reqs, 3, "one cut only; a bad item must not turn the run into a binary search")
}
