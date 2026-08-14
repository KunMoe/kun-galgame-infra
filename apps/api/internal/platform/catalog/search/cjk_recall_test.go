package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeQueryOperators(t *testing.T) {
	cases := map[string]string{
		"アヘ顔アクメ中毒 －人体改造で狂ってイク私を見ないで－": "アヘ顔アクメ中毒  人体改造で狂ってイク私を見ないで",
		"－人体改造":                "人体改造",
		"＂CLANNAD＂":            "CLANNAD",
		"CRAZY CHA!N -エルピスの鎖-": "CRAZY CHA!N  エルピスの鎖",
		`"CLANNAD"`:            "CLANNAD",
		"色仕掛け学園～思春期男子誘惑作戦～": "色仕掛け学園～思春期男子誘惑作戦～",
		"ソードアート":     "ソードアート",
		"":           "",
		"  spaced  ": "spaced",
	}
	for in, want := range cases {
		assert.Equal(t, want, sanitizeQuery(in), "in=%q", in)
	}
}

func TestLocalesForUIPairsWithTheIndex(t *testing.T) {
	assert.Equal(t, []string{"cmn"}, LocalesForUI(IndexCharacters, "zh"))
	assert.Equal(t, []string{"jpn"}, LocalesForUI(IndexCreditNames, "ja"))
	assert.Nil(t, LocalesForUI(IndexLabels, "en"))
	assert.Nil(t, LocalesForUI(IndexWorks, "zh"))
	assert.Nil(t, LocalesForUI(IndexWorks, "ja"))
}

func TestEnsureIndexesResetsWorksLocalePins(t *testing.T) {
	require.NoError(t, EnsureIndexes(testClient))
	t.Cleanup(func() { _, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexWorks)) })

	task, err := testClient.Index(IndexWorks).UpdateLocalizedAttributes(localizedAttributes())
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)
	got, err := testClient.Index(IndexWorks).GetLocalizedAttributes()
	require.NoError(t, err)
	require.NotEmpty(t, got, "precondition: the stale pins are in place")

	require.NoError(t, EnsureIndexes(testClient))
	got, err = testClient.Index(IndexWorks).GetLocalizedAttributes()
	require.NoError(t, err)
	assert.Empty(t, got, "EnsureIndexes must clear pins it no longer declares")
}

func TestWorksEqualsDelimiterTitleRecall(t *testing.T) {
	require.NoError(t, EnsureIndexes(testClient))
	t.Cleanup(func() { _, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexWorks)) })

	docs := []EntityDoc{
		{ID: "w1", EntityType: "work", NameJa: "ココロネ＝ペンデュラム！"},
		{ID: "w2", EntityType: "work", NameJa: "ココロネ=ペンデュラム！"},
	}
	task, err := testClient.Index(IndexWorks).AddDocuments(docs, nil)
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)

	idx := NewIndexer(testClient)
	for _, tc := range []struct{ name, q, want string }{
		{"left of fullwidth equals", "ココロネ", "w1"},
		{"right of fullwidth equals", "ペンデュラム", "w1"},
		{"left of ascii equals", "ココロネ", "w2"},
		{"right of ascii equals", "ペンデュラム", "w2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := idx.SearchWorks(t.Context(), WorksQuery{Q: tc.q, Limit: 12})
			require.NoError(t, err)
			id, ok := WorkDocIDToWorkID(tc.want)
			require.True(t, ok)
			assert.Contains(t, res.IDs, id, "%q must recall %s", tc.q, tc.want)
		})
	}
}

func TestWorksCJKTitleRecall(t *testing.T) {
	require.NoError(t, EnsureIndexes(testClient))
	t.Cleanup(func() { _, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexWorks)) })

	docs := []EntityDoc{
		{ID: "w1", EntityType: "work", NameJa: "色仕掛け学園～思春期男子誘惑作戦～"},
		{ID: "w2", EntityType: "work", NameJa: "マスカレード～地獄学園SO/DO/MU～"},
		{ID: "w3", EntityType: "work", NameJa: "アヘ顔アクメ中毒 －人体改造で狂ってイク私を見ないで－"},
		{ID: "w4", EntityType: "work", NameZh: "装甲恶鬼村正"},
	}
	task, err := testClient.Index(IndexWorks).AddDocuments(docs, nil)
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)

	idx := NewIndexer(testClient)
	for _, tc := range []struct{ name, q, want string }{
		{"full ja title, kana + kanji + wave dash", "色仕掛け学園～思春期男子誘惑作戦～", "w1"},
		{"kanji-only word from the middle of a title", "地獄学園", "w2"},
		{"fullwidth dash subtitle must not negate the work", "アヘ顔アクメ中毒 －人体改造で狂ってイク私を見ないで－", "w3"},
		{"a zh title still recalls itself", "装甲恶鬼村正", "w4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := idx.SearchWorks(t.Context(), WorksQuery{Q: tc.q, Limit: 12})
			require.NoError(t, err)
			id, ok := WorkDocIDToWorkID(tc.want)
			require.True(t, ok)
			assert.Contains(t, res.IDs, id, "%q must recall %s", tc.q, tc.want)
		})
	}
}
