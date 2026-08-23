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
	_            struct{}                 `json:"-" additionalProperties:"true"`
	Object       string                   `json:"object" enum:"character" doc:"Type discriminant. Always character."`
	ID           string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog character id."`
	DisplayName  string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin        *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized    map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	Gender       *string                  `json:"gender,omitempty" enum:"male,female,other" doc:"Present on view=full. null if unrecorded."`
	Birthday     *string                  `json:"birthday,omitempty" pattern:"^[0-1][0-9]-[0-3][0-9]$" maxLength:"5" doc:"MM-DD. Present on view=full. null if unrecorded. Not a date: there is no year."`
	HeightCm     *int                     `json:"height_cm,omitempty" minimum:"0" doc:"Present on view=full. null if unrecorded."`
	WeightKg     *int                     `json:"weight_kg,omitempty" minimum:"0" doc:"Present on view=full. null if unrecorded."`
	Measurements *Measurements            `json:"measurements,omitempty" doc:"Present on view=full. null if unrecorded."`
	BloodType    *string                  `json:"blood_type,omitempty" enum:"a,b,ab,o" doc:"Present on view=full. null if unrecorded."`
	InstanceOfID *string                  `json:"instance_of_id,omitempty" pattern:"^[0-9]+$" maxLength:"20" doc:"Another character this row is an instance of. Present on view=full. null if none."`
}

type Measurements struct {
	_       struct{} `json:"-" additionalProperties:"true"`
	BustCm  *int     `json:"bust_cm" minimum:"0" doc:"null if unrecorded."`
	WaistCm *int     `json:"waist_cm" minimum:"0" doc:"null if unrecorded."`
	HipCm   *int     `json:"hip_cm" minimum:"0" doc:"null if unrecorded."`
	Cup     *string  `json:"cup" maxLength:"8" doc:"Must not be used as a discriminant. null if unrecorded."`
}

type Person struct {
	_                   struct{} `json:"-" additionalProperties:"true"`
	Object              string   `json:"object" enum:"person" doc:"Type discriminant. Always person."`
	ID                  string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog person id."`
	DisplayName         string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	PrimaryCreditNameID *string  `json:"primary_credit_name_id" pattern:"^[0-9]+$" maxLength:"20" doc:"Primary credited name. null if unset."`
	Gender              *string  `json:"gender" enum:"male,female,other" doc:"null if unrecorded."`
}

type Trait struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"trait" doc:"Type discriminant. Always trait."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog trait id."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	NameZh      string   `json:"name_zh" maxLength:"512" doc:"Must not be used as a discriminant. Empty if unrecorded."`
	VndbTID     string   `json:"vndb_tid" maxLength:"32" doc:"Upstream VNDB trait id. Must not be used as a discriminant."`
	IsSexual    bool     `json:"is_sexual" doc:"Whether this trait is in the sexual family."`
}

type NameCredit struct {
	_      struct{}         `json:"-" additionalProperties:"true"`
	Object string           `json:"object" enum:"name_credit" doc:"Type discriminant. Always name_credit."`
	Work   Work             `json:"work" doc:"Work this name is credited on. view=basic."`
	Roles  []NameCreditRole `json:"roles" doc:"Roles on this work. Empty array, never null."`
}

type NameCreditRole struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	RoleKey     string   `json:"role_key" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Credit role token."`
	RoleName    string   `json:"role_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	CharacterID *string  `json:"character_id" pattern:"^[0-9]+$" maxLength:"20" doc:"null if this credit is not a voice on a character."`
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
