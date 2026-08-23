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
	_          struct{} `json:"-" additionalProperties:"true"`
	Object     string   `json:"object" enum:"proposal" doc:"Type discriminant. Always proposal."`
	ID         string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Proposal id."`
	State      string   `json:"state" enum:"open,merged,declined,withdrawn" doc:"Proposal lifecycle state."`
	EntityType string   `json:"entity_type" maxLength:"64" doc:"Editing-engine type, e.g. catalog.work. Must not be used as a discriminant."`
	EntityID   string   `json:"entity_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Target catalog id."`
	Note       string   `json:"note" maxLength:"2000" doc:"Must not be used as a discriminant."`
}

type DecisionRecord struct {
	_        struct{} `json:"-" additionalProperties:"true"`
	Object   string   `json:"object" enum:"decision" doc:"Type discriminant. Always decision."`
	ID       string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Decision event id when the backend issues one, else the subject id."`
	Decision string   `json:"decision" enum:"approve,decline,merge" doc:"Claim decisions are approve or decline. Proposal decisions are merge or decline."`
	Note     string   `json:"note" maxLength:"2000" doc:"Must not be used as a discriminant."`
}

type SnapshotRecord struct {
	_           struct{}       `json:"-" additionalProperties:"true"`
	Object      string         `json:"object" enum:"snapshot" doc:"Type discriminant. Always snapshot."`
	EntityType  string         `json:"entity_type" maxLength:"64" doc:"Editing-engine type. Must not be used as a discriminant."`
	EntityID    string         `json:"entity_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Target catalog id."`
	FieldValues map[string]any `json:"field_values" doc:"Registered field keys to current values. Empty object if none."`
}

type CoverVote struct {
	_       struct{} `json:"-" additionalProperties:"true"`
	Object  string   `json:"object" enum:"cover_vote" doc:"Type discriminant. Always cover_vote."`
	CoverID string   `json:"cover_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog cover row id."`
	WorkID  string   `json:"work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Work the ballot is on."`
	Vote    string   `json:"vote" enum:"up" doc:"Only up is stored. down is not a catalog ballot."`
}
