package repr

type Tag struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"tag" doc:"Type discriminant. Always tag."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog tag id."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Tier        string   `json:"tier" enum:"core,longtail,hidden" doc:"Tag inventory tier."`
	TagKind     string   `json:"tag_kind" enum:"content,meta" doc:"Tag vocabulary class."`
	WorkCount   int      `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate."`
}

type Company struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"company" doc:"Type discriminant. Always company."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog company id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin       *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	CompanyKind string                   `json:"company_kind" enum:"game_brand,bunko,publisher,anime_studio,doujin_circle,group" doc:"Company registry class. No other."`
	WorkCount   int                      `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate."`
}

type CreditName struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"credit_name" doc:"Type discriminant. Always credit_name."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog credit-name id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin       *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	PersonID    *string                  `json:"person_id" pattern:"^[0-9]+$" maxLength:"20" doc:"null if this name is not linked to a person."`
}

type Character struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"character" doc:"Type discriminant. Always character."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog character id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin       *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
}

type Series struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"series" doc:"Type discriminant. Always series."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog series id."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
}

type Engine struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"engine" doc:"Type discriminant. Always engine."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog engine id."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	WorkCount   int      `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate."`
}

type NewsSource struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"news_source" doc:"Type discriminant. Always news_source."`
	Name        string   `json:"name" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Source key."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
}

type NewsItem struct {
	_           struct{}   `json:"-" additionalProperties:"true"`
	Object      string     `json:"object" enum:"news_item" doc:"Type discriminant. Always news_item."`
	ID          string     `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"News item id."`
	Title       string     `json:"title" maxLength:"512" doc:"Must not be used as a discriminant."`
	Summary     string     `json:"summary" maxLength:"8000" doc:"Source-provided lede. Must not be used as a discriminant."`
	Source      NewsSource `json:"source" doc:"Attribution. Required on view=basic."`
	SourceURL   string     `json:"source_url" format:"uri" maxLength:"1024" doc:"Canonical link to the original item."`
	PublishedAt string     `json:"published_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
}

type CatalogStats struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"catalog_stats" doc:"Type discriminant. Always catalog_stats."`
	Works       int64    `json:"works" minimum:"0" doc:"Live works."`
	Labels      int64    `json:"companies" minimum:"0" doc:"Company registry rows."`
	Characters  int64    `json:"characters" minimum:"0" doc:"Live characters."`
	CreditNames int64    `json:"credit_names" minimum:"0" doc:"Live credit names."`
	Persons     int64    `json:"persons" minimum:"0" doc:"Live persons."`
}
