package repr

import "api/internal/platform/apiv2/problem"

type UserPlaytime struct {
	_       struct{} `json:"-" additionalProperties:"true"`
	Object  string   `json:"object" enum:"playtime" doc:"Type discriminant. Always playtime."`
	WorkID  string   `json:"work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog work id this row is about."`
	Minutes int      `json:"minutes" minimum:"0" doc:"Absolute cumulative minutes."`
}

type ClaimRecord struct {
	_           struct{} `json:"-" additionalProperties:"true"`
	Object      string   `json:"object" enum:"claim" doc:"Type discriminant. Always claim."`
	ID          string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog work id this claim is on."`
	State       string   `json:"state" enum:"live,draft,pending,declined,hidden" doc:"Claim lifecycle state."`
	DisplayName string   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
}

type PlaytimeBatchItem struct {
	_       struct{}         `json:"-" additionalProperties:"true"`
	Status  int              `json:"status" minimum:"0" maximum:"599" doc:"HTTP status for this item."`
	Object  *string          `json:"object,omitempty" enum:"playtime" doc:"Present on 200 items. Always playtime."`
	WorkID  *string          `json:"work_id,omitempty" pattern:"^[0-9]+$" maxLength:"20" doc:"Catalog work id. Present on 200 items."`
	Minutes *int             `json:"minutes,omitempty" minimum:"0" doc:"Present on 200 items."`
	Problem *problem.Problem `json:"problem,omitempty" doc:"Present on failed items. A full problem object."`
}

type ProposalRecord struct {
	_               struct{}        `json:"-" additionalProperties:"true"`
	Object          string          `json:"object" enum:"proposal" doc:"Type discriminant. Always proposal."`
	ID              string          `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Proposal id."`
	State           string          `json:"state" enum:"open,merged,declined,withdrawn" doc:"Proposal lifecycle state."`
	TargetObject    string          `json:"target_object" enum:"work,company,character,release,tag,engine,series" doc:"Family of the entity this proposal targets."`
	EntityType      string          `json:"entity_type" maxLength:"64" doc:"Editing-engine type, e.g. catalog.work. Must not be used as a discriminant."`
	EntityID        string          `json:"entity_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Target catalog id."`
	Note            string          `json:"note" maxLength:"2000" doc:"Proposer's own summary. Must not be used as a discriminant."`
	ProposerUID     string          `json:"proposer_uid" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"The claiming site's own user id of the proposer. Not a catalog id."`
	Site            string          `json:"site" maxLength:"64" doc:"Tenant this proposal was filed under. Open vocabulary; must not be used as a discriminant."`
	BaseRevisionSeq int             `json:"base_revision_seq" minimum:"0" doc:"Revision seq this proposal was written against."`
	DecidedByUID    *string         `json:"decided_by_uid" pattern:"^[0-9]+$" maxLength:"20" doc:"The claiming site's own user id of the decider. null while open."`
	DecidedAt       *string         `json:"decided_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC. null while open."`
	CreatedAt       string          `json:"created_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
	UpdatedAt       string          `json:"updated_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
	Amendments      *[]Amendment    `json:"amendments,omitempty" doc:"Present when include=amendments. Empty array if none."`
	Patch           *map[string]any `json:"patch,omitempty" doc:"Present when include=patch, which only the me and moderation faces publish. Field key to proposed value."`
	EffectivePatch  *map[string]any `json:"effective_patch,omitempty" doc:"Present when include=patch. The patch after every amendment is folded in; this is what a merge would write."`
}

type DecisionRecord struct {
	_        struct{} `json:"-" additionalProperties:"true"`
	Object   string   `json:"object" enum:"decision" doc:"Type discriminant. Always decision."`
	ID       string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Decision event id when the backend issues one, else the subject id."`
	Decision string   `json:"decision" enum:"approve,decline,merge,ban,unban" doc:"Claim decisions are approve, decline, ban, or unban. Proposal decisions are merge or decline."`
	Note     string   `json:"note" maxLength:"2000" doc:"Must not be used as a discriminant."`
}

type SnapshotRecord struct {
	_           struct{}       `json:"-" additionalProperties:"true"`
	Object      string         `json:"object" enum:"snapshot" doc:"Type discriminant. Always snapshot."`
	EntityType  string         `json:"entity_type" maxLength:"64" doc:"Editing-engine type. Must not be used as a discriminant."`
	EntityID    string         `json:"entity_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Target catalog id."`
	FieldValues map[string]any `json:"field_values" doc:"Registered field keys to current values. Empty object if none."`
}

type NewsSubmission struct {
	_           struct{}   `json:"-" additionalProperties:"true"`
	Object      string     `json:"object" enum:"news_submission" doc:"Type discriminant. Always news_submission."`
	ID          string     `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"News item id. Same id space as /v2/news."`
	Source      NewsSource `json:"source" doc:"The source row that grants this submission."`
	Lane        string     `json:"lane" enum:"news,column" doc:"Which of the source's two sections this item belongs to."`
	Status      string     `json:"status" enum:"pending,published,rejected,withdrawn" doc:"Moderation lifecycle state. POST always lands on pending."`
	Title       string     `json:"title" maxLength:"512" doc:"Must not be used as a discriminant."`
	Summary     string     `json:"summary" maxLength:"200" doc:"Lede, at most 200 runes. Must not be used as a discriminant."`
	SourceURL   string     `json:"source_url" format:"uri" maxLength:"1024" doc:"Canonical link to the original item."`
	BannerHash  string     `json:"banner_hash" maxLength:"64" pattern:"^([0-9a-f]{64})?$" doc:"Image-service content hash of the banner. Empty string when there is none."`
	PublishedAt string     `json:"published_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
	WorkIDs     []string   `json:"work_ids" doc:"Catalog work ids linked by hand. Empty array, never null."`
}

type CoverVote struct {
	_       struct{} `json:"-" additionalProperties:"true"`
	Object  string   `json:"object" enum:"cover_vote" doc:"Type discriminant. Always cover_vote."`
	CoverID string   `json:"cover_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog cover row id."`
	WorkID  string   `json:"work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Work the ballot is on."`
	Vote    string   `json:"vote" enum:"up" doc:"Only up is stored. down is not a catalog ballot."`
}
