package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCursor serves the two routes the lane uses: create an agent, then poll
// its run. The first poll of each run answers RUNNING so the wait loop is
// exercised. answer receives the prompt and the 1-based run number, so a test
// can make the first (big) batch fail and the split halves succeed.
type cursorProbe struct {
	client  *cursorClient
	prompts []string
}

func fakeCursor(t *testing.T, answer func(prompt string, run int) string) *cursorProbe {
	t.Helper()
	p := &cursorProbe{}
	results := map[string]string{}
	polled := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/agents":
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Prompt struct {
					Text string `json:"text"`
				} `json:"prompt"`
				Model struct {
					ID string `json:"id"`
				} `json:"model"`
			}
			require.NoError(t, json.Unmarshal(body, &req))
			assert.Equal(t, "grok-4.6", req.Model.ID)
			p.prompts = append(p.prompts, req.Prompt.Text)
			id := fmt.Sprintf("run_%d", len(p.prompts))
			results[id] = answer(req.Prompt.Text, len(p.prompts))
			fmt.Fprintf(w, `{"id":"ag_1","run":{"id":%q,"status":"RUNNING"}}`, id)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/agents/ag_1/runs/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/agents/ag_1/runs/")
			if !polled[id] {
				polled[id] = true
				fmt.Fprintf(w, `{"run":{"id":%q,"status":"RUNNING"}}`, id)
				return
			}
			out, _ := json.Marshal(map[string]any{"run": map[string]any{
				"id": id, "status": "FINISHED", "result": results[id],
			}})
			w.Write(out)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	p.client = newCursorClient("test-key", "grok-4.6", "low", 20*time.Second)
	p.client.base = srv.URL
	p.client.poll = time.Millisecond
	return p
}

func always(result string) func(string, int) string {
	return func(string, int) string { return result }
}

func twoWorks() []candidateWork {
	return []candidateWork{
		{WorkID: 11, Intro: "甲作品简介", Roster: []rosterChar{{CharacterID: 1, Name: "沙耶", ZhName: "沙耶"}}},
		{WorkID: 22, Intro: "乙作品简介", Roster: []rosterChar{{CharacterID: 2, Name: "玲"}}},
	}
}

func TestCursorExtractBatchMapsAnswersBackToTheirWork(t *testing.T) {
	p := fakeCursor(t, always(`[{"i":0,"摘录":{"沙耶":"甲的介绍"}},{"i":1,"摘录":{}}]`))

	out := p.client.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	require.NoError(t, out[0].Err)
	assert.Equal(t, map[string]string{"沙耶": "甲的介绍"}, out[0].Found)
	assert.Equal(t, "grok-4.6", out[0].Model)
	require.NoError(t, out[1].Err)
	assert.Empty(t, out[1].Found)
	require.Len(t, p.prompts, 1, "one batch is one agent run")
	assert.Contains(t, p.prompts[0], "甲作品简介")
	assert.Contains(t, p.prompts[0], "乙作品简介")
}

func TestCursorExtractBatchAcceptsTheAnnotatedRosterLabel(t *testing.T) {
	p := fakeCursor(t, always(`[{"i":0,"摘录":{"沙耶(中文名: 纱耶)":"甲的介绍"}}]`))
	batch := []candidateWork{
		{WorkID: 11, Intro: "甲作品简介", Roster: []rosterChar{{CharacterID: 1, Name: "沙耶", ZhName: "纱耶"}}},
	}

	out := p.client.ExtractBatch(context.Background(), batch)
	require.NoError(t, out[0].Err)
	assert.Equal(t, map[string]string{"沙耶": "甲的介绍"}, out[0].Found,
		"an echoed label must land on the roster name, not in the unmatched bucket")
}

