package jobs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	jobmodel "api/internal/jobs/model"
	"api/pkg/config"
)

const (
	TriggerSchedule = jobmodel.TriggerSchedule
	TriggerAdmin    = jobmodel.TriggerAdmin
)

type Summary map[string]any

type RunFunc func(ctx context.Context, cfg *config.Config) (Summary, error)

type Job struct {
	Name string
	Desc string
	Run  RunFunc
}

type Schedule struct {
	DailyAt string
	Every   time.Duration
}

func (s Schedule) Zero() bool {
	return s.DailyAt == "" && s.Every <= 0
}

func (s Schedule) Next(now time.Time) time.Time {
	if s.DailyAt != "" {
		t, err := time.Parse("15:04", s.DailyAt)
		if err != nil {
			return time.Time{}
		}
		cand := time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, now.Location())
		if !cand.After(now) {
			cand = cand.Add(24 * time.Hour)
		}
		return cand
	}
	if s.Every > 0 {
		return now.Add(s.Every)
	}
	return time.Time{}
}

func (s Schedule) String() string {
	if s.DailyAt != "" {
		return "daily@" + s.DailyAt
	}
	if s.Every > 0 {
		if s.Every%time.Hour == 0 {
			return fmt.Sprintf("every:%dh", s.Every/time.Hour)
		}
		return fmt.Sprintf("every:%dm", s.Every/time.Minute)
	}
	return ""
}

func ParseSchedule(s string) (Schedule, error) {
	if hhmm, ok := strings.CutPrefix(s, "daily@"); ok {
		if _, err := time.Parse("15:04", hhmm); err != nil || len(hhmm) != 5 {
			return Schedule{}, fmt.Errorf("jobs: invalid schedule %q", s)
		}
		return Schedule{DailyAt: hhmm}, nil
	}
	if rest, ok := strings.CutPrefix(s, "every:"); ok {
		switch {
		case strings.HasSuffix(rest, "m"):
			n, err := strconv.Atoi(strings.TrimSuffix(rest, "m"))
			if err != nil || n < 1 {
				return Schedule{}, fmt.Errorf("jobs: invalid schedule %q", s)
			}
			return Schedule{Every: time.Duration(n) * time.Minute}, nil
		case strings.HasSuffix(rest, "h"):
			n, err := strconv.Atoi(strings.TrimSuffix(rest, "h"))
			if err != nil || n < 1 {
				return Schedule{}, fmt.Errorf("jobs: invalid schedule %q", s)
			}
			return Schedule{Every: time.Duration(n) * time.Hour}, nil
		}
	}
	return Schedule{}, fmt.Errorf("jobs: invalid schedule %q", s)
}
