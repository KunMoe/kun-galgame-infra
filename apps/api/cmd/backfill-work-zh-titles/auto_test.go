package main

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTranslator struct {
	propose map[string]string
	verify  string
	refute  string
	prompts []string
}

func (f *fakeTranslator) Translate(_ context.Context, c mtCandidate, known, mates []string) (string, string, error) {
	f.prompts = append(f.prompts, userMessage(c, known, mates))
	return f.propose[cleanTitle(c.JaTitle)], "fake-model", nil
}

func (f *fakeTranslator) Chat(_ context.Context, system, user string, _ float64) (string, error) {
	f.prompts = append(f.prompts, user)
	switch system {
	case verifySystemPrompt:
		return f.verify, nil
	case refuteSystemPrompt:
		return f.refute, nil
	default: // rederive re-proposes from the same source title
		title := strings.TrimPrefix(strings.SplitN(user, "\n", 2)[0], "作品原名: ")
		return f.propose[title], nil
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	return recs
}

func TestRunAutoAcceptsAndGatesProposals(t *testing.T) {
	clean(t)
	ctx := t.Context()

	good := mkWork(t, "よい")
	mkTitle(t, good, "ja", "そらのおと", 0, 0)
	bad := mkWork(t, "わるい")
	mkTitle(t, bad, "ja", "うみのおと", 0, 0)

	tr := &fakeTranslator{
		propose: map[string]string{"そらのおと": "空之音", "うみのおと": "海のおと"},
		verify:  "通过",
		refute:  "无误",
	}
	out := filepath.Join(t.TempDir(), "auto.csv")
	require.NoError(t, runAuto(ctx, testDB, tr, autoOpts{Out: out, Model: "fake-model"}))

	recs := readCSV(t, out)
	require.Len(t, recs, 3)
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	byWork := map[string][]string{}
	for _, r := range recs[1:] {
		byWork[r[idx["work_id"]]] = r
	}
	g := byWork[itoa(good)]
	assert.Equal(t, string(verdictAccept), g[idx["verdict"]])
	assert.Equal(t, "空之音", g[idx["proposed_zh"]])
	assert.Equal(t, hashSource("そらのおと"), g[idx["src_hash"]])

	b := byWork[itoa(bad)]
	assert.Equal(t, string(verdictKanaLeft), b[idx["verdict"]])
	assert.Empty(t, b[idx["proposed_zh"]], "a gated proposal is never apply-ready")
}

func TestRunAutoFeedsAcceptedSiblingsBackIntoTheBatch(t *testing.T) {
	clean(t)
	ctx := t.Context()

	first := mkWork(t, "ロマンス")
	mkTitle(t, first, "ja", "ロマンス", 0, 0)
	second := mkWork(t, "ロマンス2")
	mkTitle(t, second, "ja", "ロマンス2", 0, 0)
	require.NoError(t, testDB.Exec(`
		INSERT INTO catalog_work_relation (a_work_id, b_work_id, relation_type_id)
		SELECT ?, ?, id FROM catalog_relation_type WHERE key = 'same_series'`, first, second).Error)

	tr := &fakeTranslator{
		propose: map[string]string{"ロマンス": "罗曼史", "ロマンス2": "罗曼史2"},
		verify:  "通过",
		refute:  "无误",
	}
	out := filepath.Join(t.TempDir(), "auto.csv")
	require.NoError(t, runAuto(ctx, testDB, tr, autoOpts{Out: out, Model: "fake-model"}))

	recs := readCSV(t, out)
	require.Len(t, recs, 3)

	joined := strings.Join(tr.prompts, "\n---\n")
	assert.Contains(t, joined, "ロマンス → 罗曼史",
		"the title accepted first must reach the sibling's prompt as series context")
}

func TestRunAutoSkipsUnchangedRows(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "こころ")
	mkTitle(t, w, "ja", "こころ", 0, 0)
	_, err := writeMachineTitle(ctx, testDB, w, "心", hashSource("こころ"), "fake-model")
	require.NoError(t, err)

	tr := &fakeTranslator{propose: map[string]string{"こころ": "心"}, verify: "通过", refute: "无误"}
	out := filepath.Join(t.TempDir(), "auto.csv")
	require.NoError(t, runAuto(ctx, testDB, tr, autoOpts{Out: out, Model: "fake-model"}))

	recs := readCSV(t, out)
	require.Len(t, recs, 2)
	assert.Equal(t, "unchanged", recs[1][7])
	assert.Empty(t, tr.prompts, "an unchanged row must not cost a gateway call")
}

type fakeConverter struct{}

func (fakeConverter) Convert(_ context.Context, s string) (string, error) {
	return strings.NewReplacer("姫", "姬", "戀", "恋").Replace(s), nil
}

func TestRunKanjiEmitsAWorklistAndNeverWrites(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "月姫")
	mkTitle(t, w, "ja", "月姫", 0, 0)
	kana := mkWork(t, "かな")
	mkTitle(t, kana, "ja", "そら", 0, 0)

	out := filepath.Join(t.TempDir(), "kanji.csv")
	require.NoError(t, runKanji(ctx, testDB, fakeConverter{}, out, 0, 0))

	recs := readCSV(t, out)
	require.Len(t, recs, 2, "only the kanji-only work is on the worklist")
	assert.Equal(t, itoa(w), recs[1][0])
	assert.Equal(t, "月姫", recs[1][1])
	assert.Equal(t, "月姬", recs[1][4])
	assert.Len(t, titlesOf(t, w), 1, "the kanji lane never writes the database")
}
