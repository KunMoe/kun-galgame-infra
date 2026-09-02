package price

import (
	"context"
	"log/slog"
	"time"
)

func (s *Service) refreshLoop() {
	defer close(s.refreshDone)
	t := time.NewTicker(s.opts.RefreshEvery)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.refreshTick()
		}
	}
}

func (s *Service) refreshTick() {
	keys, err := s.dueForRefresh(context.Background(), time.Now(), s.opts.HotWindow, s.opts.RefreshLimit)
	if err != nil {
		slog.Warn("store price: refresh query failed", "error", err)
		return
	}
	for _, k := range keys {
		if b := s.batcherFor(k); b != nil {
			b.enqueue(k.ExternalID)
		}
	}
}
