package repr

type Ref struct {
	_          struct{} `json:"-" additionalProperties:"true"`
	Source     string   `json:"source" maxLength:"64" doc:"Open vocabulary sources. Must not be used as a discriminant."`
	ExternalID string   `json:"external_id" maxLength:"256" doc:"Verbatim upstream id. Must not be used as a discriminant beyond exact match."`
}

type Claim struct {
	_            struct{} `json:"-" additionalProperties:"true"`
	Site         string   `json:"site" maxLength:"64" pattern:"^[a-z][a-z0-9_]*$" doc:"Claiming site key."`
	SiteWorkID   string   `json:"site_work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"The site's own work id, not the catalog id."`
	State        string   `json:"state" enum:"live,draft,pending,declined,hidden" doc:"Claim lifecycle state."`
	ContentLimit string   `json:"content_limit" enum:"sfw,nsfw" doc:"Editorial display axis for this claim."`
}

type List[T any] struct {
	_          struct{}                 `json:"-" additionalProperties:"true"`
	Object     string                   `json:"object" enum:"list" doc:"Type discriminant. Always list."`
	Items      []T                      `json:"items" doc:"Members of this page. Empty array, never null."`
	NextCursor *string                  `json:"next_cursor,omitempty" pattern:"^cur_[A-Za-z0-9._~-]+$" maxLength:"512" doc:"Opaque keyset cursor. Omitted on the last page."`
	Total      *int64                   `json:"total,omitempty" minimum:"0" doc:"Present only when include_total=true. Same visibility gate as items."`
	Missing    *[]string                `json:"missing,omitempty" doc:"ids requested but not visible. Present only on the ids=/refs= batch lane."`
	Facets     *map[string][]FacetValue `json:"facets,omitempty" doc:"Named facet buckets. Present only when facets= is requested."`
}

type FacetValue struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Value       string   `json:"value" maxLength:"256" doc:"Token to pass back to the same filter. Must not be used as a discriminant."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Count       int      `json:"count" minimum:"0" doc:"Hits in this bucket after the same filters as items."`
}

func NewList[T any](items []T, nextCursor *string) List[T] {
	if items == nil {
		items = []T{}
	}
	return List[T]{Object: "list", Items: items, NextCursor: nextCursor}
}

type Work struct {
	_                    struct{}                 `json:"-" additionalProperties:"true"`
	Object               string                   `json:"object" enum:"work" doc:"Type discriminant. Always work."`
	ID                   string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog work id. JSON string of a decimal integer."`
	Medium               string                   `json:"medium" enum:"galgame,manga,novel,anime,asmr,doujin_game,music" doc:"Never null."`
	DisplayName          string                   `json:"display_name" maxLength:"512" doc:"Primary label. Never empty. Must not be used as a discriminant."`
	Latin                *string                  `json:"latin" maxLength:"512" doc:"Romanization. null if unrecorded. Must not be used as a discriminant."`
	Localized            map[string]LocalizedText `json:"localized" doc:"BCP-47 keys, sparse. Empty object if none. Must not be used as a discriminant."`
	OLang                string                   `json:"olang" maxLength:"32" format:"bcp47" doc:"Original language, BCP-47. Open vocabulary languages."`
	ContentRating        string                   `json:"content_rating" enum:"all_ages,sensitive,r18" doc:"Age axis of the work."`
	ReleaseDate          *string                  `json:"release_date" format:"date" maxLength:"10" doc:"Calendar date. null when release_status is announced, cancelled, or unknown."`
	ReleaseDatePrecision *string                  `json:"release_date_precision" enum:"day,month,year" doc:"null when release_date is null. month dates sit on the 1st; year dates sit on January 1."`
	ReleaseStatus        string                   `json:"release_status" enum:"released,dated,announced,cancelled,unknown" doc:"World state of the release, distinct from our knowledge gap."`
	Cover                *Image                   `json:"cover" doc:"Selected portrait image. null if none."`
	Banner               *Image                   `json:"banner" doc:"Selected landscape image. null if none."`
	Claim                *Claim                   `json:"claim" doc:"null if unclaimed. Never omitted on view=basic."`
	CreatedAt            string                   `json:"created_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
	UpdatedAt            string                   `json:"updated_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
}

func NewWork(id int64, medium, display, olang, rating, releaseStatus, created, updated string, latin *string, localized map[string]LocalizedText, releaseDate, releasePrecision *string, cover, banner *Image, claim *Claim) (Work, bool) {
	if _, ok := MediumFromKey(medium); !ok {
		return Work{}, false
	}
	if localized == nil {
		localized = map[string]LocalizedText{}
	}
	return Work{
		Object: "work", ID: ID(id), Medium: medium, DisplayName: display, Latin: latin,
		Localized: localized, OLang: olang, ContentRating: rating,
		ReleaseDate: releaseDate, ReleaseDatePrecision: releasePrecision, ReleaseStatus: releaseStatus,
		Cover: cover, Banner: banner, Claim: claim, CreatedAt: created, UpdatedAt: updated,
	}, true
}
