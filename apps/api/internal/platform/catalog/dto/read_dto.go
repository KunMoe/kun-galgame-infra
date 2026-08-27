package dto

type WorkByAnchorResponse struct {
	Work           WorkCore            `json:"work"`
	Titles         []WorkTitle         `json:"titles"`
	Releases       []ReleaseBrief      `json:"releases"`
	Labels         []WorkLabel         `json:"labels"`
	Refs           []WorkRef           `json:"refs"`
	Characters     []WorkCharacter     `json:"characters"`
	Intro          []WorkIntro         `json:"intro"`
	Covers         []WorkCover         `json:"covers"`
	Screenshots    []WorkScreenshot    `json:"screenshots"`
	Ratings        []WorkRating        `json:"ratings"`
	Tags           []WorkTag           `json:"tags"`
	Popularity     []WorkPopularity    `json:"popularity"`
	Playtimes      []WorkPlaytime      `json:"playtimes"`
	Series         []WorkSeries        `json:"series"`
	Platforms      []WorkPlatform      `json:"platforms"`
	Relations      []WorkRelation      `json:"relations"`
	SeriesSiblings []WorkSeriesSibling `json:"series_siblings"`
}

type WorkPlatform struct {
	Platform string `json:"platform" doc:"catalog_platform registry code (win/and/ios/…)"`
	SourceID int16  `json:"source_id" doc:"catalog_source id that asserted it"`
}

type WorkSeries struct {
	ID          int64  `json:"id"`
	Name        string `json:"name" doc:"series display name, verbatim from the source (dlsite series_name)"`
	SourceID    int16  `json:"source_id" doc:"catalog_source id the series was materialized from"`
	MemberCount int    `json:"member_count" doc:"total anchored works in this series"`
}

type WorkPopularity struct {
	SourceID int16 `json:"source_id" doc:"catalog_source id (provenance + unit selector): counters are source-relative and never summed across sources"`
	Metric   int16 `json:"metric" doc:"popularity metric: 0=downloads 1=wishlist 2=reviews (extensible vocabulary)"`
	Value    int64 `json:"value" doc:"raw counter verbatim from the source; a published 0 is a real value, an unpublished counter has no element"`
}

type WorkPlaytime struct {
	SourceID  int16 `json:"source_id" doc:"catalog_source id (provenance): vndb = vote-backed median, erogamescape = community median"`
	Minutes   int   `json:"minutes" doc:"median playtime in minutes (unit-normalized; estimate semantics stay source-native)"`
	VoteCount int   `json:"vote_count" doc:"user reports backing the estimate; 0 = the source publishes no per-work count"`
}

type WorkTag struct {
	Name        string `json:"name" doc:"tag text, verbatim from the source (no vocabulary mapping)"`
	Count       int    `json:"count,omitempty" doc:"source vote count backing the tag; absent when the source has no votes (bridged galgame wiki tags)"`
	SourceID    int16  `json:"source_id" doc:"catalog_source id (provenance): galgame_wiki/vndb for a bridged claimed tag, the backfill source (bangumi) for a bodyless tag"`
	CanonicalID *int64 `json:"canonical_id,omitempty" doc:"catalog_tag id when this (source_id, name) is mapped into the canonical vocabulary; absent otherwise"`
	Tier        *int16 `json:"tier,omitempty" doc:"canonical display tier (0=core 1=longtail 2=hidden); absent when unmapped"`
	Kind        *int16 `json:"kind,omitempty" doc:"canonical kind (0=content 1=meta/platform-attribute); absent when unmapped"`
}

