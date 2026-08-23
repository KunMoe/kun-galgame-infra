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
