package main

import (
	"context"
	"testing"
	"time"

	infrasearch "api/internal/infrastructure/search"
	"api/internal/platform/catalog/model"
	catalogSearch "api/internal/platform/catalog/search"
	"api/internal/testsupport/dbtest"
	"api/pkg/config"
)

func TestReindexWorksPurgesSoftDeleted(t *testing.T) {
	truncateFacetTables(t)
	live := mkWork(t, "purge-live", 0, galgameMedium)
	dead := mkWork(t, "purge-dead", 0, galgameMedium)
	if err := facetTestDB.Delete(&model.CatalogWork{}, dead).Error; err != nil {
		t.Fatalf("soft-delete work %d: %v", dead, err)
	}

	host, apiKey := dbtest.SearchHost()
	if host == "" {
		dbtest.SkipSearch(t, "MEILISEARCH_TEST_HOST unset")
	}
	prefix := dbtest.SearchIndexPrefix("reindex")
	client, err := infrasearch.NewClient(config.MeilisearchConfig{
		Host: host, APIKey: apiKey, IndexPrefix: prefix,
	})
	if err != nil {
		dbtest.SkipSearch(t, "meilisearch client: %v", err)
	}
	if err := client.Health(); err != nil {
		dbtest.SkipSearch(t, "meilisearch unreachable: %v", err)
	}
	idx := catalogSearch.NewIndexer(client)
	if err := idx.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	t.Cleanup(func() { dbtest.SweepSearchIndexes(client.Svc(), prefix) })
	docs := []catalogSearch.EntityDoc{
		catalogSearch.BuildWorkDoc(catalogSearch.WorkDocInput{ID: live, DisplayName: "purge-live", OLang: "ja"}),
		catalogSearch.BuildWorkDoc(catalogSearch.WorkDocInput{ID: dead, DisplayName: "purge-dead", OLang: "ja"}),
	}
	if err := idx.UpsertBatch(context.Background(), catalogSearch.IndexWorks, docs); err != nil {
		t.Fatalf("seed index: %v", err)
	}
	waitWorkDocs(t, client, 2)

	if err := reindexWorks(context.Background(), facetTestDB, idx, 100); err != nil {
		t.Fatalf("reindexWorks: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		var doc map[string]any
		deadErr := client.Index(catalogSearch.IndexWorks).GetDocument(catalogSearch.WorkDocID(dead), nil, &doc)
		liveErr := client.Index(catalogSearch.IndexWorks).GetDocument(catalogSearch.WorkDocID(live), nil, &doc)
		if deadErr != nil && liveErr == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("purge did not converge: dead doc err=%v (want non-nil), live doc err=%v (want nil)",
				deadErr, liveErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitWorkDocs(t *testing.T, client *infrasearch.Client, want int64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		stats, err := client.Index(catalogSearch.IndexWorks).GetStats()
		if err == nil && stats.NumberOfDocuments == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("works index did not reach %d docs (last err %v)", want, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
