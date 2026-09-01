package search

import (
	"context"
	"fmt"

	"api/internal/infrastructure/search"
	"api/internal/platform/catalog/search/spec"
	"api/pkg/config"
)

const EngineOpenSearch = "opensearch"

type Engine interface {
	EnsureIndexes(ctx context.Context) error
	UpsertBatch(ctx context.Context, uid string, docs []EntityDoc) error
	DeleteBatch(ctx context.Context, uid string, ids []string) error
	Count(ctx context.Context, uid string) (int64, error)
	Refresh(ctx context.Context, uid string) error
	Health(ctx context.Context) error
	SearchEntities(ctx context.Context, uid string, q spec.EntityQuery) (SearchResult, error)
	SearchWorks(ctx context.Context, q spec.WorksQuery) (WorksResult, error)
}

type Indexer struct{ engine Engine }

func NewIndexer(client *search.Client) *Indexer {
	return &Indexer{engine: &meiliEngine{client: client}}
}

func NewIndexerFromConfig(cfg *config.Config) (*Indexer, string, error) {
	name := cfg.SearchEngine
	if name == "" {
		name = EngineMeilisearch
	}
	switch name {
	case EngineMeilisearch:
		client, err := search.NewClient(cfg.Meilisearch)
		if err != nil {
			return nil, "", err
		}
		return NewIndexer(client), name, nil
	case EngineOpenSearch:
		client, err := newOpenSearchClient(cfg)
		if err != nil {
			return nil, "", err
		}
		return NewOpenSearchIndexer(client), name, nil
	default:
		return nil, "", fmt.Errorf("unknown search engine %q", name)
	}
}

func (i *Indexer) EnsureIndexes(ctx context.Context) error {
	return i.engine.EnsureIndexes(ctx)
}

func (i *Indexer) UpsertBatch(ctx context.Context, uid string, docs []EntityDoc) error {
	return i.engine.UpsertBatch(ctx, uid, docs)
}

func (i *Indexer) DeleteBatch(ctx context.Context, uid string, ids []string) error {
	return i.engine.DeleteBatch(ctx, uid, ids)
}

func (i *Indexer) Count(ctx context.Context, uid string) (int64, error) {
	return i.engine.Count(ctx, uid)
}

func (i *Indexer) Refresh(ctx context.Context, uid string) error {
	return i.engine.Refresh(ctx, uid)
}

func (i *Indexer) Health(ctx context.Context) error {
	return i.engine.Health(ctx)
}

func (i *Indexer) SearchEntities(ctx context.Context, uid string, q spec.EntityQuery) (SearchResult, error) {
	return i.engine.SearchEntities(ctx, uid, q)
}

func (i *Indexer) SearchWorks(ctx context.Context, q spec.WorksQuery) (WorksResult, error) {
	return i.engine.SearchWorks(ctx, q)
}

type indexRecreator interface {
	RecreateIndex(ctx context.Context, uid string) error
}

func (i *Indexer) RecreateIndex(ctx context.Context, uid string) error {
	r, ok := i.engine.(indexRecreator)
	if !ok {
		return fmt.Errorf("-recreate is unsupported for this search engine")
	}
	return r.RecreateIndex(ctx, uid)
}
