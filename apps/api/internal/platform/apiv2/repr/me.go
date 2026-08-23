package repr

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

type CoverVote struct {
	_       struct{} `json:"-" additionalProperties:"true"`
	Object  string   `json:"object" enum:"cover_vote" doc:"Type discriminant. Always cover_vote."`
	CoverID string   `json:"cover_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog cover row id."`
	WorkID  string   `json:"work_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Work the ballot is on."`
	Vote    string   `json:"vote" enum:"up" doc:"Only up is stored. down is not a catalog ballot."`
}
