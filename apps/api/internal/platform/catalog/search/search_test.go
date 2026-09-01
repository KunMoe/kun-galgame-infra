package search

import (
	"os"
	"testing"
	"time"

	"api/internal/infrastructure/search"
	"api/internal/platform/catalog/search/spec"
	"api/internal/testsupport/dbtest"
	"api/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testClient *search.Client

var testPrefix = dbtest.SearchIndexPrefix("search")

func TestMain(m *testing.M) {
	host, apiKey := dbtest.SearchHost()
	if host == "" {
		dbtest.SkipSearchMain("catalog/search", "MEILISEARCH_TEST_HOST unset")
	}
	client, err := search.NewClient(config.MeilisearchConfig{
		Host: host, APIKey: apiKey, IndexPrefix: testPrefix,
	})
	if err != nil {
		dbtest.SkipSearchMain("catalog/search", "meilisearch client: %v", err)
	}
	if err := client.Health(); err != nil {
		dbtest.SkipSearchMain("catalog/search", "meilisearch unreachable: %v", err)
	}
	testClient = client
	code := m.Run()
	dbtest.SweepSearchIndexes(client.Svc(), testPrefix)
	os.Exit(code)
}

func TestEnsureIndexesMatchesMatrix(t *testing.T) {
	require.NoError(t, NewIndexer(testClient).EnsureIndexes(t.Context()))
	t.Cleanup(func() {
		for _, uid := range []string{IndexCreditNames, IndexCharacters, IndexLabels, IndexWorks, IndexTags} {
			_, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(uid))
		}
	})

	for _, uid := range []string{IndexCreditNames, IndexCharacters, IndexLabels, IndexWorks, IndexTags} {
		s, err := testClient.Index(uid).GetSettings()
		require.NoError(t, err, uid)

		locales := map[string][]string{}
		for _, la := range s.LocalizedAttributes {
			for _, p := range la.AttributePatterns {
				locales[p] = la.Locales
			}
		}
		if uid == IndexWorks {
			assert.Empty(t, s.LocalizedAttributes, uid)
			assert.Nil(t, LocalesForUI(uid, "zh"), "the works lane must never pin a query locale either")
		} else {
			assert.Equal(t, []string{"jpn"}, locales["*_ja"], uid)
			assert.Equal(t, []string{"cmn"}, locales["*_zh"], uid)
		}

		assert.ElementsMatch(t, titleDelimiterSeparators, s.SeparatorTokens, uid)

		require.NotNil(t, s.TypoTolerance, uid)
		assert.Contains(t, s.TypoTolerance.DisableOnAttributes, "name_ja", uid)
		assert.Contains(t, s.TypoTolerance.DisableOnAttributes, "name_zh", uid)
		assert.NotContains(t, s.TypoTolerance.DisableOnAttributes, "latin", uid)

		if uid != IndexTags {
			for _, f := range []string{"aliases_ja", "aliases_zh", "aliases_other"} {
				assert.Contains(t, s.SearchableAttributes, f, uid)
			}
			assert.Less(t, indexOf(s.SearchableAttributes, "name_ja"),
				indexOf(s.SearchableAttributes, "aliases_ja"), uid)
			assert.Less(t, indexOf(s.SearchableAttributes, "aliases_other"),
				indexOf(s.SearchableAttributes, "latin"), uid)
		}

		assert.Contains(t, s.SortableAttributes, "popularity", uid)
		rr := s.RankingRules
		require.NotEmpty(t, rr, uid)
		assert.Equal(t, "popularity:desc", rr[len(rr)-1], uid)
		assert.Less(t, indexOf(rr, "exactness"), indexOf(rr, "popularity:desc"), uid)
		assert.Less(t, indexOf(rr, "words"), indexOf(rr, "sort"), uid)
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func TestWorksNSFWFilter(t *testing.T) {
	require.NoError(t, NewIndexer(testClient).EnsureIndexes(t.Context()))
	t.Cleanup(func() {
		_, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexWorks))
	})

	r18, safe := int16(2), int16(0)
	docA := EntityDoc{ID: "w1", EntityType: "work", ContentRating: &r18, Popularity: 3}
	docA.SetNameOrAlias("ja", "いろとりどりのセカイ")
	docA.SetNameOrAlias("zh", "五彩斑斓的世界")
	docB := EntityDoc{ID: "w2", EntityType: "work", ContentRating: &safe, Popularity: 1}
	docB.SetNameOrAlias("ja", "全年齢作品")

	task, err := testClient.Index(IndexWorks).AddDocuments([]EntityDoc{docA, docB}, nil)
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)

	idx := NewIndexer(testClient)
	ctx := t.Context()

	res, err := idx.SearchEntities(ctx, IndexWorks, spec.EntityQuery{Q: "いろとりどり", Locales: []string{"jpn"}, Limit: 20, ContentRatingNot: &r18})
	require.NoError(t, err)
	assert.Len(t, res.Hits, 0, "r18 filtered")
	res, err = idx.SearchEntities(ctx, IndexWorks, spec.EntityQuery{Q: "五彩斑斓", Locales: []string{"cmn"}, Limit: 20})
	require.NoError(t, err)
	if assert.Len(t, res.Hits, 1) {
		assert.Equal(t, "w1", res.Hits[0].ID)
		require.NotNil(t, res.Hits[0].ContentRating)
		assert.EqualValues(t, 2, *res.Hits[0].ContentRating)
	}
	res, err = idx.SearchEntities(ctx, IndexWorks, spec.EntityQuery{Q: "", Limit: 20, ContentRatingNot: &r18})
	require.NoError(t, err)
	if assert.Len(t, res.Hits, 1) {
		assert.Equal(t, "w2", res.Hits[0].ID)
	}
}

