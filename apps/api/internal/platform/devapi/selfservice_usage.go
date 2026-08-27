package devapi

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *SelfServiceService) Usage(ctx context.Context, ownerUserID uint, clientID string, days int) ([]UsageDayFace, error) {
	if _, err := s.repo.GetAppByOwner(ctx, clientID, ownerUserID); err != nil {
		return nil, err
	}
	since := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	return s.repo.AggregateUsageByClient(ctx, clientID, since)
}

type OwnerUsageAppTotal struct {
	ClientID  string `json:"client_id"`
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	Status4xx int64  `json:"status_4xx"`
	Status5xx int64  `json:"status_5xx"`
}

type LiveKeyUsage struct {
	AppName        string `json:"app_name"`
	KeyID          uint   `json:"key_id"`
	RateLimit      int    `json:"rate_limit"`
	QuotaLimit     int    `json:"quota_limit"`
	QuotaUsed      int64  `json:"quota_used"`
	QuotaRemaining int64  `json:"quota_remaining"`
	QuotaReset     int64  `json:"quota_reset"`
}

type OwnerUsageSummary struct {
	Days            int                  `json:"days"`
	Since           string               `json:"since"`
	TotalCount      int64                `json:"total_count"`
	Total4xx        int64                `json:"total_4xx"`
	Total5xx        int64                `json:"total_5xx"`
	Daily           []UsageDayTotal      `json:"daily"`
	ByApp           []OwnerUsageAppTotal `json:"by_app"`
	ByFace          []UsageFaceTotal     `json:"by_face"`
	Live            []LiveKeyUsage       `json:"live"`
	LiveUnavailable bool                 `json:"live_unavailable,omitempty"`
}

func (s *SelfServiceService) OwnerUsage(ctx context.Context, ownerUserID uint, days int) (*OwnerUsageSummary, error) {
	apps, err := s.repo.ListAppsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	summary := &OwnerUsageSummary{
		Days:   days,
		Since:  now.AddDate(0, 0, -(days - 1)).Format("2006-01-02"),
		ByApp:  []OwnerUsageAppTotal{},
		ByFace: []UsageFaceTotal{},
		Live:   []LiveKeyUsage{},
	}
	s.fillLive(ctx, summary, ownerUserID, now)

	if len(apps) == 0 {
		summary.Daily = denseDays(now, days, nil)
		return summary, nil
	}

	clientIDs := make([]string, len(apps))
	for i := range apps {
		clientIDs[i] = apps[i].ID
	}
	dayRows, err := s.repo.SumUsageByDay(ctx, clientIDs, summary.Since)
	if err != nil {
		return nil, err
	}
	clientRows, err := s.repo.SumUsageByClient(ctx, clientIDs, summary.Since)
	if err != nil {
		return nil, err
	}
	faceRows, err := s.repo.SumUsageByFace(ctx, clientIDs, summary.Since)
	if err != nil {
		return nil, err
	}
	if faceRows != nil {
		summary.ByFace = faceRows
	}

	summary.Daily = denseDays(now, days, dayRows)
	for i := range summary.Daily {
		summary.TotalCount += summary.Daily[i].Count
		summary.Total4xx += summary.Daily[i].Status4xx
		summary.Total5xx += summary.Daily[i].Status5xx
	}

	totalsByID := make(map[string]UsageClientTotal, len(clientRows))
	for _, r := range clientRows {
		totalsByID[r.ClientID] = r
	}
	for i := range apps {
		t := totalsByID[apps[i].ID]
		summary.ByApp = append(summary.ByApp, OwnerUsageAppTotal{
			ClientID:  apps[i].ID,
			Name:      apps[i].Name,
			Count:     t.Count,
			Status4xx: t.Status4xx,
			Status5xx: t.Status5xx,
		})
	}
	slices.SortFunc(summary.ByApp, func(a, b OwnerUsageAppTotal) int {
		switch {
		case a.Count > b.Count:
			return -1
		case a.Count < b.Count:
			return 1
		default:
			return 0
		}
	})
	return summary, nil
}

func (s *SelfServiceService) fillLive(ctx context.Context, summary *OwnerUsageSummary, ownerUserID uint, now time.Time) {
	if s.store == nil || !s.store.Available(ctx) {
		summary.LiveUnavailable = true
		return
	}
	keys, err := s.repo.ListOwnerActiveKeys(ctx, ownerUserID, now)
	if err != nil {
		summary.LiveUnavailable = true
		return
	}
	day := now.UTC().Format("2006-01-02")
	reset := nextDayStartUnix(now)
	for _, k := range keys {
		cred := &Credential{Tier: k.DevTier, RateOverride: k.DevRatePerMin, QuotaOverride: k.DevQuotaDaily}
		rate, _ := cred.EffectiveRate()
		quota, unlimited := cred.EffectiveQuota()
		row := LiveKeyUsage{
			AppName:    k.AppName,
			KeyID:      k.KeyID,
			RateLimit:  rate,
			QuotaLimit: quota,
			QuotaReset: reset,
		}
		if !unlimited {
			row.QuotaUsed = s.readCounter(ctx, quotaCounterKey(k.KeyID, day))
			if rem := int64(quota) - row.QuotaUsed; rem > 0 {
				row.QuotaRemaining = rem
			}
		}
		summary.Live = append(summary.Live, row)
	}
}

func (s *SelfServiceService) readCounter(ctx context.Context, key string) int64 {
	b, err := s.store.Get(ctx, key)
	if err != nil || len(b) == 0 {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func denseDays(now time.Time, days int, rows []UsageDayTotal) []UsageDayTotal {
	byDay := make(map[string]UsageDayTotal, len(rows))
	for _, r := range rows {
		byDay[r.Day] = r
	}
	out := make([]UsageDayTotal, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if r, ok := byDay[day]; ok {
			r.Day = day
			out = append(out, r)
		} else {
			out = append(out, UsageDayTotal{Day: day})
		}
	}
	return out
}
