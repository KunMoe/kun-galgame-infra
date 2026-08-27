package service

import (
	"context"
	"errors"
	"time"

	"api/internal/platform/store/model"
)

const (
	MaxStatsRangeDays     = 92
	DefaultStatsRangeDays = 30
)

var ErrInvalidRange = errors.New("store: from/to must be YYYY-MM-DD JST days, from <= to, at most 92 days apart")

type StatRow struct {
	Kind       string  `json:"kind" doc:"purchase (a product link) or coupon (a campaign's claim link)"`
	ProductID  *string `json:"product_id" doc:"The DLsite product number for a purchase row; null on a coupon row"`
	CampaignID *int64  `json:"campaign_id" doc:"The campaign id for a coupon row; null on a purchase row"`
	Date       string  `json:"date" doc:"JST calendar day, YYYY-MM-DD"`
	Total      int64   `json:"total" doc:"Clicks that day, before de-duplication"`
	Uniques    int64   `json:"uniques" doc:"Distinct (day, fingerprint) clicks that day — the number settlement uses"`
}

type StatTotal struct {
	Kind    string `json:"kind,omitempty" doc:"Omitted on the grand total"`
	Total   int64  `json:"total" doc:"Clicks in the range, before de-duplication"`
	Uniques int64  `json:"uniques" doc:"De-duplicated clicks in the range"`
}

type MyStats struct {
	From   string      `json:"from" doc:"First JST day covered, inclusive"`
	To     string      `json:"to" doc:"Last JST day covered, inclusive"`
	Rows   []StatRow   `json:"rows" doc:"One row per link per day; days with no clicks are absent"`
	Totals StatTotal   `json:"totals" doc:"Grand total over the range"`
	ByKind []StatTotal `json:"by_kind" doc:"Totals split into purchase and coupon"`
}

// ResolveRange turns the two optional query values into a closed JST-day
// interval, defaulting to the last DefaultStatsRangeDays days ending today.
func ResolveRange(now time.Time, rawFrom, rawTo string) (from, to string, err error) {
	today := model.JSTDay(now)

	toDay := today
	if rawTo != "" {
		if _, ok := model.ParseJSTDay(rawTo); !ok {
			return "", "", ErrInvalidRange
		}
		toDay = rawTo
	}
	toT, _ := model.ParseJSTDay(toDay)

	fromDay := model.JSTDay(toT.AddDate(0, 0, -(DefaultStatsRangeDays - 1)))
	if rawFrom != "" {
		if _, ok := model.ParseJSTDay(rawFrom); !ok {
			return "", "", ErrInvalidRange
		}
		fromDay = rawFrom
	}
	fromT, _ := model.ParseJSTDay(fromDay)

	if toT.Before(fromT) || model.DaySpan(fromT, toT) > MaxStatsRangeDays {
		return "", "", ErrInvalidRange
	}
	return fromDay, toDay, nil
}

type statScanRow struct {
	Kind       string
	ProductID  *string
	CampaignID *int64
	Day        string
	Total      int64
	Uniques    int64
}

const clientStatsSQL = `
SELECT 'purchase' AS kind, p.product_id AS product_id, NULL::bigint AS campaign_id,
       s.day, s.total, s.uniques
  FROM store_link_daily_stats s
  JOIN store_purchase_links p ON p.alias = s.alias
 WHERE p.client_id IN ? AND s.day >= ? AND s.day <= ?
UNION ALL
SELECT 'coupon' AS kind, NULL::text AS product_id, c.campaign_id,
       s.day, s.total, s.uniques
  FROM store_link_daily_stats s
  JOIN store_coupon_links c ON c.alias = s.alias
 WHERE c.client_id IN ? AND s.day >= ? AND s.day <= ?
 ORDER BY day, kind, product_id NULLS LAST, campaign_id NULLS LAST`

func (s *Service) MyStats(ctx context.Context, clientID, from, to string) (*MyStats, error) {
	rows, err := s.statRows(ctx, []string{clientID}, from, to)
	if err != nil {
		return nil, err
	}
	out := &MyStats{From: from, To: to, Rows: make([]StatRow, 0, len(rows))}
	purchase := StatTotal{Kind: model.KindPurchase}
	coupon := StatTotal{Kind: model.KindCoupon}
	for _, r := range rows {
		out.Rows = append(out.Rows, StatRow{
			Kind: r.Kind, ProductID: r.ProductID, CampaignID: r.CampaignID,
			Date: r.Day, Total: r.Total, Uniques: r.Uniques,
		})
		out.Totals.Total += r.Total
		out.Totals.Uniques += r.Uniques
		if r.Kind == model.KindCoupon {
			coupon.Total += r.Total
			coupon.Uniques += r.Uniques
		} else {
			purchase.Total += r.Total
			purchase.Uniques += r.Uniques
		}
	}
	out.ByKind = []StatTotal{purchase, coupon}
	return out, nil
}

func (s *Service) statRows(ctx context.Context, clientIDs []string, from, to string) ([]statScanRow, error) {
	if len(clientIDs) == 0 {
		return nil, nil
	}
	var rows []statScanRow
	err := s.db.WithContext(ctx).
		Raw(clientStatsSQL, clientIDs, from, to, clientIDs, from, to).
		Scan(&rows).Error
	return rows, err
}
