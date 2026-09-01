package search

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"api/internal/infrastructure/opensearch"
	"api/internal/platform/catalog/search/spec"
	"api/internal/testsupport/dbtest"
	"api/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testOS *opensearch.Client

var testIdx *Indexer

var testPrefix = dbtest.SearchIndexPrefix("search")

func TestMain(m *testing.M) {
	host := dbtest.SearchHost()
	if host == "" {
		dbtest.SkipSearchMain("catalog/search", "KUN_OPENSEARCH_TEST_HOST unset")
	}
	client, err := opensearch.NewClient(config.OpenSearchConfig{
		Host: host, IndexPrefix: testPrefix,
	})
	if err != nil {
		dbtest.SkipSearchMain("catalog/search", "opensearch client: %v", err)
	}
	if err := client.Health(context.Background()); err != nil {
		dbtest.SkipSearchMain("catalog/search", "opensearch unreachable: %v", err)
	}
	testOS = client
	testIdx = NewOpenSearchIndexer(client)
	code := m.Run()
	dbtest.SweepSearchIndexes(client, testPrefix)
	os.Exit(code)
}

func ensureSearchIndexes(t *testing.T, uids ...string) *Indexer {
	t.Helper()
	require.NoError(t, testIdx.EnsureIndexes(t.Context()))
	t.Cleanup(func() {
		for _, uid := range uids {
			_ = testOS.Do(context.Background(), http.MethodDelete, "/"+testOS.IndexName(uid), nil, nil)
		}
	})
	return testIdx
}

func putDocs(t *testing.T, uid string, docs []EntityDoc) {
	t.Helper()
	require.NoError(t, testIdx.UpsertBatch(t.Context(), uid, docs))
	require.NoError(t, testIdx.Refresh(t.Context(), uid))
}

func TestWorksNSFWFilter(t *testing.T) {
	idx := ensureSearchIndexes(t, IndexWorks)

	r18, safe := int16(2), int16(0)
	docA := EntityDoc{ID: "w1", EntityType: "work", ContentRating: &r18, Popularity: 3}
	docA.SetNameOrAlias("ja", "いろとりどりのセカイ")
	docA.SetNameOrAlias("zh", "五彩斑斓的世界")
	docB := EntityDoc{ID: "w2", EntityType: "work", ContentRating: &safe, Popularity: 1}
	docB.SetNameOrAlias("ja", "全年龄作品")
	putDocs(t, IndexWorks, []EntityDoc{docA, docB})

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
	idx := ensureSearchIndexes(t, IndexLabels)

	kind := int16(4)
	doc := EntityDoc{ID: "b5147", EntityType: "label", Kind: &kind}
	doc.SetName("ja", "ゆずソフト")
	for _, a := range []string{"Yuzu-Soft", "Yuzusoft", "ユズソフト"} {
		doc.AddAlias("en", a)
	}
	doc.AddAlias("zh", "柚子社")
	putDocs(t, IndexLabels, []EntityDoc{doc})

	for _, q := range []string{"Yuzusoft", "yuzusoft", "柚子社", "ゆずソフト"} {
		res, err := idx.SearchEntities(t.Context(), IndexLabels, spec.EntityQuery{Q: q, Limit: 20})
		require.NoError(t, err, q)
		if assert.Len(t, res.Hits, 1, "query %q found nothing", q) {
			assert.Equal(t, "b5147", res.Hits[0].ID, q)
		}
	}
}

func TestDeleteBatchRemovesTombstones(t *testing.T) {
	idx := ensureSearchIndexes(t, IndexLabels)

	live := EntityDoc{ID: "b5147", EntityType: "label"}
	live.SetName("ja", "ゆずソフト")
	dead := EntityDoc{ID: "b96", EntityType: "label"}
	dead.SetName("ja", "ゆずソフト")
	putDocs(t, IndexLabels, []EntityDoc{live, dead})

	ctx := t.Context()
	res, err := idx.SearchEntities(ctx, IndexLabels, spec.EntityQuery{Q: "ゆずソフト", Locales: []string{"jpn"}, Limit: 20})
	require.NoError(t, err)
	require.Len(t, res.Hits, 2, "both are indexed before the purge")

	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, []string{"b96"}))
	require.NoError(t, idx.Refresh(ctx, IndexLabels))
	require.Eventually(t, func() bool {
		res, err := idx.SearchEntities(ctx, IndexLabels, spec.EntityQuery{Q: "ゆずソフト", Locales: []string{"jpn"}, Limit: 20})
		return err == nil && len(res.Hits) == 1 && res.Hits[0].ID == "b5147"
	}, 10*time.Second, 100*time.Millisecond, "the merged label is gone, the survivor stays")

	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, []string{"b96"}))
	require.NoError(t, idx.DeleteBatch(ctx, IndexLabels, nil))
}
