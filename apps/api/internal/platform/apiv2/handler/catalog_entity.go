package handler

import (
	"context"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catmodel "api/internal/platform/catalog/model"
	catsvc "api/internal/platform/catalog/service"
)

func (c *Catalog) GetCreditName(ctx context.Context, id int64, nsfw bool) (repr.CreditName, error) {
	if c == nil || c.Public == nil {
		return repr.CreditName{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.Name(ctx, id, false, nsfw, 0, 0)
	if err != nil {
		return repr.CreditName{}, err
	}
	if !found {
		return repr.CreditName{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeCreditName, "credit_name", id)
	}
	var personID *string
	if rec.PersonID > 0 {
		s := repr.ID(rec.PersonID)
		personID = &s
	}
	return repr.CreditName{
		Object: "credit_name", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName,
		Latin: optString(rec.Latin), Localized: localizedFrom(rec.Localized), PersonID: personID,
	}, nil
}

func (c *Catalog) GetCharacter(ctx context.Context, id int64, nsfw bool) (repr.Character, error) {
	if c == nil || c.Public == nil {
		return repr.Character{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.Character(ctx, id, false, nsfw, 0, 0, 0)
	if err != nil {
		return repr.Character{}, err
	}
	if !found {
		return repr.Character{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeCharacter, "character", id)
	}
	return repr.Character{
		Object: "character", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName,
		Latin: optString(rec.Latin), Localized: localizedFrom(rec.Localized),
	}, nil
}

func (c *Catalog) GetCompany(ctx context.Context, id int64, nsfw bool) (repr.Company, error) {
	if c == nil || c.Public == nil {
		return repr.Company{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.Label(ctx, id, false, nsfw, 0, 0)
	if err != nil {
		return repr.Company{}, err
	}
	if !found {
		return repr.Company{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeLabel, "company", id)
	}
	kind := rec.Kind
	if _, ok := repr.CompanyKindFromKey(kind); !ok {
		kind = "group"
	}
	return repr.Company{
		Object: "company", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName,
		Localized: localizedFrom(rec.Localized), CompanyKind: kind, WorkCount: rec.WorkCount,
	}, nil
}

func (c *Catalog) GetTag(ctx context.Context, id int64, nsfw bool) (repr.Tag, error) {
	if c == nil || c.Public == nil {
		return repr.Tag{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.TagDetail(ctx, id, false, nsfw, 0, 0)
	if err != nil {
		return repr.Tag{}, err
	}
	if !found {
		return repr.Tag{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeTag, "tag", id)
	}
	return repr.Tag{
		Object: "tag", ID: repr.ID(rec.ID), DisplayName: rec.Name,
		Tier: rec.Tier, TagKind: rec.Kind, WorkCount: rec.WorkCount,
	}, nil
}

func (c *Catalog) GetEngine(ctx context.Context, id int64, nsfw bool) (repr.Engine, error) {
	if c == nil || c.Public == nil {
		return repr.Engine{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.EngineDetail(ctx, id, nsfw)
	if err != nil {
		return repr.Engine{}, err
	}
	if !found {
		return repr.Engine{}, problem.New(problem.CodeNotFound, "", "", "engine not found.")
	}
	return repr.Engine{Object: "engine", ID: repr.ID(rec.ID), DisplayName: rec.Name, WorkCount: rec.WorkCount}, nil
}

func (c *Catalog) GetSeries(ctx context.Context, id int64, nsfw bool) (repr.Series, error) {
	if c == nil || c.Public == nil {
		return repr.Series{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	rec, found, err := c.Public.SeriesDetail(ctx, id, false, nsfw, 0, 0)
	if err != nil {
		return repr.Series{}, err
	}
	if !found {
		return repr.Series{}, problem.New(problem.CodeNotFound, "", "", "series not found.")
	}
	return repr.Series{Object: "series", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName}, nil
}

func (c *Catalog) GetRelease(ctx context.Context, id int64, nsfw bool) (repr.Release, error) {
	if c == nil || c.Public == nil {
		return repr.Release{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	f := catsvc.ReleaseFeedFilter{NSFW: nsfw, IDs: []int64{id}, Kinds: releaseFeedKinds()}
	data, err := c.Public.ReleaseFeed(ctx, f, "", 1)
	if err != nil {
		return repr.Release{}, err
	}
	if len(data.Items) == 0 {
		return repr.Release{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeRelease, "release", id)
	}
	return releaseFromFeed(data.Items[0]), nil
}