type WorkRating struct {
	SourceID     int16          `json:"source_id" doc:"catalog_source id (provenance + scale selector): vndb = 1-10 bayesian-smoothed rating, bangumi = 0-10 mean, dlsite = 0-5 star mean, erogamescape = 0-100 median"`
	Score        float64        `json:"score" doc:"rating on the source-native scale (never normalized across sources)"`
	VoteCount    int            `json:"vote_count" doc:"number of ratings backing the score"`
	Rank         *int           `json:"rank,omitempty" doc:"source-internal rank; absent when the source has no rank or the work is unranked"`
	Distribution []RatingBucket `json:"distribution,omitempty" doc:"vote histogram on the source-native scale, ascending, sparse (an absent bucket has no votes); detail face only. All four sources publish it: bangumi 1-10, dlsite 1-5, vndb 1-10, erogamescape 0-100 in decile steps (0, 10, ... 100). The bars do not share one denominator: bangumi and dlsite publish the histogram together with the aggregate, so their bars sum to vote_count; erogamescape bars are computed from the reviews mirror, which syncs on a cursor independent of the row score and vote_count come from, so sum-of-bars is the histogram's own denominator and need not equal vote_count; vndb bars come from the public votes dump, which omits votes on private lists, so they sum to at most vote_count"`
	Stats        *RatingStats   `json:"stats,omitempty" doc:"spread of the same vote population as score; detail face only. Carried by erogamescape (average, stdev, min and max, on its 0-100 scale) and by vndb (average only — the plain mean beside the bayesian-smoothed rating that score holds). bangumi and dlsite carry no stats"`
}

type RatingBucket struct {
	Score int `json:"score" doc:"bucket value on the source-native scale"`
	Count int `json:"count" doc:"votes cast at this value"`
}

