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
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "latin", "localized", "company_kind", "work_count"},
	}
}

func CreditNameSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "latin", "localized", "person_id"},
	}
}

func CharacterSpec() Spec {
	full := []string{"gender", "birthday", "height_cm", "weight_kg", "measurements", "blood_type", "instance_of_id"}
	fields := []string{"object", "id", "display_name", "latin", "localized"}
	fields = append(fields, full...)
	return Spec{
		Sort:    []string{"id"},
		Include: append([]string{}, full...),
		FullSet: append([]string{}, full...),
		Fields:  fields,
	}
}

func PersonSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "primary_credit_name_id", "gender"},
	}
}

func TraitSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "name_zh", "vndb_tid", "is_sexual"},
	}
}

func PlaytimeSpec() Spec {
	return Spec{
		Sort:    []string{"updated"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "work_id", "minutes"},
	}
}

func CoverVoteSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "cover_id", "work_id", "vote"},
	}
}

func NameCreditSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "work", "roles"},
	}
}

func TagSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "tier", "tag_kind", "work_count"},
	}
}

func SeriesSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name"},
	}
}

func EngineSpec() Spec {
	return Spec{
		Sort:    []string{"id"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "display_name", "work_count"},
	}
}

func WorkSubSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id"},
	}
}

func ReleaseSpec() Spec {
	return Spec{
		Sort:    []string{"date_desc", "date_asc"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "id", "work_id", "release_kind", "date", "title", "lang", "platform", "platforms", "refs"},
	}
}

func ChangesSpec() Spec {
	return Spec{
		Sort:    []string{"updated"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "target_object", "id", "updated_at"},
	}
}

func RedirectsSpec() Spec {
	return Spec{
		Sort:    []string{"merged_at"},
		Include: []string{},
		FullSet: []string{},
		Fields:  []string{"object", "target_object", "old_id", "current_id", "merged_at"},
	}
}

func SearchSpec() Spec {
	return Spec{
		Include: []string{},
		FullSet: []string{},
		Fields: []string{
			"object", "target_object", "id", "display_name", "latin", "localized",
			"sources", "content_rating", "tier", "tag_kind",
		},
	}
}

func CalendarSpec() Spec {
	fields := append([]string{}, WorkBasicFields...)
	fields = append(fields, WorkInclude...)
	return Spec{
		Sort:    []string{"id"},
		Include: WorkInclude,
		FullSet: WorkFullSet,
		Fields:  fields,
	}
}

func ObjectSpec(object string) (Spec, bool) {
	switch object {
	case "work":
		return WorkSpec(), true
	case "company":
		return CompanySpec(), true
	case "character":
		return CharacterSpec(), true
	case "release":
		return ReleaseSpec(), true
	case "tag":
		return TagSpec(), true
	case "engine":
		return EngineSpec(), true
	case "series":
		return SeriesSpec(), true
	case "person":
		return PersonSpec(), true
	case "trait":
		return TraitSpec(), true
	default:
		return Spec{}, false
	}
}
