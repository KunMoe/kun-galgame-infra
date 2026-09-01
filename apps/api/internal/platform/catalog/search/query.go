package search

func IndexForType(t string) (uid string, ok bool) {
	switch t {
	case "names":
		return IndexCreditNames, true
	case "characters":
		return IndexCharacters, true
	case "labels":
		return IndexLabels, true
	case "works":
		return IndexWorks, true
	case "tags":
		return IndexTags, true
	default:
		return "", false
	}
}

func LocalesForUI(uid, locale string) []string {
	if uid == IndexWorks {
		return nil
	}
	switch locale {
	case "zh":
		return []string{"cmn"}
	case "ja":
		return []string{"jpn"}
	default:
		return nil
	}
}

type SearchResult struct {
	Hits  []EntityDoc
	Total int64
}

func (d EntityDoc) Name() string {
	switch {
	case d.NameJa != "":
		return d.NameJa
	case d.NameZh != "":
		return d.NameZh
	default:
		return d.NameOther
	}
}
