package devapi

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type usageKey struct {
	clientID string
	keyID    uint
	face     string
	path     string
	day      string
}

type usageDelta struct {
	count, s4xx, s5xx int64
}

type UsageRecorder struct {
	mu     sync.Mutex
	deltas map[usageKey]*usageDelta
	repo   *Repository
	store  Store
}

func NewUsageRecorder(repo *Repository, store Store) *UsageRecorder {
	return &UsageRecorder{deltas: make(map[usageKey]*usageDelta), repo: repo, store: store}
}

func (u *UsageRecorder) Record(cred *Credential, face, path string, status int) {
	day := time.Now().UTC().Format("2006-01-02")
	k := usageKey{clientID: cred.ClientID, keyID: cred.KeyID, face: face, path: path, day: day}
	u.mu.Lock()
	defer u.mu.Unlock()
	d := u.deltas[k]
	if d == nil {
		d = &usageDelta{}
		u.deltas[k] = d
	}
	d.count++
	switch {
	case status >= 500:
		d.s5xx++
	case status >= 400:
		d.s4xx++
	}
}

func (u *UsageRecorder) Flush(ctx context.Context) error {
	u.mu.Lock()
	pending := u.deltas
	u.deltas = make(map[usageKey]*usageDelta)
	u.mu.Unlock()

	if len(pending) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]DeveloperAPIUsage, 0, len(pending))
	for k, d := range pending {
		rows = append(rows, DeveloperAPIUsage{
			ClientID:  k.clientID,
			KeyID:     k.keyID,
			Face:      k.face,
			Path:      k.path,
			Day:       k.day,
			Count:     d.count,
			Status4xx: d.s4xx,
			Status5xx: d.s5xx,
			UpdatedAt: now,
		})
	}
	if err := u.repo.UpsertUsage(ctx, rows); err != nil {
		u.remerge(pending)
		return err
	}
	return nil
}

func (u *UsageRecorder) remerge(pending map[usageKey]*usageDelta) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for k, d := range pending {
		cur := u.deltas[k]
		if cur == nil {
			cur = &usageDelta{}
			u.deltas[k] = cur
		}
		cur.count += d.count
		cur.s4xx += d.s4xx
		cur.s5xx += d.s5xx
	}
}

func (u *UsageRecorder) TouchLastUsed(ctx context.Context, cred *Credential) {
	minute := time.Now().UTC().Unix() / 60
	key := fmt.Sprintf("devkey:lastused:%d:%d", cred.KeyID, minute)
	n, err := u.store.Incr(ctx, key, 90*time.Second)
	if err != nil || n != 1 {
		return
	}
	_ = u.repo.TouchLastUsed(ctx, cred.KeyID, time.Now().UTC())
}
