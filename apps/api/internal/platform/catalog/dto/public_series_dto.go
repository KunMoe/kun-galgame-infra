package dto

type PublicSeriesDetail struct {
	ID          int64                `json:"id"`
	DisplayName string               `json:"display_name"`
	WorkCount   int                  `json:"work_count"`
	Refs        []PublicCatalogRef   `json:"refs"`
	Intros      []PublicSeriesIntro  `json:"intros"`
	Works       []PublicWorkBrief    `json:"works,omitempty"`
	Members     []PublicSeriesMember `json:"members,omitempty"`
	NextOffset  *int                 `json:"next_offset,omitempty"`
	HasNSFW     bool                 `json:"has_nsfw"`
}

type PublicSeriesMember struct {
	WorkID   int64  `json:"work_id"`
	Position int16  `json:"position"`
	Kind     string `json:"kind" enum:"unknown,main,fandisc,side_story,collection"`
}

type PublicSeriesListData struct {
	Items      []PublicSeriesListItem `json:"items"`
	Total      int64                  `json:"total"`
	NextCursor *string                `json:"next_cursor,omitempty"`
}

type PublicSeriesListItem struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
	WorkCount   int    `json:"work_count"`
	HasNSFW     bool   `json:"has_nsfw"`
}

type PublicSeriesIntro struct {
	Lang   string `json:"lang"`
	Intro  string `json:"intro"`
	Source string `json:"source"`
}
