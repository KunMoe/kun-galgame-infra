package repr

type CompanyGraph struct {
	_      struct{}           `json:"-" additionalProperties:"true"`
	Object string             `json:"object" enum:"company_graph" doc:"Type discriminant. Always company_graph."`
	Nodes  []CompanyGraphNode `json:"nodes" doc:"Companies in this family. Empty array, never null."`
	Edges  []CompanyGraphEdge `json:"edges" doc:"Directed relations among nodes. Empty array, never null. Inverse relations are not emitted."`
}

type CompanyGraphNode struct {
	_           struct{}                 `json:"-" additionalProperties:"true"`
	Object      string                   `json:"object" enum:"company" doc:"Type discriminant. Always company."`
	ID          string                   `json:"id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Catalog company id."`
	DisplayName string                   `json:"display_name" maxLength:"512" doc:"Must not be used as a discriminant."`
	Localized   map[string]LocalizedText `json:"localized" doc:"BCP-47 keys. Empty object if none. Must not be used as a discriminant."`
	WorkCount   int                      `json:"work_count" minimum:"0" doc:"Works visible under the same NSFW gate."`
}

type CompanyGraphEdge struct {
	_        struct{} `json:"-" additionalProperties:"true"`
	FromID   string   `json:"from_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Source company id. to_id is the named relation of from_id."`
	ToID     string   `json:"to_id" pattern:"^[0-9]+$" minLength:"1" maxLength:"20" doc:"Target company id."`
	Relation string   `json:"relation" enum:"parent,imprint,spawned,succeeded_by" doc:"to_id is this relation of from_id. Inverse edges are not emitted."`
}
