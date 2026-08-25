package vocab

type Value struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Value       string   `json:"value" maxLength:"64" doc:"Token as it appears on the wire. Must not be used as a discriminant."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"English label for this token. Must not be used as a discriminant."`
	Description string   `json:"description" maxLength:"512" doc:"English prose. Must not be used as a discriminant."`
}

type Vocabulary struct {
	_      struct{} `json:"-" additionalProperties:"true"`
	Object string   `json:"object" enum:"vocabulary" doc:"Type discriminant. Always vocabulary."`
	Name   string   `json:"name" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Vocabulary name. The path segment of /v2/vocabularies/{name}."`
	Closed bool     `json:"closed" doc:"true if adding or removing a value is a breaking change."`
	Values []Value  `json:"values" doc:"Every value currently published. Empty array, never null."`
}

func All() []Vocabulary {
	out := make([]Vocabulary, len(registry))
	copy(out, registry)
	return out
}

func Lookup(name string) (Vocabulary, bool) {
	for _, v := range registry {
		if v.Name == name {
			return v, true
		}
	}
	return Vocabulary{}, false
}

func closed(name string, values []Value) Vocabulary {
	return Vocabulary{Object: "vocabulary", Name: name, Closed: true, Values: values}
}

func open(name string, values []Value) Vocabulary {
	return Vocabulary{Object: "vocabulary", Name: name, Closed: false, Values: values}
}

func v(value, display, desc string) Value {
	return Value{Value: value, DisplayName: display, Description: desc}
}

