package collect

var WorkInclude = []string{
	"titles", "refs", "relations", "credits", "releases", "popularity",
	"ratings", "tags", "playtimes", "series", "platforms", "intros",
	"covers", "screenshots", "characters", "companies", "engines", "links",
}

var WorkFullSet = []string{
	"titles", "refs", "intros", "covers", "companies", "tags",
	"releases", "ratings", "relations", "engines", "series", "links",
}

var WorkBasicFields = []string{
	"object", "id", "medium", "display_name", "latin", "localized", "olang",
	"content_rating", "release_date", "release_date_precision", "release_status",
	"cover", "banner", "claim", "created_at", "updated_at",
}

var WorkSort = []string{"id", "updated"}

var WorkFacets = []string{
	"tag_id", "company_id", "olang", "content_rating", "medium", "platform",
}

func WorkSpec() Spec {
	fields := append([]string{}, WorkBasicFields...)
	fields = append(fields, WorkInclude...)
	return Spec{
		Sort:    WorkSort,
		Include: WorkInclude,
		FullSet: WorkFullSet,
		Fields:  fields,
		Facets:  WorkFacets,
	}
}

func VocabSpec() Spec {
	return Spec{
		Sort:    []string{"name"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "name", "closed", "values"},
		Facets:  []string{},
	}
}

func ProblemSpec() Spec {
	return Spec{
		Sort:    []string{"code"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "code", "domain", "status", "type", "title", "description"},
		Facets:  []string{},
	}
}

func NewsSpec() Spec {
	return Spec{
		Sort:    []string{"published"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "title", "summary", "source", "source_url", "published_at"},
		Facets:  []string{},
	}
}

func CompanySpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "latin", "localized", "company_kind", "work_count"},
	}
}

func CreditNameSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "latin", "localized", "person_id"},
	}
}

func CharacterSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "latin", "localized"},
	}
}

func TagSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "tier", "tag_kind", "work_count"},
	}
}

func SeriesSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name"},
	}
}

func EngineSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "work_count"},
	}
}
