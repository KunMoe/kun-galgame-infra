package repr

type Screenshot struct {
	_         struct{} `json:"-" additionalProperties:"true"`
	URL       string   `json:"url" format:"uri" maxLength:"512" doc:"Absolute image URL. Never a bare hash."`
	Hash      string   `json:"hash" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$" doc:"Image-service content hash."`
	Width     *int     `json:"width" minimum:"0" maximum:"65535" doc:"Pixel width. null if unknown."`
	Height    *int     `json:"height" minimum:"0" maximum:"65535" doc:"Pixel height. null if unknown."`
	Thumbhash *string  `json:"thumbhash" maxLength:"128" pattern:"^[A-Za-z0-9+/=_-]+$" doc:"Thumbhash. null if unknown."`
	Sexual    *string  `json:"sexual" enum:"safe,suggestive,explicit" doc:"Sexual depiction. null means not assessed."`
	Violence  *string  `json:"violence" enum:"tame,violent,brutal" doc:"Violent depiction. null means not assessed. Currently always null."`
	Source    string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	Caption   string   `json:"caption" maxLength:"2048" doc:"Must not be used as a discriminant."`
}

type WorkTag struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	ID          *string  `json:"id" pattern:"^[0-9]+$" maxLength:"20" doc:"Canonical tag id. null if this row is not mapped."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Source      string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	Tier        *string  `json:"tier" enum:"core,longtail,hidden" doc:"Tag inventory tier. null if unmapped."`
	TagKind     *string  `json:"tag_kind" enum:"content,meta" doc:"Tag vocabulary class. null if unmapped."`
	Spoiler     string   `json:"spoiler" enum:"none,minor,major" doc:"Spoiler level of this attachment."`
	IsSexual    bool     `json:"is_sexual" doc:"Whether this tag is in the sexual family."`
	WorkCount   *int     `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate. null if unmapped."`
}

type WorkCharacter struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"character" doc:"Type discriminant. Always character."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog character id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin       *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	RosterRole  string                   `json:"roster_role" enum:"main,secondary,appears,unknown" doc:"Appearance strength on this work."`
	Spoiler     string                   `json:"spoiler" enum:"none,minor,major" doc:"Spoiler level of this appearance."`
	Image       *Image                   `json:"image" doc:"Character image. null if none."`
	Figure      *Image                   `json:"figure" doc:"Figure image. null if none."`
}

type CreditGroup struct {
	_        struct{}      `json:"-" additionalProperties:"true"`
	RoleKey  string        `json:"role_key" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Credit role token."`
	RoleName string        `json:"role_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Credits  []CreditEntry `json:"credits" doc:"Names credited in this role. Empty array, never null."`
}

type CreditEntry struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"credit_name" doc:"Type discriminant. Always credit_name."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog credit-name id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Latin       *string                  `json:"latin" maxLength:"512" doc:"null if unrecorded. Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	CharacterID *string                  `json:"character_id" pattern:"^[0-9]+$" maxLength:"20" doc:"null if this credit is not a voice on a character."`
}

type Release struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"release" doc:"Type discriminant. Always release."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog release id."`
	ReleaseKind string   `json:"release_kind" enum:"default,digital,physical,trial,patch" doc:"Release class."`
	Date        *string  `json:"date" format:"date" maxLength:"10" doc:"Calendar date. null if undated."`
	Title       *string  `json:"title" maxLength:"512" doc:"Must not be used as a discriminant."`
	Lang        string   `json:"lang" maxLength:"32" format:"bcp47" doc:"BCP-47. Empty if unrecorded."`
	Platform    string   `json:"platform" maxLength:"64" doc:"Must not be used as a discriminant."`
	Platforms   []string `json:"platforms" doc:"Every platform on this release. Empty array, never null."`
	Refs        []Ref    `json:"refs" doc:"Exact upstream anchors. Empty array, never null."`
}

type Relation struct {
	_            struct{} `json:"-" additionalProperties:"true"`
	RelationType string   `json:"relation_type" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Relation token."`
	Phrase       string   `json:"phrase" maxLength:"512" doc:"Must not be used as a discriminant."`
	Work         Work     `json:"work" doc:"Related work. Same type as the work collection. view=basic."`
}

type WorkCompany struct {
	_               struct{}                 `json:"-" additionalProperties:"true"`
	Object          string                   `json:"object" enum:"company" doc:"Type discriminant. Always company."`
	ID              string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog company id."`
	DisplayName     string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Localized       map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	CompanyKind     string                   `json:"company_kind" enum:"game_brand,bunko,publisher,anime_studio,doujin_circle,group" doc:"Company registry class."`
	AttributionRole string                   `json:"attribution_role" enum:"circle,publisher,developer,brand" doc:"Primary capacity on this work."`
	WorkCount       int                      `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate."`
}

type WorkLink struct {
	_      struct{} `json:"-" additionalProperties:"true"`
	Source string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	URL    string   `json:"url" format:"uri" maxLength:"1024" doc:"Absolute URL."`
}

type Rating struct {
	_         struct{} `json:"-" additionalProperties:"true"`
	Source    string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	Score     float64  `json:"score" minimum:"0" doc:"Source-native aggregate score."`
	VoteCount int      `json:"vote_count" minimum:"0" doc:"Votes backing score."`
	Rank      *int     `json:"rank" minimum:"0" doc:"Source rank. null if unrecorded."`
}

type Popularity struct {
	_      struct{} `json:"-" additionalProperties:"true"`
	Source string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	Metric string   `json:"metric" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Source-native metric token."`
	Value  int64    `json:"value" minimum:"0" doc:"Metric value."`
}

type Playtime struct {
	_         struct{} `json:"-" additionalProperties:"true"`
	Source    string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	Minutes   int      `json:"minutes" minimum:"0" doc:"Estimated minutes."`
	VoteCount int      `json:"vote_count" minimum:"0" doc:"Votes backing the estimate."`
}

type WorkPlatform struct {
	_        struct{} `json:"-" additionalProperties:"true"`
	Platform string   `json:"platform" maxLength:"64" doc:"Must not be used as a discriminant."`
	Source   string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
}

type WorkSeriesRef struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"series" doc:"Type discriminant. Always series."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog series id."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Source      string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	MemberCount int      `json:"member_count" minimum:"0" doc:"Members visible under the same NSFW gate."`
}