func TestLabelAliasIsSearchable(t *testing.T) {
	require.NoError(t, NewIndexer(testClient).EnsureIndexes(t.Context()))
	t.Cleanup(func() {
		_, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexLabels))
	})

	kind := int16(4)
	doc := EntityDoc{ID: "b5147", EntityType: "label", Kind: &kind}
	doc.SetName("ja", "ゆずソフト")
	for _, a := range []string{"Yuzu-Soft", "Yuzusoft", "ユズソフト"} {
		doc.AddAlias("en", a)
	}
	doc.AddAlias("zh", "柚子社")

	task, err := testClient.Index(IndexLabels).AddDocuments([]EntityDoc{doc}, nil)
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)

	idx := NewIndexer(testClient)
	for _, q := range []string{"Yuzusoft", "yuzusoft", "柚子社", "ゆずソフト"} {
		res, err := idx.SearchEntities(t.Context(), IndexLabels, spec.EntityQuery{Q: q, Limit: 20})
		require.NoError(t, err, q)
		if assert.Len(t, res.Hits, 1, "query %q found nothing", q) {
			assert.Equal(t, "b5147", res.Hits[0].ID, q)
		}
	}
}

func TestDeleteBatchRemovesTombstones(t *testing.T) {
	require.NoError(t, NewIndexer(testClient).EnsureIndexes(t.Context()))
	t.Cleanup(func() {
		_, _ = testClient.Svc().DeleteIndex(testClient.IndexUID(IndexLabels))
	})

	live := EntityDoc{ID: "b5147", EntityType: "label"}
	live.SetName("ja", "ゆずソフト")
	dead := EntityDoc{ID: "b96", EntityType: "label"}
	dead.SetName("ja", "ゆずソフト")

	task, err := testClient.Index(IndexLabels).AddDocuments([]EntityDoc{live, dead}, nil)
	require.NoError(t, err)
	_, err = testClient.Svc().WaitForTask(task.TaskUID, 0)
	require.NoError(t, err)

	idx := NewIndexer(testClient)
	ctx := t.Context()
	res, err := idx.SearchEntities(ctx, IndexLabels, spec.EntityQuery{Q: "ゆずソフト", Locales: []string{"jpn"}, Limit: 20})
	require.NoError(t, err)
	require.Len(t, res.Hits, 2, "both are indexed before the purge")

	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, []string{"b96"}))
	require.Eventually(t, func() bool {
		res, err := idx.SearchEntities(ctx, IndexLabels, spec.EntityQuery{Q: "ゆずソフト", Locales: []string{"jpn"}, Limit: 20})
		return err == nil && len(res.Hits) == 1 && res.Hits[0].ID == "b5147"
	}, 10*time.Second, 100*time.Millisecond, "the merged label is gone, the survivor stays")

	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, []string{"b96"}))
	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, nil))
}
