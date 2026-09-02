package price

import (
	"context"
	"time"
)

type Anchor struct{ Source, ExternalID string }

type Key struct{ Source, ExternalID, Region string }

type Upstream struct {
	Found           bool
	URL             string
	Currency        string
	ListMinor       int64
	CurrentMinor    int64
	DiscountPercent int
	SaleEndsAt      *time.Time
	Converted       map[string]int64
}

type Fetcher interface {
	Source() string
	Regions() []string
	Batch() int
	Gap() time.Duration
	Accepts(externalID string) bool
	URL(externalID string) string
	Fetch(ctx context.Context, region string, ids []string) (map[string]Upstream, error)
}
