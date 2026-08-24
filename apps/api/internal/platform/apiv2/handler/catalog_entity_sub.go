package handler

import (
	"context"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catmodel "api/internal/platform/catalog/model"
)

func (c *Catalog) CreditNameCredits(ctx context.Context, id int64, nsfw bool, cursor string, limit int) (repr.List[repr.NameCredit], error) {
	offset, limit, err := c.subPage(cursor, limit)
	if err != nil {
		return repr.List[repr.NameCredit]{}, err
	}
	if c == nil || c.Public == nil {
		return repr.List[repr.NameCredit]{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.Name(ctx, id, true, nsfw, limit, offset)
	if err != nil {
		return repr.List[repr.NameCredit]{}, err
	}
	if !found {
		return repr.List[repr.NameCredit]{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeCreditName, "credit_name", id)
	}
	items := make([]repr.NameCredit, 0, len(rec.Credits))
	for _, row := range rec.Credits {
		roles := make([]repr.NameCreditRole, 0, len(row.Roles))
		for _, r := range row.Roles {
			var ch *string
			if r.CharacterID > 0 {
				s := repr.ID(r.CharacterID)
				ch = &s
			}
			roles = append(roles, repr.NameCreditRole{
				RoleKey: r.RoleKey, RoleName: r.RoleName, CharacterID: ch,
			})
		}
		items = append(items, repr.NameCredit{Object: "name_credit", Work: workFromBrief(row.Work), Roles: roles})
	}
	return repr.NewList(items, collect.EncodeOffset(derefInt(rec.NextOffset))), nil
}

func (c *Catalog) PersonCreditNames(ctx context.Context, id int64) (repr.List[repr.CreditName], error) {
	if c == nil || c.Public == nil {
		return repr.List[repr.CreditName]{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rows, found, err := c.Public.PersonNames(ctx, id)
	if err != nil {
		return repr.List[repr.CreditName]{}, err
	}
	if !found {
		return repr.List[repr.CreditName]{}, c.mergedOrNotFound(ctx, catmodel.EntityTypePerson, "person", id)
	}
	items := make([]repr.CreditName, 0, len(rows))
	for _, it := range rows {
		items = append(items, creditNameFromRow(it))
	}
	return finishList(items, nil, int64(len(items)), collect.Query{}, nil), nil
}
