package price

import (
	"context"
	"log/slog"
	"sync"
	"time"

	skeys "api/internal/platform/settings/keys"
	"api/internal/platform/store/model"

	"gorm.io/gorm"
)

type Options struct {
	NegativeFor  time.Duration
	ErrorFor     time.Duration
	HotWindow    time.Duration
	RefreshEvery time.Duration
	RefreshLimit int
	FlushEvery   time.Duration
}

type Service struct {
	db          *gorm.DB
	fetchers    map[string]Fetcher
	batchers    map[batcherKey]*batcher
	opts        Options
	mu          sync.Mutex
	started     bool
	stopped     bool
	stop        chan struct{}
	refreshDone chan struct{}
}

func New(db *gorm.DB, fetchers []Fetcher, opts Options) *Service {
	if opts.NegativeFor == 0 {
		opts.NegativeFor = 24 * time.Hour
	}
	if opts.ErrorFor == 0 {
		opts.ErrorFor = 15 * time.Minute
	}
	if opts.HotWindow == 0 {
		opts.HotWindow = 7 * 24 * time.Hour
	}
	if opts.RefreshEvery == 0 {
		opts.RefreshEvery = 60 * time.Second
	}
	if opts.RefreshLimit == 0 {
		opts.RefreshLimit = 500
	}
	if opts.FlushEvery == 0 {
		opts.FlushEvery = time.Second
	}
	fm := map[string]Fetcher{}
	for _, f := range fetchers {
		if f != nil {
			fm[f.Source()] = f
		}
	}
	return &Service{
		db:       db,
		fetchers: fm,
		batchers: map[batcherKey]*batcher{},
		opts:     opts,
	}
}

func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return
	}
	s.started = true
	s.stop = make(chan struct{})
	s.refreshDone = make(chan struct{})
	for _, f := range s.fetchers {
		for _, region := range f.Regions() {
			b := newBatcher(s, f, region, s.stop)
			s.batchers[batcherKey{f.Source(), region}] = b
			go b.loop()
		}
	}
	go s.refreshLoop()
}

func (s *Service) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	if !s.started {
		s.mu.Unlock()
		return
	}
	stop := s.stop
	batchers := s.batchers
	done := s.refreshDone
	s.mu.Unlock()
	close(stop)
	for _, b := range batchers {
		<-b.done
	}
	<-done
}

type Quote struct {
	Key             Key
	State           string
	URL             string
	Currency        string
	ListMinor       int64
	CurrentMinor    int64
	DiscountPercent int
	SaleEndsAt      *time.Time
	Converted       map[string]int64
	FetchedAt       *time.Time
	ExpiresAt       *time.Time
	Stale           bool
}

func (s *Service) Quotes(ctx context.Context, anchors []Anchor, wait time.Duration) ([]Quote, error) {
	keys := s.expand(anchors)
	now := time.Now()
	rows, err := s.loadRows(ctx, keys)
	if err != nil {
		return nil, err
	}
	type slot struct {
		key  Key
		row  *model.PriceQuote
		wait <-chan struct{}
	}
	slots := make([]slot, 0, len(keys))
	var waitChans []<-chan struct{}
	for _, k := range keys {
		sl := slot{key: k}
		if row, ok := rows[k]; ok {
			r := row
			sl.row = &r
			if !now.Before(row.ExpiresAt) {
				if b := s.batcherFor(k); b != nil {
					b.enqueue(k.ExternalID)
				}
			}
		} else if b := s.batcherFor(k); b != nil {
			sl.wait = b.enqueue(k.ExternalID)
			waitChans = append(waitChans, sl.wait)
		}
		slots = append(slots, sl)
	}
	if wait > 0 && len(waitChans) > 0 {
		waitAll(ctx, waitChans, wait)
		var missing []Key
		for _, sl := range slots {
			if sl.row == nil {
				missing = append(missing, sl.key)
			}
		}
		more, err := s.loadRows(ctx, missing)
		if err != nil {
			return nil, err
		}
		for i, sl := range slots {
			if sl.row != nil {
				continue
			}
			if row, ok := more[sl.key]; ok {
				r := row
				slots[i].row = &r
			}
		}
	}
	now = time.Now()
	out := make([]Quote, 0, len(slots))
	var served []int64
	for _, sl := range slots {
		if sl.row != nil {
			out = append(out, quoteFromRow(*sl.row, now))
			served = append(served, sl.row.ID)
			continue
		}
		out = append(out, s.pendingQuote(sl.key))
	}
	if err := s.touchRequested(ctx, served, now); err != nil {
		slog.Warn("store price: touch last_requested_at failed", "error", err)
	}
	return out, nil
}

func (s *Service) expand(anchors []Anchor) []Key {
	seen := map[Key]struct{}{}
	var keys []Key
	for _, a := range anchors {
		f, ok := s.fetchers[a.Source]
		if !ok || !f.Accepts(a.ExternalID) {
			continue
		}
		for _, region := range f.Regions() {
			k := Key{Source: a.Source, ExternalID: a.ExternalID, Region: region}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	return keys
}

func (s *Service) batcherFor(k Key) *batcher {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.batchers[batcherKey{k.Source, k.Region}]
}

func (s *Service) expiry(fetched time.Time, up Upstream) time.Time {
	if !up.Found {
		return fetched.Add(s.opts.NegativeFor)
	}
	exp := fetched.Add(time.Duration(skeys.StorePriceFreshForHours.Get()) * time.Hour)
	if up.SaleEndsAt != nil && up.SaleEndsAt.After(fetched) && up.SaleEndsAt.Before(exp) {
		return *up.SaleEndsAt
	}
	return exp
}

func (s *Service) pendingQuote(k Key) Quote {
	q := Quote{Key: k, State: "pending", Converted: map[string]int64{}}
	if f := s.fetchers[k.Source]; f != nil {
		q.URL = f.URL(k.ExternalID)
	}
	return q
}

func quoteFromRow(row model.PriceQuote, now time.Time) Quote {
	fetched := row.FetchedAt
	expires := row.ExpiresAt
	converted := decodeConverted(row.Converted)
	return Quote{
		Key:             Key{Source: row.Source, ExternalID: row.ExternalID, Region: row.Region},
		State:           row.QuoteState,
		URL:             row.URL,
		Currency:        row.Currency,
		ListMinor:       row.ListMinor,
		CurrentMinor:    row.CurrentMinor,
		DiscountPercent: int(row.DiscountPercent),
		SaleEndsAt:      row.SaleEndsAt,
		Converted:       converted,
		FetchedAt:       &fetched,
		ExpiresAt:       &expires,
		Stale:           !now.Before(row.ExpiresAt),
	}
}

func waitAll(ctx context.Context, chans []<-chan struct{}, wait time.Duration) {
	done := make(chan struct{})
	go func() {
		for _, ch := range chans {
			<-ch
		}
		close(done)
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
}
