package handler

import (
	"fmt"

	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
	catsvc "api/internal/platform/catalog/service"
)

func companyFromListItem(it dto.PublicLabelListItem, include []string, logoURL string) repr.Company {
	kind := it.Kind
	if _, ok := repr.CompanyKindFromKey(kind); !ok {
		kind = "group"
	}
	out := repr.Company{
		Object: "company", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		Latin: optString(it.Latin), Lang: optString(it.Lang), Localized: localizedFrom(it.Localized),
		CompanyKind: kind, WorkCount: it.WorkCount,
	}
	for _, t := range include {
		switch t {
		case "aliases":
			out.Aliases = ptrSlice(entityNamesFrom(it.Aliases))
		case "logo":
			if logoURL != "" {
				out.Logo = imageFromPublicMeta(logoURL, it.LogoMeta, "")
			}
		}
	}
	return out
}

func companyFromDetail(rec dto.PublicLabel, include []string, logoURL string) repr.Company {
	kind := rec.Kind
	if _, ok := repr.CompanyKindFromKey(kind); !ok {
		kind = "group"
	}
	out := repr.Company{
		Object: "company", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName,
		Latin: optString(rec.Latin), Lang: optString(rec.Lang), Localized: localizedFrom(rec.Localized),
		CompanyKind: kind, WorkCount: rec.WorkCount,
	}
	for _, t := range include {
		switch t {
		case "aliases":
			out.Aliases = ptrSlice(entityNamesFrom(rec.Aliases))
		case "logo":
			if logoURL != "" {
				out.Logo = imageFromPublicMeta(logoURL, rec.LogoMeta, "")
			}
		case "intros":
			out.Intros = ptrSlice(introsFrom(rec.Intros))
		case "links":
			out.Links = ptrSlice(labelLinksFrom(rec.Links))
		}
	}
	return out
}

func entityNamesFrom(in []dto.PublicAlias) []repr.EntityName {
	out := make([]repr.EntityName, 0, len(in))
	for _, a := range in {
		kind := a.Kind
		if kind != "translation" {
			kind = "spelling_variant"
		}
		out = append(out, repr.EntityName{Lang: a.Lang, Value: a.Value, AliasKind: kind, IsMachine: a.Machine})
	}
	return out
}

func labelLinksFrom(in []dto.PublicLabelLink) []repr.WorkLink {
	out := make([]repr.WorkLink, 0, len(in))
	for _, l := range in {
		out = append(out, repr.WorkLink{Source: l.Source, URL: l.URL})
	}
	return out
}

func tagFromListItem(it dto.PublicTagListItem) repr.Tag {
	return repr.Tag{
		Object: "tag", ID: repr.ID(it.ID), DisplayName: it.Name,
		Tier: it.Tier, TagKind: it.Kind, WorkCount: it.WorkCount, IsSexual: it.Sexual,
	}
}

func engineFromListItem(it dto.PublicEngineListItem) repr.Engine {
	aliases := it.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	return repr.Engine{
		Object: "engine", ID: repr.ID(it.ID), DisplayName: it.Name, WorkCount: it.WorkCount,
		Description: it.Description, Aliases: aliases,
	}
}

func tagFromDetail(rec dto.PublicTagDetail, include []string) repr.Tag {
	out := repr.Tag{
		Object: "tag", ID: repr.ID(rec.ID), DisplayName: rec.Name,
		Tier: rec.Tier, TagKind: rec.Kind, WorkCount: rec.WorkCount, IsSexual: rec.Sexual,
	}
	for _, t := range include {
		if t == "intros" {
			out.Intros = ptrSlice(introsFrom(rec.Intros))
		}
	}
	return out
}

func seriesFromDetail(rec dto.PublicSeriesDetail, include []string) repr.Series {
	out := repr.Series{
		Object: "series", ID: repr.ID(rec.ID), DisplayName: rec.DisplayName,
		WorkCount: rec.WorkCount, HasNSFW: rec.HasNSFW,
	}
	for _, t := range include {
		switch t {
		case "intros":
			out.Intros = ptrSlice(seriesIntrosFrom(rec.Intros))
		case "refs":
			out.Refs = ptrSlice(refsFrom(rec.Refs))
		}
	}
	return out
}

func seriesFromListItem(it dto.PublicSeriesListItem) repr.Series {
	return repr.Series{
		Object: "series", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		WorkCount: it.WorkCount, HasNSFW: it.HasNSFW,
	}
}

