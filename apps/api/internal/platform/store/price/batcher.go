package price

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"api/internal/platform/store/model"
)

type batcherKey struct{ source, region string }

type batcher struct {
	svc      *Service
	fetch    Fetcher
	region   string
	mu       sync.Mutex
	pending  map[string]chan struct{}
	queue    []string
	lastCall time.Time
	wake     chan struct{}
	stop     <-chan struct{}
	done     chan struct{}
}

func newBatcher(svc *Service, fetch Fetcher, region string, stop <-chan struct{}) *batcher {
	return &batcher{
		svc:     svc,
		fetch:   fetch,
		region:  region,
		pending: map[string]chan struct{}{},
		wake:    make(chan struct{}, 1),
		stop:    stop,
		done:    make(chan struct{}),
	}
}

func (b *batcher) enqueue(id string) <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.pending[id]; ok {
		return ch
	}
	ch := make(chan struct{})
	b.pending[id] = ch
	b.queue = append(b.queue, id)
	select {
	case b.wake <- struct{}{}:
	default:
	}
	return ch
}

func (b *batcher) loop() {
	defer close(b.done)
	for {
		select {
		case <-b.stop:
			b.drain()
			return
		case <-b.wake:
		}
		for {
			if !b.waitFlush() {
				b.drain()
				return
			}
			b.mu.Lock()
			n := len(b.queue)
			b.mu.Unlock()
			if n == 0 {
				break
			}
			b.flush()
		}
	}
}

func (b *batcher) waitFlush() bool {
	timer := time.NewTimer(b.svc.opts.FlushEvery)
	defer timer.Stop()
	for {
		b.mu.Lock()
		n := len(b.queue)
		batch := b.fetch.Batch()
		b.mu.Unlock()
		if n == 0 {
			return true
		}
		if batch > 0 && n >= batch {
			return true
		}
		select {
		case <-b.stop:
			return false
		case <-timer.C:
			return true
		case <-b.wake:
		}
	}
}

func (b *batcher) drain() {
	for {
		b.mu.Lock()
		n := len(b.queue)
		b.mu.Unlock()
		if n == 0 {
			break
		}
		b.flush()
	}
	b.closePending()
}

func (b *batcher) closePending() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ch := range b.pending {
		close(ch)
		delete(b.pending, id)
	}
	b.queue = nil
}

func (b *batcher) take() ([]string, []chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.queue) == 0 {
		return nil, nil
	}
	n := b.fetch.Batch()
	if n <= 0 || n > len(b.queue) {
		n = len(b.queue)
	}
	ids := append([]string{}, b.queue[:n]...)
	b.queue = append([]string{}, b.queue[n:]...)
	chans := make([]chan struct{}, 0, len(ids))
	for _, id := range ids {
		if ch, ok := b.pending[id]; ok {
			chans = append(chans, ch)
		}
	}
	return ids, chans
}

// An id stays in pending until its row is written: take() used to delete it,
// so a second request arriving during the ~1s fetch re-queued the same id and
// the storefront was asked twice for one price.
func (b *batcher) settle(ids []string, chans []chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range chans {
		close(ch)
	}
	for _, id := range ids {
		delete(b.pending, id)
	}
}

func (b *batcher) flush() {
	ids, chans := b.take()
	if len(ids) == 0 {
		return
	}
	defer b.settle(ids, chans)
	if gap := b.fetch.Gap(); gap > 0 {
		if wait := gap - time.Since(b.lastCall); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-b.stop:
				timer.Stop()
			}
		}
	}
	b.lastCall = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	got, err := b.fetch.Fetch(ctx, b.region, ids)
	now := time.Now()
	if err != nil {
		slog.Warn("store price: upstream fetch failed",
			"source", b.fetch.Source(), "region", b.region, "count", len(ids), "error", err)
		b.writeErrors(ctx, ids, now)
		return
	}
	if got == nil {
		got = map[string]Upstream{}
	}
	rows := make([]model.PriceQuote, 0, len(ids))
	for _, id := range ids {
		up, ok := got[id]
		if !ok {
			up = Upstream{Found: false}
		}
		rows = append(rows, b.rowFrom(id, up, now))
	}
	if err := b.svc.upsert(ctx, rows); err != nil {
		slog.Warn("store price: upsert failed",
			"source", b.fetch.Source(), "region", b.region, "count", len(ids), "error", err)
	}
}

func (b *batcher) writeErrors(ctx context.Context, ids []string, now time.Time) {
	keys := make([]Key, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, Key{Source: b.fetch.Source(), ExternalID: id, Region: b.region})
	}
	have, err := b.svc.loadRows(ctx, keys)
	if err != nil {
		slog.Warn("store price: load rows after fetch error failed",
			"source", b.fetch.Source(), "region", b.region, "error", err)
		return
	}
	var rows []model.PriceQuote
	for _, id := range ids {
		k := Key{Source: b.fetch.Source(), ExternalID: id, Region: b.region}
		if _, ok := have[k]; ok {
			continue
		}
		rows = append(rows, model.PriceQuote{
			Source:          b.fetch.Source(),
			ExternalID:      id,
			Region:          b.region,
			QuoteState:      "unavailable",
			URL:             b.fetch.URL(id),
			Converted:       encodeConverted(nil),
			FetchedAt:       now,
			ExpiresAt:       now.Add(b.svc.opts.ErrorFor),
			LastRequestedAt: now,
		})
	}
	if err := b.svc.upsert(ctx, rows); err != nil {
		slog.Warn("store price: upsert failed",
			"source", b.fetch.Source(), "region", b.region, "count", len(rows), "error", err)
	}
}

func (b *batcher) rowFrom(id string, up Upstream, now time.Time) model.PriceQuote {
	state := "unavailable"
	url := b.fetch.URL(id)
	if up.Found {
		state = "priced"
	}
	if up.URL != "" {
		url = up.URL
	}
	converted := up.Converted
	if converted == nil {
		converted = map[string]int64{}
	}
	return model.PriceQuote{
		Source:          b.fetch.Source(),
		ExternalID:      id,
		Region:          b.region,
		QuoteState:      state,
		URL:             url,
		Currency:        up.Currency,
		ListMinor:       up.ListMinor,
		CurrentMinor:    up.CurrentMinor,
		DiscountPercent: int16(up.DiscountPercent),
		SaleEndsAt:      up.SaleEndsAt,
		Converted:       encodeConverted(converted),
		FetchedAt:       now,
		ExpiresAt:       b.svc.expiry(now, up),
		LastRequestedAt: now,
	}
}
