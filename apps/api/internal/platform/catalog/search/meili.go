package search

import (
	"context"

	"api/internal/infrastructure/search"
)

const EngineMeilisearch = "meilisearch"

type meiliEngine struct {
	client *search.Client
}

func (e *meiliEngine) Health(ctx context.Context) error {
	return e.client.Health()
}

func (e *meiliEngine) Refresh(context.Context, string) error { return nil }

func (e *meiliEngine) UpsertBatch(ctx context.Context, uid string, docs []EntityDoc) error {
	if len(docs) == 0 {
		return nil
	}
	_, err := e.client.Index(uid).AddDocumentsWithContext(ctx, docs, nil)
	return err
}

func (e *meiliEngine) DeleteBatch(ctx context.Context, uid string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := e.client.Index(uid).DeleteDocumentsWithContext(ctx, ids, nil)
	return err
}

func (e *meiliEngine) Count(ctx context.Context, uid string) (int64, error) {
	stats, err := e.client.Index(uid).GetStats()
	if err != nil {
		return 0, err
	}
	return stats.NumberOfDocuments, nil
}
