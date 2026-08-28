package repr

type Change struct {
	_            struct{} `json:"-" additionalProperties:"true"`
	Object       string   `json:"object" enum:"change" doc:"Type discriminant. Always change."`
	TargetObject string   `json:"target_object" enum:"work,release,character,credit_name,person,company,tag,engine" doc:"Family of the row that changed."`
	ID           string   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog id of the changed row."`
	UpdatedAt    string   `json:"updated_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC. The moment the row last changed, and the value the cursor resumes from."`
	Gone         *bool    `json:"gone,omitempty" doc:"Present and true when the id has left the public population: merged away, or otherwise no longer served. Absent means the id is still live — never sent as false. Drop the mirrored row; if it was merged, /v2/catalog/redirects names the id that replaced it."`
}

type Redirect struct {
	_            struct{} `json:"-" additionalProperties:"true"`
	Object       string   `json:"object" enum:"redirect" doc:"Type discriminant. Always redirect."`
	TargetObject string   `json:"target_object" enum:"work,release,character,credit_name,person,company,tag,engine" doc:"Family that was merged."`
	OldID        string   `json:"old_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Retired catalog id."`
	CurrentID    string   `json:"current_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Canonical catalog id."`
	MergedAt     string   `json:"merged_at" format:"date-time" maxLength:"32" doc:"RFC 3339 UTC."`
}
