package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuto struct {
	translateOut string
	translateErr error
	verifyOut    string
	refuteOut    string
	rederiveOut  string
	calls        []string
}

func (f *fakeAuto) Translate(_ context.Context, _ mtResidueCandidate) (string, string, error) {
	f.calls = append(f.calls, "translate")
	return f.translateOut, "glm-5.2", f.translateErr
}

func (f *fakeAuto) chat(_ context.Context, system, _ string, _ float64) (string, error) {
	switch system {
	case verifySystemPrompt:
		f.calls = append(f.calls, "verify")
		return f.verifyOut, nil
	case refuteSystemPrompt:
		f.calls = append(f.calls, "refute")
		return f.refuteOut, nil
	case rederiveSystemPrompt:
		f.calls = append(f.calls, "rederive")
		return f.rederiveOut, nil
	}
	return "", fmt.Errorf("unknown system prompt")
}

var autoCand = mtResidueCandidate{ID: 7, DisplayName: "アリス", Latin: "Alice", Uses: 3, Works: "w1"}

func autoRow(t *testing.T, f *fakeAuto) (map[string]string, *autoStats) {
	t.Helper()
	st := &autoStats{}
	rec := judgeOne(context.Background(), f, "glm-5.2", autoCand, 0, st)
	require.Len(t, rec, len(autoCSVHeader))
	out := make(map[string]string, len(rec))
	for i, col := range autoCSVHeader {
		out[col] = rec[i]
	}
	return out, st
}

func TestJudgeOneUnanimousAccepts(t *testing.T) {
	f := &fakeAuto{translateOut: "爱丽丝", verifyOut: "通过", refuteOut: "无误", rederiveOut: "爱丽丝"}
	row, st := autoRow(t, f)
	assert.Equal(t, "accept", row["verdict"])
	assert.Equal(t, "爱丽丝", row["proposed_zh"])
	assert.Equal(t, []string{"translate", "verify", "refute", "rederive"}, f.calls)
	assert.Equal(t, 1, st.Accepted)
}

func TestJudgeOneRederiveToleratesSeparatorVariants(t *testing.T) {
	f := &fakeAuto{translateOut: "爱丽丝·玛格", verifyOut: "通过", refuteOut: "无误", rederiveOut: "爱丽丝・玛格"}
	row, _ := autoRow(t, f)
	assert.Equal(t, "accept", row["verdict"])
}

func TestJudgeOneVerifyFailShortCircuits(t *testing.T) {
	f := &fakeAuto{translateOut: "爱丽丝", verifyOut: "不通过", refuteOut: "无误", rederiveOut: "爱丽丝"}
	row, st := autoRow(t, f)
	assert.Equal(t, "reject_verify", row["verdict"])
	assert.Empty(t, row["proposed_zh"], "a rejected proposal must not reach --apply-csv")
	assert.Equal(t, "爱丽丝", row["candidate_zh"], "the audit trail keeps the proposal")
	assert.Equal(t, []string{"translate", "verify"}, f.calls, "later framings are not called")
	assert.Equal(t, 1, st.Verify)
}

func TestJudgeOneRefuteFailRejects(t *testing.T) {
	f := &fakeAuto{translateOut: "爱丽丝", verifyOut: "通过", refuteOut: "音译不通行,通行写法是艾莉丝", rederiveOut: "爱丽丝"}
	row, _ := autoRow(t, f)
	assert.Equal(t, "reject_refute", row["verdict"])
	assert.Empty(t, row["proposed_zh"])
	assert.NotEmpty(t, row["detail"])
}

func TestJudgeOneRederiveMismatchRejects(t *testing.T) {
	f := &fakeAuto{translateOut: "爱丽丝", verifyOut: "通过", refuteOut: "无误", rederiveOut: "艾莉丝"}
	row, st := autoRow(t, f)
	assert.Equal(t, "reject_rederive", row["verdict"])
	assert.Empty(t, row["proposed_zh"])
	assert.Equal(t, "艾莉丝", row["rederived_zh"])
	assert.Equal(t, 1, st.Rederive)
}

func TestJudgeOnePreGatesSkipTheJudges(t *testing.T) {
	kana := &fakeAuto{translateOut: "爱丽丝ちゃん"}
	row, st := autoRow(t, kana)
	assert.Equal(t, "reject_kana_left", row["verdict"])
	assert.Equal(t, []string{"translate"}, kana.calls)
	assert.Equal(t, 1, st.Kana)

	skip := &fakeAuto{translateOut: "SKIP"}
	row, st = autoRow(t, skip)
	assert.Equal(t, "skip_model", row["verdict"])
	assert.Equal(t, 1, st.Skips)
}

