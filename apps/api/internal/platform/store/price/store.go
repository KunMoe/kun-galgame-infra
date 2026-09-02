package price

import (
	"context"
	"encoding/json"
	"time"

	"api/internal/platform/store/model"

	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

func (s *Service) loadRows(ctx context.Context, keys []Key) (map[Key]model.PriceQuote, error) {
	out := map[Key]model.PriceQuote{}
	if len(keys) == 0 {
		return out, nil
	}
	tuples := make([][]any, len(keys))
	for i, k := range keys {
		tuples[i] = []any{k.Source, k.ExternalID, k.Region}
	}
	var rows []model.PriceQuote
	if err := s.db.WithContext(ctx).
		Where("(source, external_id, region) IN ?", tuples).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[Key{Source: r.Source, ExternalID: r.ExternalID, Region: r.Region}] = r
	}
	return out, nil
}

func (s *Service) upsert(ctx context.Context, rows []model.PriceQuote) error {
	if len(rows) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source"}, {Name: "external_id"}, {Name: "region"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"quote_state", "url", "currency", "list_minor", "current_minor",
			"discount_percent", "sale_ends_at", "converted", "fetched_at", "expires_at",
		}),
	}).Create(&rows).Error
}

func (s *Service) touchRequested(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.PriceQuote{}).
		Where("id IN ? AND last_requested_at < ?", ids, now.Add(-time.Hour)).
		Update("last_requested_at", now).Error
}

func (s *Service) dueForRefresh(ctx context.Context, now time.Time, hot time.Duration, limit int) ([]Key, error) {
	var rows []model.PriceQuote
	q := s.db.WithContext(ctx).Model(&model.PriceQuote{}).
		Select("source", "external_id", "region").
		Where("expires_at < ? AND last_requested_at > ?", now, now.Add(-hot)).
		Order("expires_at")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Key, 0, len(rows))
	for _, r := range rows {
		out = append(out, Key{Source: r.Source, ExternalID: r.ExternalID, Region: r.Region})
	}
	return out, nil
}

func encodeConverted(m map[string]int64) datatypes.JSON {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func decodeConverted(j datatypes.JSON) map[string]int64 {
	out := map[string]int64{}
	if len(j) == 0 {
		return out
	}
	_ = json.Unmarshal(j, &out)
	if out == nil {
		return map[string]int64{}
	}
	return out
}
