package jobs

import (
	"context"
	"log/slog"
	"time"

	"api/internal/platform/settings/keys"
)

const staleMaxAge = 6 * time.Hour

const rescheduleTick = 30 * time.Second

func StartScheduler(ctx context.Context, reg *Registry, runner *Runner) {
	runner.ReapStale(ctx, staleMaxAge)
	for _, job := range reg.List() {
		go runLoop(ctx, job, runner)
	}
	slog.Info("jobs: scheduler started", "total", len(reg.List()))
}

func runLoop(ctx context.Context, job Job, runner *Runner) {
	jk, _ := keys.Job(job.Name)
	var (
		spec  string
		sched Schedule
		next  time.Time
	)
	for {
		if cur := jk.Schedule.Get(); cur != spec {
			spec = cur
			s, err := ParseSchedule(cur)
			if err != nil {
				slog.Error("jobs: bad schedule; job idle until it changes", "job", job.Name, "schedule", cur, "err", err)
				next = time.Time{}
			} else {
				sched = s
				next = s.Next(time.Now())
				slog.Info("jobs: scheduled", "job", job.Name, "schedule", cur, "next", next.Format(time.RFC3339))
			}
		}
		wait := rescheduleTick
		if !next.IsZero() {
			if d := time.Until(next); d < wait {
				wait = d
			}
		}
		select {
		case <-ctx.Done():
			slog.Info("jobs: scheduler loop stopped", "job", job.Name)
			return
		case <-time.After(wait):
		}
		if next.IsZero() || time.Now().Before(next) {
			continue
		}
		if jk.Enabled.Get() {
			runner.Run(ctx, job, TriggerSchedule)
		} else {
			slog.Info("jobs: skipped, disabled", "job", job.Name, "schedule", spec)
		}
		next = sched.Next(time.Now())
	}
}
