package dto

type ModerateTextRequest struct {
	Text        string `json:"text" doc:"the UGC text to scan"`
	AuthorID    *int64 `json:"author_id,omitempty" doc:"optional author global id (attribution/repeat-offender signal); NOT the tenant — the tenant is derived from the client binding"`
	SubjectSite string `json:"subject_site,omitempty" doc:"optional site the content belongs to (e.g. kungal) when the caller proxies for other sites; falls back to the client-binding site"`
	SubjectKind string `json:"subject_kind,omitempty" doc:"optional product-side content kind (e.g. forum_topic); with the subject site it selects forced Tier2 escalation for configured (site,kind) pairs"`
}

type ModerateTextResponse struct {
	Route      string   `json:"route" doc:"the semantic route served (v0: moderate-text)"`
	Flagged    bool     `json:"flagged" doc:"true = upstream flagged the text; always false on a degraded (fail-open) path"`
	Categories []string `json:"categories,omitempty" doc:"upstream category labels when flagged"`
	Score      *float32 `json:"score,omitempty" doc:"upstream confidence in [0,1] when provided"`
	Channel    string   `json:"channel" doc:"the upstream channel/model that scored; '' when degraded"`
	Degraded   bool     `json:"degraded" doc:"true = upstream unavailable / over budget / env empty; result is fail-open (flagged:false)"`
}
