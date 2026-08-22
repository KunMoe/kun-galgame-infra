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

// Cloudflare answers 524 once the origin passes 100s, which a 40-work
// extraction does. Failing the whole batch there costs 40 works at a time, so
// an exhausted retryable status has to reach the split retry like a truncated
// reply does.
func TestHTTPBatchSplitsWhenTheGatewayGivesUp(t *testing.T) {
	orig := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond}
	t.Cleanup(func() { retryBackoff = orig })

	p := fakeChat(t, func(chatReq, int) (string, string) { return `[{"i":0,"摘录":{}}]`, "stop" })
	p.status = func(n int) int {
		if n <= len(retryBackoff)+1 {
			return 524 // the first call and every one of its retries
		}
		return http.StatusOK
	}

	out := p.ex.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	for i, e := range out {
		require.NoError(t, e.Err, "work %d survived on a half-sized call", i)
	}
}
