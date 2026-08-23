package handler

import (
	"fmt"

	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
	catsvc "api/internal/platform/catalog/service"
)

func companyFromListItem(it dto.PublicLabelListItem) repr.Company {
	kind := it.Kind
	if _, ok := repr.CompanyKindFromKey(kind); !ok {
		kind = "group"
	}
	return repr.Company{
		Object: "company", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		Localized: localizedFrom(it.Localized), CompanyKind: kind, WorkCount: it.WorkCount,
	}
}

func tagFromListItem(it dto.PublicTagListItem) repr.Tag {
	return repr.Tag{
		Object: "tag", ID: repr.ID(it.ID), DisplayName: it.Name,
		Tier: it.Tier, TagKind: it.Kind, WorkCount: it.WorkCount,
	}
}

func engineFromListItem(it dto.PublicEngineListItem) repr.Engine {
	return repr.Engine{Object: "engine", ID: repr.ID(it.ID), DisplayName: it.Name, WorkCount: it.WorkCount}
}

func seriesFromDetail(id int64, display string) repr.Series {
	return repr.Series{Object: "series", ID: repr.ID(id), DisplayName: display}
}

func characterFromRow(it catsvc.EntityListRow) repr.Character {
	return repr.Character{
		Object: "character", ID: repr.ID(it.ID), DisplayName: it.DisplayName,
		Latin: it.Latin, Localized: localizedFrom(it.Localized),
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
		Latin: it.Latin, Localized: localizedFrom(it.Localized), PersonID: personID,
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