func TestAutoCSVFeedsApplyWithOnlyAcceptedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auto.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	w := csv.NewWriter(f)
	require.NoError(t, w.Write(autoCSVHeader))
	st := &autoStats{}
	ok := &fakeAuto{translateOut: "爱丽丝", verifyOut: "通过", refuteOut: "无误", rederiveOut: "爱丽丝"}
	require.NoError(t, w.Write(judgeOne(context.Background(), ok, "glm-5.2", autoCand, 0, st)))
	bad := &fakeAuto{translateOut: "爱丽丝", verifyOut: "不通过"}
	require.NoError(t, w.Write(judgeOne(context.Background(), bad, "glm-5.2", autoCand, 0, st)))
	w.Flush()
	require.NoError(t, f.Close())

	rows, err := readReviewedCSV(path)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the accepted row survives into the apply path")
	assert.Equal(t, "爱丽丝", rows[0].Zh)
	assert.Equal(t, "glm-5.2", rows[0].Model)
}

func TestContainsKana(t *testing.T) {
	assert.True(t, containsKana("爱丽丝ちゃん"))
	assert.True(t, containsKana("ミク"))
	assert.True(t, containsKana("雪风ー"), "long-vowel mark is katakana block")
	assert.True(t, containsKana("ｱﾘｽ"), "halfwidth katakana")
	assert.False(t, containsKana("爱丽丝"))
	assert.False(t, containsKana("Alice·2号"))
}

func TestCleanProposalNormalizesKatakanaDots(t *testing.T) {
	// Rehearsal incident: a fully translated "双叶・莉莉・拉姆塞斯" was rejected by
	// the kana-left pre-gate because ・(U+30FB) is inside the katakana block.
	got := cleanProposal("双叶・莉莉・拉姆塞斯")
	assert.Equal(t, "双叶·莉莉·拉姆塞斯", got)
	assert.False(t, containsKana(got))
	assert.Equal(t, "白菊·A", cleanProposal("白菊･A"))
}

func TestParseShard(t *testing.T) {
	i, n, err := parseShard("3/8")
	require.NoError(t, err)
	assert.Equal(t, 3, i)
	assert.Equal(t, 8, n)
	i, n, err = parseShard("")
	require.NoError(t, err)
	assert.Zero(t, i)
	assert.Zero(t, n)
	for _, bad := range []string{"8/8", "-1/8", "abc", "3", "3/0"} {
		_, _, err := parseShard(bad)
		assert.Error(t, err, bad)
	}
}

func TestFirstLine(t *testing.T) {
	assert.Equal(t, "通过", firstLine(" 「通过」。\n理由是…"))
	assert.Equal(t, "无误", firstLine("无误"))
	assert.NotEqual(t, "通过", firstLine("不通过:音译生僻"), "a qualified answer never passes the gate")
}

func TestChatModelRerollsOnLengthFinish(t *testing.T) {
	// The 08-15 ramp incident: length finishes are stochastic reasoning
	// death-spirals, so a fresh identical request usually succeeds. The tool
	// must re-roll instead of failing the character on the first spiral.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		finish := "length"
		content := ""
		if calls == 2 {
			finish = "stop"
			content = "琪露诺"
		}
		fmt.Fprintf(w, `{"model":"m","choices":[{"message":{"content":%q},"finish_reason":%q}]}`, content, finish)
	}))
	defer srv.Close()

	tr := newHTTPTranslator(srv.URL, "tok", "m", 4096)
	got, model, err := tr.chatModel(context.Background(), "sys", "user", 0)
	require.NoError(t, err)
	assert.Equal(t, "琪露诺", got)
	assert.Equal(t, "m", model)
	assert.Equal(t, 2, calls)
}

func TestChatModelGivesUpAfterThreeLengthFinishes(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"model":"m","choices":[{"message":{"content":""},"finish_reason":"length"}]}`)
	}))
	defer srv.Close()

	tr := newHTTPTranslator(srv.URL, "tok", "m", 4096)
	_, _, err := tr.chatModel(context.Background(), "sys", "user", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `finish_reason="length"`)
	assert.Equal(t, 3, calls)
}