type RatingStats struct {
	Average *float64 `json:"average,omitempty" doc:"mean on the source-native scale (score itself is the median for erogamescape)"`
	Stdev   *float64 `json:"stdev,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
}

type WorkScreenshot struct {
	ImageHash string `json:"image_hash"`
	Caption   string `json:"caption"`
	SortOrder int    `json:"sort_order" doc:"gallery position"`
	Sexual    int16  `json:"sexual" doc:"content flag: 0=safe 1=suggestive 2=explicit"`
	Violence  int16  `json:"violence" doc:"content flag: 0=safe 1=suggestive 2=explicit"`
	SourceID  int16  `json:"source_id" doc:"catalog_source id (provenance): galgame_wiki/vndb for a bridged claimed screenshot, the backfill source for a bodyless screenshot"`
}

type WorkCover struct {
	ID             int64  `json:"id" doc:"catalog_work_cover row id (the vote endpoints' cover id)"`
	ImageHash      string `json:"image_hash"`
	Kind           string `json:"kind"`
	PortraitPinned bool   `json:"portrait_pinned" doc:"true = the vertical portrait pin (portrait-first UI)"`
	SortOrder      int    `json:"sort_order" doc:"gallery/pin position; 0 = pinned landscape banner"`
	Sexual         int16  `json:"sexual" doc:"content flag: 0=safe 1=suggestive 2=explicit"`
	Violence       int16  `json:"violence" doc:"content flag: 0=safe 1=suggestive 2=explicit"`
	SourceID       int16  `json:"source_id" doc:"catalog_source id (provenance): galgame_wiki/vndb/bangumi/upscale for a bridged claimed cover, the backfill source for a bodyless cover"`
	VoteCount      int    `json:"vote_count" doc:"advisory best-cover votes on this cover; never reorders anything server-side"`
}

type WorkIntro struct {
	Lang     string `json:"lang"`
	Intro    string `json:"intro"`
	SourceID int16  `json:"source_id" doc:"catalog_source id (provenance): e.g. galgame_wiki for a bridged claimed work, vndb for a bodyless backfill"`
	Machine  bool   `json:"machine,omitempty" doc:"true if this intro is an LLM machine translation (no source text for this language); omitted for source/bridged rows"`
}

type WorkCharacter struct {
	CharacterID int64             `json:"character_id"`
	DisplayName string            `json:"display_name"`
	Latin       string            `json:"latin,omitempty"`
	Gender      int16             `json:"gender,omitempty"`
	Kind        int16             `json:"kind" doc:"appearance strength: 0=unknown 1=main 2=secondary 3=appears"`
	Spoiler     int16             `json:"spoiler" doc:"appearance spoiler level: 0=none 1=minor 2=major"`
	ImageHash   string            `json:"image_hash,omitempty"`
	FigureHash  string            `json:"figure_hash,omitempty"`
	Identity    string            `json:"identity,omitempty" doc:"Opaque row identity for catalog.work.roster.suppressed; echo it back, never rebuild it. Present when the character is on the roster; ABSENT when it appears only through a voice credit, in which case kind and spoiler are 0 and the row is edited via /works/{id}/credits"`
	Va          []WorkCharacterVA `json:"va"`
}

// WorkCharacterVA deliberately carries NO `identity`, and neither do the other
// roster-shaped voice projections (VoiceName, PublicRosterVoice,
// PublicVoiceName). Their SQL is SELECT DISTINCT over catalog_credit with no
// role filter, so one rendered row stands for 1..N credit rows with 1..N
// different identities. A single value would be a lie and an array would be
// worse: one click would suppress N rows the reader never saw. Suppressing a VA
// is done on /works/{id}/credits, which is 1:1 with the table.
type WorkCharacterVA struct {
	CreditNameID int64  `json:"credit_name_id"`
	Name         string `json:"name"`
}

type WorkRef struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
	Level      string `json:"level" doc:"work | release"`
	ReleaseID  int64  `json:"release_id,omitempty" doc:"present when level=release"`
}

type WorkCore struct {
	ID            int64  `json:"id"`
	MediumID      int16  `json:"medium_id"`
	DisplayName   string `json:"display_name"`
	OLang         string `json:"olang"`
	ContentRating int16  `json:"content_rating" doc:"0=all_ages 1=sensitive 2=r18"`
	Status        int16  `json:"status" doc:"0=live 1=stub 2=merged"`
	Site          string `json:"site,omitempty"`
	ProductWorkID int64  `json:"product_work_id,omitempty"`
}

type WorkTitle struct {
	Lang    string `json:"lang"`
	Title   string `json:"title"`
	Latin   string `json:"latin,omitempty"`
	Kind    int16  `json:"kind" doc:"0=official 1=alias 2=abbreviation 3=search_hint"`
	Machine bool   `json:"machine,omitempty" doc:"true if this title is an LLM machine translation (no source published one for this language); omitted for source rows"`
}

type ReleaseBrief struct {
	ID        int64       `json:"id"`
	Kind      int16       `json:"kind" doc:"0=default 1=digital 2=physical 3=trial 4=patch"`
	ReleasedY int16       `json:"released_y,omitempty"`
	ReleasedM int16       `json:"released_m,omitempty"`
	ReleasedD int16       `json:"released_d,omitempty"`
	Platform  string      `json:"platform,omitempty"`
	Platforms []string    `json:"platforms,omitempty"`
	Hidden    bool        `json:"hidden,omitempty" doc:"true when this release is editor-hidden; omitted on live rows"`
	Anchors   []AnchorRef `json:"anchors"`
}

type AnchorRef struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
	LinkKind   int16  `json:"link_kind" doc:"0=exact 1=probable 2=related"`
	MatchedBy  string `json:"matched_by,omitempty"`
}

type WorkLabel struct {
	LabelID     int64  `json:"label_id"`
	DisplayName string `json:"display_name"`
	LabelKind   int16  `json:"label_kind" doc:"the label's own kind (0=game_brand … 4=doujin_circle …)"`
	Kind        int16  `json:"kind" doc:"attribution nature: 0=circle 1=publisher 2=developer 3=brand"`
}

type WorkCreditsResponse struct {
	WorkID int64         `json:"work_id"`
	Groups []CreditGroup `json:"groups"`
}

type CreditGroup struct {
	RoleID   int64        `json:"role_id"`
	RoleKey  string       `json:"role_key"`
	RoleName string       `json:"role_name"`
	Credits  []CreditItem `json:"credits"`
}

type CreditItem struct {
	CreditNameID int64  `json:"credit_name_id"`
	Name         string `json:"name"`
	Lang         string `json:"lang"`
	Latin        string `json:"latin,omitempty"`
	CharacterID  int64  `json:"character_id,omitempty"`
	Character    string `json:"character,omitempty"`
	Note         string `json:"note,omitempty"`
	Source       string `json:"source,omitempty"`
	Identity     string `json:"identity" doc:"Opaque row identity for catalog.work.credits.suppressed; echo it back, never rebuild it"`
}

type WorkSearchResponse struct {
	Items []WorkSearchHit `json:"items"`
}

type WorkSearchHit struct {
	WorkID        int64  `json:"work_id"`
	DisplayName   string `json:"display_name"`
	MediumID      int16  `json:"medium_id"`
	ContentRating int16  `json:"content_rating" doc:"0=all_ages 1=sensitive 2=r18"`
	Status        int16  `json:"status" doc:"0=live 1=stub 2=merged"`
	Site          string `json:"site,omitempty"`
	DlsiteID      string `json:"dlsite_id,omitempty"`
}

type EntitySearchResponse struct {
	Items []EntitySearchHit `json:"items"`
	Total int64             `json:"total"`
}

type EntitySearchHit struct {
	ID            string   `json:"id" doc:"prefixed: n{id} / c{id} / b{id}"`
	EntityType    string   `json:"entity_type"`
	Name          string   `json:"name"`
	Latin         string   `json:"latin,omitempty"`
	Sources       []string `json:"sources"`
	Popularity    float64  `json:"popularity"`
	Kind          *int16   `json:"kind,omitempty" doc:"label kind (labels only)"`
	PersonID      *int64   `json:"person_id,omitempty" doc:"credit-name → person link (names only; absent = orphan)"`
	ContentRating *int16   `json:"content_rating,omitempty" doc:"works only (wave 105): 0=all_ages 1=sensitive 2=r18"`
	LogoHash      string   `json:"logo_hash,omitempty" doc:"labels only: brand logo content hash in the image service; absent = none"`
}

type WorkBrief struct {
	WorkID        int64  `json:"work_id"`
	DisplayName   string `json:"display_name"`
	MediumID      int16  `json:"medium_id"`
	ContentRating int16  `json:"content_rating" doc:"0=all_ages 1=sensitive 2=r18"`
	Status        int16  `json:"status" doc:"0=live 1=stub 2=merged"`
	Site          string `json:"site,omitempty"`
}

type NameWorksResponse struct {
	Name  NameHead      `json:"name"`
	Items []NameWorkRow `json:"items"`
	Total int64         `json:"total"`
}

type NameHead struct {
	ID          int64         `json:"id"`
	DisplayName string        `json:"display_name"`
	Lang        string        `json:"lang,omitempty" doc:"BCP-47 language of display_name; empty when unrecorded"`
	Latin       string        `json:"latin,omitempty"`
	PersonID    int64         `json:"person_id,omitempty" doc:"same-person group id (public links only; absent = orphan or hidden)"`
	Siblings    []SiblingName `json:"siblings" doc:"the same person's other public-linked names"`
	PhotoHash   string        `json:"photo_hash" doc:"person photo content hash in the image service; \"\" = no photo (or the person link is hidden)"`
	Gender      *int16        `json:"gender,omitempty" doc:"person gender code; absent = unknown, orphan or hidden link"`
	BirthY      *int16        `json:"birth_y,omitempty" doc:"fuzzy birth date, year; absent = not recorded at this precision"`
	BirthM      *int16        `json:"birth_m,omitempty" doc:"fuzzy birth date, month"`
	BirthD      *int16        `json:"birth_d,omitempty" doc:"fuzzy birth date, day"`
}

type SiblingName struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Lang        string `json:"lang,omitempty" doc:"BCP-47 language of display_name; empty when unrecorded"`
	Latin       string `json:"latin,omitempty"`
}

type NameWorkRow struct {
	Work  WorkBrief      `json:"work"`
	Roles []NameWorkRole `json:"roles"`
}

type NameWorkRole struct {
	RoleID      int64  `json:"role_id"`
	RoleKey     string `json:"role_key"`
	RoleName    string `json:"role_name"`
	CharacterID int64  `json:"character_id,omitempty"`
	Character   string `json:"character,omitempty"`
	Identity    string `json:"identity" doc:"Opaque row identity for catalog.work.credits.suppressed; echo it back, never rebuild it"`
}

type CharacterWorksResponse struct {
	Character CharacterHead      `json:"character"`
	Items     []CharacterWorkRow `json:"items"`
	Total     int64              `json:"total"`
}

type CharacterHead struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Lang        string `json:"lang,omitempty" doc:"BCP-47 language of display_name; empty when unrecorded"`
	Latin       string `json:"latin,omitempty"`
}

type CharacterWorkRow struct {
	Work     WorkBrief   `json:"work"`
	Kind     int16       `json:"kind" doc:"roster appearance strength: 0=unknown 1=main 2=secondary 3=appears (0 also when reached only via a voice credit)"`
	Spoiler  int16       `json:"spoiler" doc:"roster appearance spoiler level: 0=none 1=minor 2=major (0 also when reached only via a voice credit)"`
	Identity string      `json:"identity,omitempty" doc:"Opaque row identity for catalog.work.roster.suppressed on THIS work; echo it back, never rebuild it. Absent when the work is reached only through a voice credit"`
	Voiced   bool        `json:"voiced" doc:"true when a voice credit names this character on this work"`
	Voices   []VoiceName `json:"voices"`
}

type CharacterDetailResponse struct {
	ID            int64             `json:"id"`
	DisplayName   string            `json:"display_name"`
	Latin         string            `json:"latin,omitempty"`
	Lang          string            `json:"lang"`
	Gender        int16             `json:"gender,omitempty"`
	Description   string            `json:"description,omitempty"`
	InstanceOf    int64             `json:"instance_of,omitempty"`
	ImageHash     string            `json:"image_hash,omitempty"`
	FigureHash    string            `json:"figure_hash,omitempty"`
	BirthdayMonth int16             `json:"birthday_month,omitempty"`
	BirthdayDay   int16             `json:"birthday_day,omitempty"`
	BloodType     int16             `json:"blood_type,omitempty" doc:"1=A 2=B 3=AB 4=O; absent = unknown"`
	HeightCm      int16             `json:"height_cm,omitempty"`
	WeightKg      int16             `json:"weight_kg,omitempty"`
	BustCm        int16             `json:"bust_cm,omitempty"`
	WaistCm       int16             `json:"waist_cm,omitempty"`
	HipCm         int16             `json:"hip_cm,omitempty"`
	Cup           string            `json:"cup,omitempty"`
	Extra         map[string]any    `json:"extra,omitempty"`
	AttrSources   map[string]string `json:"attr_sources,omitempty"`
	Aliases       []CharacterAlias  `json:"aliases"`
	Intros        []WorkIntro       `json:"intros"`
	Traits        []CharacterTrait  `json:"traits"`
}

type CharacterTrait struct {
	ID           int64  `json:"id"`
	Name         string `json:"name" doc:"verbatim VNDB English trait name"`
	NameZh       string `json:"name_zh,omitempty" doc:"Simplified-Chinese trait name; absent when the vocabulary row has none yet"`
	GroupTID     string `json:"group_tid" doc:"VNDB root-group tid (i1=Hair, i35=Eyes, …); empty when the trait is itself a root"`
	GroupName    string `json:"group_name,omitempty" doc:"root-group display name; absent for a root trait"`
	GroupNameZh  string `json:"group_name_zh,omitempty" doc:"Simplified-Chinese root-group name; absent for a root trait or an unrendered group"`
	Sexual       bool   `json:"sexual,omitempty" doc:"the trait belongs to a sexual trait family — consumers gate rendering themselves"`
	SpoilerLevel int16  `json:"spoiler_level" doc:"per-character spoiler grade: 0 none / 1 minor / 2 major"`
	Lie          bool   `json:"lie,omitempty" doc:"presented as true in-story but actually a lie (VNDB flag)"`
}

type CharacterAlias struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Latin              string `json:"latin,omitempty"`
	Lang               string `json:"lang"`
	Kind               int16  `json:"kind" doc:"0=translation 1=spelling_variant 2=search_hint"`
	IsPrimaryForLocale bool   `json:"is_primary_for_locale"`
}

// No `identity` here either — see WorkCharacterVA.
type VoiceName struct {
	CreditNameID int64  `json:"credit_name_id"`
	Name         string `json:"name"`
	Lang         string `json:"lang"`
	Latin        string `json:"latin,omitempty"`
}

type WorkRelation struct {
	RelationType  string `json:"relation_type" doc:"vocabulary key, e.g. sequel_of / fandisc_of / remake_of / collects / shares_character"`
	Phrase        string `json:"phrase" doc:"direction-resolved display phrase from this work's perspective"`
	WorkID        int64  `json:"work_id" doc:"the other end's catalog work id"`
	DisplayName   string `json:"display_name"`
	MediumID      int16  `json:"medium_id"`
	ContentRating int16  `json:"content_rating"`
	Status        int16  `json:"status"`
	Site          string `json:"site,omitempty" doc:"claim site when the other end is claimed"`
	ProductWorkID int64  `json:"product_work_id,omitempty"`
}

type WorkSeriesSibling struct {
	WorkID        int64  `json:"work_id"`
	DisplayName   string `json:"display_name"`
	MediumID      int16  `json:"medium_id"`
	ContentRating int16  `json:"content_rating"`
	Status        int16  `json:"status"`
	Site          string `json:"site,omitempty"`
	ProductWorkID int64  `json:"product_work_id,omitempty"`
}