// A reply that drops the tail must never be trusted by position — that files
// every passage onto the wrong work. The batch is cut in half and retried, and
// only what still fails is reported as failed.
func TestCursorExtractBatchSplitsAShortReply(t *testing.T) {
	p := fakeCursor(t, func(_ string, run int) string {
		if run == 1 {
			return `[{"i":0,"摘录":{"沙耶":"甲的介绍"}}]` // one item for a batch of two
		}
		return `[{"i":0,"摘录":{"沙耶":"甲的介绍"}}]`
	})

	out := p.client.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	assert.Len(t, p.prompts, 3, "the failed batch plus its two halves")
	require.NoError(t, out[0].Err)
	assert.Equal(t, map[string]string{"沙耶": "甲的介绍"}, out[0].Found)
	require.NoError(t, out[1].Err, "the second half answers for its own single work")
}

func TestCursorExtractBatchFailsEveryWorkWhenSplittingCannotHelp(t *testing.T) {
	p := fakeCursor(t, always(`抱歉,我无法完成这个任务。`))

	out := p.client.ExtractBatch(context.Background(), twoWorks())
	require.Len(t, out, 2)
	assert.Len(t, p.prompts, 3)
	for i, e := range out {
		require.Error(t, e.Err, "item %d", i)
		assert.Contains(t, e.Err.Error(), "no JSON array")
	}
}

type countingJudge struct{ batches []int }

func (j *countingJudge) CompareBatch(_ context.Context, batch []comparison) []comparisonResult {
	j.batches = append(j.batches, len(batch))
	out := make([]comparisonResult, len(batch))
	for i, c := range batch {
		out[i] = comparisonResult{Vote: voteOf("A", c.ChallengerFirst)}
	}
	return out
}

func TestVoteDispatchChunksAndKeepsOrder(t *testing.T) {
	j := &countingJudge{}
	cmps := make([]comparison, 5)
	for i := range cmps {
		cmps[i] = comparison{Name: fmt.Sprint(i), ChallengerFirst: i%2 == 1}
	}

	out := voteDispatch(j, 2, 1, 0)(context.Background(), cmps)
	require.Len(t, out, 5)
	assert.Equal(t, []int{2, 2, 1}, j.batches)
	for i, r := range out {
		require.NoError(t, r.Err)
		want := voteIncumbent
		if i%2 == 1 {
			want = voteChallenger
		}
		assert.Equal(t, want, r.Vote, "vote %d kept its own position", i)
	}
}

type shortJudge struct{}

func (shortJudge) CompareBatch(_ context.Context, _ []comparison) []comparisonResult {
	return nil
}

func TestResolvePanelKeepsIncumbentWhenARoundFails(t *testing.T) {
	w := &writer{st: &stats{}, judge: shortJudge{}}
	contested := []gated{{WorkID: 1, Target: rosterChar{CharacterID: 1, Name: "沙耶", Incumbent: "既有"}, Passage: "提取"}}

	adopted := w.resolvePanel(context.Background(), contested, voteDispatch(shortJudge{}, 10, 1, 0))
	assert.Empty(t, adopted)
	assert.Equal(t, 1, w.st.PanelErrors)
	assert.Zero(t, w.st.PanelKept, "an unjudged character is not a kept verdict — it is retryable")
}

func TestCursorModelFallsBackOffTheOpenAIDefault(t *testing.T) {
	assert.Equal(t, "grok-4.6", cursorModel("glm-5.2"))
	assert.Equal(t, "grok-4.6", cursorModel(""))
	assert.Equal(t, "grok-4.7", cursorModel("grok-4.7"))
}

func TestChunkWorksCoversEveryWorkOnce(t *testing.T) {
	works := make([]candidateWork, 7)
	for i := range works {
		works[i] = candidateWork{WorkID: int64(i)}
	}
	var seen []int64
	for _, c := range chunkWorks(works, 3) {
		for _, w := range c {
			seen = append(seen, w.WorkID)
		}
	}
	assert.Equal(t, []int64{0, 1, 2, 3, 4, 5, 6}, seen)
}

func TestReadCursorKeyRejectsAnEmptyFile(t *testing.T) {
	t.Setenv("CURSOR_API_KEY", "")
	path := t.TempDir() + "/key"
	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))
	_, err := readCursorKey(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	require.NoError(t, os.WriteFile(path, []byte("crsr_secret\n"), 0o600))
	key, err := readCursorKey(path)
	require.NoError(t, err)
	assert.Equal(t, "crsr_secret", key)
}