var registry = []Vocabulary{
	closed("medium", []Value{
		v("galgame", "Galgame", "A galgame / visual novel work."),
		v("manga", "Manga", "A manga work."),
		v("novel", "Novel", "A light novel or other prose work."),
		v("anime", "Anime", "An anime work."),
		v("asmr", "ASMR", "An ASMR work."),
		v("doujin_game", "Doujin game", "A doujin game that is not classified as galgame."),
		v("music", "Music", "A music work."),
	}),
	closed("content_rating", []Value{
		v("all_ages", "All ages", "Rated suitable for all ages."),
		v("sensitive", "Sensitive", "Sexual or violent content below the r18 line."),
		v("r18", "R18", "Adult-only content."),
	}),
	closed("sexual", []Value{
		v("safe", "Safe", "No sexual depiction."),
		v("suggestive", "Suggestive", "Suggestive sexual depiction."),
		v("explicit", "Explicit", "Explicit sexual depiction."),
	}),
	closed("violence", []Value{
		v("tame", "Tame", "No violent depiction."),
		v("violent", "Violent", "Violent depiction short of brutality."),
		v("brutal", "Brutal", "Brutal violent depiction."),
	}),
	closed("spoiler", []Value{
		v("none", "None", "Not a spoiler."),
		v("minor", "Minor", "Minor spoiler."),
		v("major", "Major", "Major spoiler."),
	}),
	closed("gender", []Value{
		v("male", "Male", "Male."),
		v("female", "Female", "Female."),
		v("other", "Other", "Recorded as other. Absence of a record is null, not a value."),
	}),
	closed("blood_type", []Value{
		v("a", "A", "Blood type A."),
		v("b", "B", "Blood type B."),
		v("ab", "AB", "Blood type AB."),
		v("o", "O", "Blood type O."),
	}),
	closed("release_status", []Value{
		v("released", "Released", "The release has shipped."),
		v("dated", "Dated", "A date is known and the release has not shipped."),
		v("announced", "Announced", "Announced without a usable date."),
		v("cancelled", "Cancelled", "Cancelled."),
		v("unknown", "Unknown", "We have not determined the status."),
	}),
	closed("claim_state", []Value{
		v("live", "Live", "The claim is live."),
		v("draft", "Draft", "The claim is a draft."),
		v("pending", "Pending", "The claim is waiting for review."),
		v("declined", "Declined", "The claim was declined."),
		v("hidden", "Hidden", "The claim is hidden."),
	}),
	closed("content_limit", []Value{
		v("sfw", "SFW", "Safe to render in an SFW context."),
		v("nsfw", "NSFW", "Not safe to render in an SFW context."),
	}),
	closed("tier", []Value{
		v("core", "Core", "Core vocabulary tag."),
		v("longtail", "Longtail", "Long-tail tag."),
		v("hidden", "Hidden", "Hidden from default listings."),
	}),
	closed("title_kind", []Value{
		v("official", "Official", "Official title."),
		v("alias", "Alias", "Alternate title."),
		v("abbreviation", "Abbreviation", "Abbreviated title. search_hint is internal and is not in this vocabulary."),
	}),
	closed("alias_kind", []Value{
		v("translation", "Translation", "A translated name."),
		v("spelling_variant", "Spelling variant", "A spelling variant. search_hint is internal and is not in this vocabulary."),
	}),
	closed("roster_role", []Value{
		v("unknown", "Unknown", "The roster role was not recorded."),
		v("main", "Main", "A main character."),
		v("secondary", "Secondary", "A secondary character."),
		v("appears", "Appears", "A character who appears."),
	}),
	closed("attribution_role", []Value{
		v("circle", "Circle", "Circle / doujin group attribution."),
		v("publisher", "Publisher", "Publisher attribution."),
		v("developer", "Developer", "Developer attribution."),
		v("brand", "Brand", "Brand attribution."),
	}),
	closed("company_kind", []Value{
		v("game_brand", "Game brand", "A game brand."),
		v("bunko", "Bunko", "A bunko / imprint."),
		v("publisher", "Publisher", "A publisher."),
		v("anime_studio", "Anime studio", "An anime studio."),
		v("doujin_circle", "Doujin circle", "A doujin circle."),
		v("group", "Group", "A group."),
	}),
	closed("tag_kind", []Value{
		v("content", "Content", "A content tag."),
		v("meta", "Meta", "A meta tag."),
	}),
	closed("release_kind", []Value{
		v("default", "Default", "The default / unspecified edition."),
		v("digital", "Digital", "A digital edition."),
		v("physical", "Physical", "A physical edition."),
		v("trial", "Trial", "A trial / demo edition."),
		v("patch", "Patch", "A patch, including fan translations."),
	}),
	closed("member_kind", []Value{
		v("unknown", "Unknown", "The series-member role was not recorded."),
		v("main", "Main", "The main entry of the series."),
		v("fandisc", "Fandisc", "A fandisc."),
		v("side_story", "Side story", "A side story."),
		v("collection", "Collection", "A collection / omnibus."),
	}),
	closed("problem_domain", []Value{
		v("platform", "Platform", "Errors any face may emit."),
		v("catalog", "Catalog", "Catalog-face errors."),
		v("me", "Me", "User-facing /v2/me errors."),
		v("moderation", "Moderation", "Moderation-face errors."),
		v("news", "News", "News-face errors. Both codes come from the source-row-as-grant model on /v2/me/news."),
	}),
	open("sources", []Value{
		v("user", "User", "Manual curation, not an import source."),
		v("vndb", "VNDB", "The VNDB catalog."),
		v("bangumi", "Bangumi", "The Bangumi catalog."),
		v("dlsite", "DLsite", "DLsite storefront identities."),
		v("erogamescape", "ErogameScape", "ErogameScape."),
		v("anilist", "AniList", "AniList."),
		v("mal", "MyAnimeList", "MyAnimeList."),
		v("steam", "Steam", "Steam storefront."),
		v("official_site", "Official site", "A publisher or brand official site."),
		v("twitter", "Twitter / X", "A Twitter / X identity."),
		v("pixiv", "Pixiv", "Pixiv."),
		v("curated", "Curated", "First-party curated / human lane."),
		v("upscale", "Upscale", "First-party AI-upscaled cover derivation."),
		v("cien", "Ci-en", "Ci-en creator-support platform."),
		v("dmm", "DMM", "DMM storefront."),
		v("web", "Web", "A generic web page. external_id is the full URL."),
		v("getchu", "Getchu", "Getchu.com retailer pages."),
		v("derived", "Derived", "First-party machine inference over catalog facts."),
		v("nextmoe", "NextMoe", "First-party measurements aggregated from our users."),
	}),
	open("relation_types", []Value{
		v("adaptation_of", "Adaptation of", "This work is an adaptation of the other."),
		v("sequel_of", "Sequel of", "This work is a sequel of the other."),
		v("side_story_of", "Side story of", "This work is a side story of the other."),
		v("fandisc_of", "Fandisc of", "This work is a fandisc of the other."),
		v("collects", "Collects", "This work collects the other."),
		v("remake_of", "Remake of", "This work is a remake of the other."),
		v("same_series", "Same series", "The two works belong to the same series. Symmetric."),
		v("same_setting", "Same setting", "The two works share a setting. Symmetric."),
		v("crossover_with", "Crossover with", "The two works crossover. Symmetric."),
		v("shares_character", "Shares character", "The two works share a character. Symmetric."),
		v("alternative_setting", "Alternative setting", "The two works share characters in a different setting. Symmetric."),
		v("alternative_version", "Alternative version", "The two works are alternative versions. Symmetric."),
		v("imprint_of", "Imprint of", "This company is an imprint of the other."),
		v("renamed_from", "Renamed from", "This company was renamed from the other."),
		v("subsidiary_of", "Subsidiary of", "This company is a subsidiary of the other."),
		v("member_of", "Member of", "This company is a member of the other."),
	}),
}
