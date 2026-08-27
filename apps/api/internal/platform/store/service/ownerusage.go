package service

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"time"

	"api/internal/platform/store/model"
)

const (
	DefaultOwnerUsageDays = 30
	MaxOwnerUsageDays     = MaxStatsRangeDays
)

// OwnerApp is the slice of an application the portal panel needs. The full row
// belongs to devapi, and the caller passes only these two fields so the store
// domain does not reach into the developer-platform tables.
type OwnerApp struct {
	ClientID string
	Name     string
}

type OwnerUsageDay struct {
	Day     string `json:"day"`
	Total   int64  `json:"total"`
	Uniques int64  `json:"uniques"`
}

type OwnerUsageApp struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Links    int64  `json:"links"`
	Total    int64  `json:"total"`
	Uniques  int64  `json:"uniques"`
}

type OwnerUsageLink struct {
	ClientID   string  `json:"client_id"`
	AppName    string  `json:"app_name"`
	Kind       string  `json:"kind"`
	ProductID  *string `json:"product_id"`
	CampaignID *int64  `json:"campaign_id"`
	Total      int64   `json:"total"`
	Uniques    int64   `json:"uniques"`
}

type OwnerUsageSummary struct {
	Days      int              `json:"days"`
	Since     string           `json:"since"`
	Until     string           `json:"until"`
	Total     int64            `json:"total"`
	Uniques   int64            `json:"uniques"`
	LinkCount int64            `json:"link_count"`
	Daily     []OwnerUsageDay  `json:"daily"`
	ByApp     []OwnerUsageApp  `json:"by_app"`
	ByLink    []OwnerUsageLink `json:"by_link"`
}

func ClampOwnerUsageDays(days int) int {
	if days <= 0 {
		return DefaultOwnerUsageDays
	}
	if days > MaxOwnerUsageDays {
		return MaxOwnerUsageDays
	}
	return days
}

func (s *Service) OwnerUsage(ctx context.Context, apps []OwnerApp, days int) (*OwnerUsageSummary, error) {
	days = ClampOwnerUsageDays(days)
	now := time.Now()
	until := model.JSTDay(now)
	since := model.JSTDay(now.AddDate(0, 0, -(days - 1)))

	out := &OwnerUsageSummary{
		Days: days, Since: since, Until: until,
		Daily:  denseDays(now, days),
		ByApp:  []OwnerUsageApp{},
		ByLink: []OwnerUsageLink{},
	}
	if len(apps) == 0 {
		return out, nil
	}

	names := make(map[string]string, len(apps))
	clientIDs := make([]string, len(apps))
	for i, a := range apps {
		clientIDs[i] = a.ClientID
		names[a.ClientID] = a.Name
	}

	linkCounts, err := s.linkCounts(ctx, clientIDs)
	if err != nil {
		return nil, err
	}
	rows, err := s.ownerStatRows(ctx, clientIDs, since, until)
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]int, len(out.Daily))
	for i, d := range out.Daily {
		byDay[d.Day] = i
	}
	perApp := map[string]*OwnerUsageApp{}
	perLink := map[string]*OwnerUsageLink{}

	for _, r := range rows {
		out.Total += r.Total
		out.Uniques += r.Uniques
		if i, ok := byDay[r.Day]; ok {
			out.Daily[i].Total += r.Total
			out.Daily[i].Uniques += r.Uniques
		}

		app, ok := perApp[r.ClientID]
		if !ok {
			app = &OwnerUsageApp{ClientID: r.ClientID, Name: names[r.ClientID]}
			perApp[r.ClientID] = app
		}
		app.Total += r.Total
		app.Uniques += r.Uniques

		key := r.ClientID + "\x00" + r.Kind + "\x00" + derefString(r.ProductID) + "\x00" + derefInt64(r.CampaignID)
		link, ok := perLink[key]
		if !ok {
			link = &OwnerUsageLink{
				ClientID: r.ClientID, AppName: names[r.ClientID], Kind: r.Kind,
				ProductID: r.ProductID, CampaignID: r.CampaignID,
			}
			perLink[key] = link
		}
		link.Total += r.Total
		link.Uniques += r.Uniques
	}

	for _, a := range apps {
		row := OwnerUsageApp{ClientID: a.ClientID, Name: a.Name, Links: linkCounts[a.ClientID]}
		if got, ok := perApp[a.ClientID]; ok {
			row.Total, row.Uniques = got.Total, got.Uniques
		}
		if row.Links == 0 && row.Total == 0 {
			continue
		}
		out.LinkCount += row.Links
		out.ByApp = append(out.ByApp, row)
	}
	slices.SortFunc(out.ByApp, func(a, b OwnerUsageApp) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.ClientID, b.ClientID))
	})

	for _, l := range perLink {
		out.ByLink = append(out.ByLink, *l)
	}
	slices.SortFunc(out.ByLink, func(a, b OwnerUsageLink) int {
		return cmp.Or(
			cmp.Compare(b.Total, a.Total),
			cmp.Compare(a.ClientID, b.ClientID),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(derefString(a.ProductID), derefString(b.ProductID)),
			cmp.Compare(derefInt64(a.CampaignID), derefInt64(b.CampaignID)),
		)
	})
	return out, nil
}

type ownerStatRow struct {
	ClientID   string
	Kind       string
	ProductID  *string
	CampaignID *int64
	Day        string
	Total      int64
	Uniques    int64
}

const ownerStatsSQL = `
SELECT p.client_id, 'purchase' AS kind, p.product_id AS product_id, NULL::bigint AS campaign_id,
       s.day, s.total, s.uniques
  FROM store_link_daily_stats s
  JOIN store_purchase_links p ON p.alias = s.alias
 WHERE p.client_id IN ? AND s.day >= ? AND s.day <= ?
UNION ALL
SELECT c.client_id, 'coupon' AS kind, NULL::text AS product_id, c.campaign_id,
       s.day, s.total, s.uniques
  FROM store_link_daily_stats s
  JOIN store_coupon_links c ON c.alias = s.alias
 WHERE c.client_id IN ? AND s.day >= ? AND s.day <= ?`

func (s *Service) ownerStatRows(ctx context.Context, clientIDs []string, from, to string) ([]ownerStatRow, error) {
	var rows []ownerStatRow
	err := s.db.WithContext(ctx).
		Raw(ownerStatsSQL, clientIDs, from, to, clientIDs, from, to).
		Scan(&rows).Error
	return rows, err
}

const linkCountSQL = `
SELECT client_id, count(*) AS n FROM store_purchase_links WHERE client_id IN ? GROUP BY client_id
UNION ALL
SELECT client_id, count(*) AS n FROM store_coupon_links WHERE client_id IN ? GROUP BY client_id`

func (s *Service) linkCounts(ctx context.Context, clientIDs []string) (map[string]int64, error) {
	var rows []struct {
		ClientID string
		N        int64
	}
	if err := s.db.WithContext(ctx).Raw(linkCountSQL, clientIDs, clientIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.ClientID] += r.N
	}
	return out, nil
}

func denseDays(now time.Time, days int) []OwnerUsageDay {
	out := make([]OwnerUsageDay, 0, days)
	for i := days - 1; i >= 0; i-- {
		out = append(out, OwnerUsageDay{Day: model.JSTDay(now.AddDate(0, 0, -i))})
	}
	return out
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}
