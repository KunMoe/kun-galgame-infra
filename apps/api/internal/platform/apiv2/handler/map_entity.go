package handler

import (
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
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
