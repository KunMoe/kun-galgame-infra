package intromt

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollapseFiresOnLostBodyNotOnShortIntros(t *testing.T) {
	long := strings.Repeat("あらすじの本文。", 20)

	assert.True(t, collapsed(long, "略"),
		"a long source rendered as two characters lost its body")
	assert.False(t, collapsed(long, strings.Repeat("正文。", 10)),
		"a real translation of a long source is not a collapse")
	assert.False(t, collapsed("短いあらすじ。", "简短的简介。"),
		"a short source has nowhere to collapse from — 529 live rows are this shape")
}

func TestACollapsedTranslationKeepsThePreviousRow(t *testing.T) {
	clean(t)
	ctx := context.Background()
	medium, _, bangumi := reg(t)

	w := mkWork(t, medium, "collapse-me", nil)
	mkIntro(t, w, "ja", strings.Repeat("これは本編のあらすじです。", 12), bangumi)

	good := &fakeTranslator{model: "mt", fn: func(string) string {
		return strings.Repeat("这是正文的简介。", 12)
	}}
	st, err := Run(ctx, good, Opts{DSN: testDSN, Apply: true})
	require.NoError(t, err)
	require.Equal(t, 1, st.Inserted)

	empty := &fakeTranslator{model: "mt", fn: func(string) string { return "（省略）" }}
	st, err = Run(ctx, empty, Opts{DSN: testDSN, Apply: true, Force: true})
	require.NoError(t, err)
	assert.Equal(t, 1, st.Collapsed)
	assert.Zero(t, st.Retranslated, "a collapse must not count as a rewrite")

	var zh string
	require.NoError(t, testDB.Raw(
		`SELECT intro FROM catalog_work_intro WHERE work_id = ? AND lang = 'zh-Hans' AND provenance = 1`,
		w).Scan(&zh).Error)
	assert.Equal(t, strings.Repeat("这是正文的简介。", 12), zh,
		"the previous translation survives, so the work comes back as a candidate")
}
