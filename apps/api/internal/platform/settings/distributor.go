package settings

import (
	"context"
	"log/slog"
	"os"
	"time"

	"gorm.io/gorm"
)

const ChangeChannel = "settings:changed"

const PollInterval = 30 * time.Second

type Broadcaster interface {
	Publish(ctx context.Context, channel, payload string) error
	Subscribe(ctx context.Context, channel string) (<-chan string, error)
}

type Distributor struct {
	store *Store
	reg   *Registry
	bus   Broadcaster
	env   map[string]any
}

func NewDistributor(db *gorm.DB, reg *Registry, bus Broadcaster) *Distributor {
	return &Distributor{
		store: NewStore(db),
		reg:   reg,
		bus:   bus,
		env:   LoadEnv(reg, os.Getenv),
	}
}

func (d *Distributor) Refresh(ctx context.Context) error {
	rows, err := d.store.Values(ctx, PlatformScope)
	if err != nil {
		return err
	}
	for _, c := range Resolve(d.reg, rows, d.env) {
		slog.Info("settings: applied", "key", c.Key, "value", c.New, "source", c.NewSource)
	}
	return nil
}

func (d *Distributor) Announce(ctx context.Context) {
	if d.bus == nil {
		return
	}
	if err := d.bus.Publish(ctx, ChangeChannel, "refresh"); err != nil {
		slog.Warn("settings: overlay change announcement failed; peers refresh on their next poll",
			"channel", ChangeChannel, "err", err)
	}
}

var startupBackoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

func (d *Distributor) Start(ctx context.Context) {
	d.initialLoad(ctx)

	var nudges <-chan string
	if d.bus != nil {
		ch, err := d.bus.Subscribe(ctx, ChangeChannel)
		if err != nil {
			slog.Warn("settings: overlay subscribe failed; refreshing on the poll interval only",
				"channel", ChangeChannel, "err", err)
		} else {
			nudges = ch
		}
	}

	go func() {
		ticker := time.NewTicker(PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case _, ok := <-nudges:
				if !ok {
					nudges = nil
					continue
				}
			}
			if err := d.Refresh(ctx); err != nil {
				slog.Warn("settings: overlay refresh failed; previous snapshot stays in force", "err", err)
			}
		}
	}()
}

func (d *Distributor) initialLoad(ctx context.Context) {
	var lastErr error
	for attempt, wait := range startupBackoff {
		if lastErr = d.Refresh(ctx); lastErr == nil {
			if attempt > 0 {
				slog.Info("settings: overlay loaded after a retry", "attempts", attempt+1)
			}
			d.logLoaded()
			return
		}
		if attempt == len(startupBackoff)-1 {
			break
		}
		slog.Warn("settings: initial overlay load failed; retrying",
			"attempt", attempt+1, "retry_in", wait, "err", lastErr)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}

	slog.Error("settings: initial overlay load failed after every retry; "+
		"THIS PROCESS IS RUNNING ON THE CODE/ENV FLOOR ONLY — no database override is in force until a later poll succeeds",
		"attempts", len(startupBackoff), "poll_interval", PollInterval, "err", lastErr)
	Resolve(d.reg, nil, d.env)
}

func (d *Distributor) logLoaded() {
	n, nDB, nEnv := 0, 0, 0
	for _, e := range d.reg.Entries() {
		n++
		switch e.Source() {
		case SourceDB:
			nDB++
		case SourceEnv:
			nEnv++
		}
	}
	slog.Info("settings: loaded", "keys", n, "overrides", nDB, "env_floor", nEnv)
}