func seriesIntrosFrom(in []dto.PublicSeriesIntro) []repr.Intro {
	out := make([]repr.Intro, 0, len(in))
	for _, s := range in {
		out = append(out, repr.Intro{Lang: s.Lang, Value: s.Intro, Source: s.Source})
	}
	return out
}

func characterFromRow(it catsvc.EntityListRow) repr.Character {
	return repr.Character{
		Object: "character", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		Latin: it.Latin, Lang: optString(it.Lang), Localized: localizedFrom(it.Localized),
	}
}

func creditNameFromRow(it catsvc.EntityListRow) repr.CreditName {
	var personID *string
	if it.PersonID != nil && *it.PersonID > 0 {
		s := repr.ID(*it.PersonID)
		personID = &s
	}
	return repr.CreditName{
		Object: "credit_name", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		Latin: it.Latin, Lang: optString(it.Lang), Localized: localizedFrom(it.Localized),
		PersonID: personID,
	}
}

func personFromRow(it catsvc.EntityListRow) repr.Person {
	return repr.Person{Object: "person", ID: repr.ID(it.ID), DisplayName: it.DisplayName}
}

func traitFromRow(it catsvc.EntityListRow) repr.Trait {
	return repr.Trait{
		Object: "trait", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		NameZh: it.NameZh, VndbTID: it.VndbTID, IsSexual: it.Sexual,
	}
}

func attachCharacterBlocks(out *repr.Character, rec dto.PublicCharacter, include []string) {
	for _, t := range include {
		switch t {
		case "image":
			if rec.Image != "" {
				out.Image = imageFromPublicMeta(rec.Image, rec.ImageMeta, "")
			}
		case "figure":
			if rec.Figure != "" {
				out.Figure = imageFromPublicMeta(rec.Figure, rec.FigureMeta, "")
			}
		case "traits":
			out.Traits = ptrSlice(characterTraitsFrom(rec.Traits))
		case "aliases":
			out.Aliases = ptrSlice(entityNamesFrom(rec.Aliases))
		case "intros":
			out.Intros = ptrSlice(introsFrom(rec.Intros))
		case "refs":
			out.Refs = ptrSlice(refsFrom(rec.Refs))
		}
	}
}

func characterTraitsFrom(in []dto.PublicCharacterTrait) []repr.CharacterTrait {
	out := make([]repr.CharacterTrait, 0, len(in))
	for _, tr := range in {
		sp, ok := repr.Spoiler(tr.Spoiler)
		if !ok {
			sp = "none"
		}
		out = append(out, repr.CharacterTrait{
			Object: "character_trait", ID: repr.ID(tr.ID), DisplayName: tr.Name,
			Group: optString(tr.Group), Localized: localizedFrom(tr.Localized),
			GroupLocalized: localizedFrom(tr.GroupLocalized),
			Spoiler:        sp, IsSexual: tr.Sexual, IsLie: tr.Lie,
		})
	}
	return out
}

func characterWantsAttrs(include []string) bool {
	for _, t := range include {
		switch t {
		case "gender", "birthday", "height_cm", "weight_kg", "measurements", "blood_type", "instance_of_id":
			return true
		}
	}
	return false
}

func attachCharacterAttrs(out *repr.Character, a catsvc.CharacterAttributes) {
	g, ok := repr.Gender(a.Gender)
	if ok {
		out.Gender = g
	}
	out.BloodType, _ = repr.BloodType(a.BloodType)
	out.HeightCm = intFromI16(a.HeightCm)
	out.WeightKg = intFromI16(a.WeightKg)
	if a.BirthdayMonth != nil && a.BirthdayDay != nil && *a.BirthdayMonth > 0 && *a.BirthdayDay > 0 {
		s := fmt.Sprintf("%02d-%02d", *a.BirthdayMonth, *a.BirthdayDay)
		out.Birthday = &s
	}
	if a.InstanceOf != nil && *a.InstanceOf > 0 {
		s := repr.ID(*a.InstanceOf)
		out.InstanceOfID = &s
	}
	m := repr.Measurements{
		BustCm: intFromI16(a.BustCm), WaistCm: intFromI16(a.WaistCm),
		HipCm: intFromI16(a.HipCm), Cup: a.Cup,
	}
	if m.BustCm != nil || m.WaistCm != nil || m.HipCm != nil || m.Cup != nil {
		out.Measurements = &m
	}
}

func intFromI16(v *int16) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}
