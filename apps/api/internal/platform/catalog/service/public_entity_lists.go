package service

import (
	"context"
	"strings"

	"api/internal/platform/catalog/dto"
)

const (
	taxonomyLaneCharacters = "characters"
	taxonomyLaneNames      = "credit_names"
	taxonomyLanePersons    = "persons"
	taxonomyLaneTraits     = "traits"
)

type EntityListRow struct {
	ID          int64
	DisplayName string
	Latin       *string
	PersonID    *int64
	VndbTID     string
	NameZh      string
	Sexual      bool
	Localized   map[string]dto.PublicLocalizedName
}

type EntityListPage struct {
	Items      []EntityListRow
	NextCursor *string
	Total      int64
}

type CharacterAttributes struct {
	Gender        *int16
	BirthdayMonth *int16
	BirthdayDay   *int16
	BloodType     *int16
	HeightCm      *int16
	WeightKg      *int16
	BustCm        *int16
	WaistCm       *int16
	HipCm         *int16
	Cup           *string
	InstanceOf    *int64
}

type PersonRow struct {
	ID                  int64
	DisplayName         string
	PrimaryCreditNameID *int64
	Gender              *int16
}

type TraitRow struct {
	ID          int64
	DisplayName string
	NameZh      string
	VndbTID     string
	Sexual      bool
	Description string
}

func (s *PublicService) CharactersList(ctx context.Context, ids []int64, cursor string, limit int) (EntityListPage, error) {
	return s.entityIDList(ctx, entityListSpec{
		lane:      taxonomyLaneCharacters,
		table:     "catalog_character",
		selectSQL: "id, display_name, latin",
		deleted:   true,
		ids:       ids, cursor: cursor, limit: limit,
		alias: "character",
	})
}

func (s *PublicService) NamesList(ctx context.Context, ids []int64, q, cursor string, limit int) (EntityListPage, error) {
	return s.entityIDList(ctx, entityListSpec{
		lane:      taxonomyLaneNames,
		table:     "catalog_credit_name",
		selectSQL: "id, name AS display_name, latin, person_id",
		ids:       ids, q: q, qCol: "name", cursor: cursor, limit: limit,
		alias: "name",
	})
}

func (s *PublicService) PersonsList(ctx context.Context, ids []int64, cursor string, limit int) (EntityListPage, error) {
	return s.entityIDList(ctx, entityListSpec{
		lane:      taxonomyLanePersons,
		table:     "catalog_person",
		selectSQL: "id, display_name",
		deleted:   true,
		ids:       ids, cursor: cursor, limit: limit,
	})
}

func (s *PublicService) TraitsList(ctx context.Context, ids []int64, cursor string, limit int) (EntityListPage, error) {
	return s.entityIDList(ctx, entityListSpec{
		lane:      taxonomyLaneTraits,
		table:     "catalog_character_trait",
		selectSQL: "id, name AS display_name, name_zh, vndb_tid, sexual",
		ids:       ids, cursor: cursor, limit: limit,
	})
}

type entityListSpec struct {
	lane, table, selectSQL, q, qCol, alias, cursor string
	deleted                                        bool
	ids                                            []int64
	limit                                          int
}

func (s *PublicService) entityIDList(ctx context.Context, spec entityListSpec) (EntityListPage, error) {
	cur, err := decodePublicCursor(spec.cursor, spec.lane)
	if err != nil {
		return EntityListPage{}, err
	}
	limit := clampBrowseLimit(spec.limit)
	var where []string
	var args []any
	if spec.deleted {
		where = append(where, "deleted_at IS NULL")
	}
	if len(spec.ids) > 0 {
		where = append(where, "id IN ?")
		args = append(args, spec.ids)
	}
	if spec.q != "" && spec.qCol != "" {
		where = append(where, spec.qCol+" ILIKE ?")
		args = append(args, "%"+spec.q+"%")
	}
	filterWhere, filterArgs := append([]string(nil), where...), append([]any(nil), args...)
	if cur.ID > 0 {
		where = append(where, "id > ?")
		args = append(args, cur.ID)
	}
	q := `SELECT ` + spec.selectSQL + ` FROM ` + spec.table + ` ` + whereClause(where) + ` ORDER BY id ASC`
	q, args, paginated := applyBrowseLimit(q, args, limit+taxonomyOverFetch, spec.ids)
	var rows []EntityListRow
	if err := s.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return EntityListPage{}, err
	}
	var more bool
	if paginated {
		rows, more = taxonomyTrim(rows, limit)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	if spec.alias != "" {
		loc, lerr := s.LocalizedForEntities(ctx, spec.alias, ids)
		if lerr != nil {
			return EntityListPage{}, lerr
		}
		for i := range rows {
			rows[i].Localized = loc[rows[i].ID]
		}
	}
	out := EntityListPage{Items: rows, NextCursor: taxonomyNextCursor(spec.lane, ids, more)}
	if out.Total, err = s.taxonomyTotal(ctx, spec.table, filterWhere, filterArgs); err != nil {
		return EntityListPage{}, err
	}
	return out, nil
}

func (s *PublicService) CharacterAttributes(ctx context.Context, id int64) (CharacterAttributes, bool, error) {
	var row CharacterAttributes
	err := s.db.WithContext(ctx).Raw(
		`SELECT gender, birthday_month, birthday_day, blood_type, height_cm, weight_kg,
			bust_cm, waist_cm, hip_cm, cup, instance_of
		 FROM catalog_character WHERE id = ? AND deleted_at IS NULL`, id).Scan(&row).Error
	if err != nil {
		return CharacterAttributes{}, false, err
	}
	var n int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT count(*) FROM catalog_character WHERE id = ? AND deleted_at IS NULL`, id).Scan(&n).Error; err != nil {
		return CharacterAttributes{}, false, err
	}
	return row, n > 0, nil
}

func (s *PublicService) Person(ctx context.Context, id int64) (PersonRow, bool, error) {
	var row PersonRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, display_name, primary_credit_name_id, gender
		 FROM catalog_person WHERE id = ? AND deleted_at IS NULL`, id).Scan(&row).Error
	if err != nil {
		return PersonRow{}, false, err
	}
	if row.ID == 0 {
		return PersonRow{}, false, nil
	}
	return row, true, nil
}

func (s *PublicService) PersonNames(ctx context.Context, personID int64) ([]EntityListRow, bool, error) {
	if _, ok, err := s.Person(ctx, personID); err != nil || !ok {
		return nil, ok, err
	}
	var rows []EntityListRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, name AS display_name, latin, person_id
		 FROM catalog_credit_name WHERE person_id = ? ORDER BY id ASC`, personID).Scan(&rows).Error
	if err != nil {
		return nil, false, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	loc, err := s.LocalizedForEntities(ctx, "name", ids)
	if err != nil {
		return nil, false, err
	}
	for i := range rows {
		rows[i].Localized = loc[rows[i].ID]
	}
	return rows, true, nil
}

func (s *PublicService) Trait(ctx context.Context, id int64) (TraitRow, bool, error) {
	var row TraitRow
	err := s.db.WithContext(ctx).Raw(
		`SELECT id, name AS display_name, name_zh, vndb_tid, sexual, description
		 FROM catalog_character_trait WHERE id = ?`, id).Scan(&row).Error
	if err != nil {
		return TraitRow{}, false, err
	}
	if row.ID == 0 {
		return TraitRow{}, false, nil
	}
	return row, true, nil
}

func NormalizeNameQuery(q string) string {
	return strings.TrimSpace(q)
}
