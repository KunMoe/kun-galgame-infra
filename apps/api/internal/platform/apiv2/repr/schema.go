package repr

type ObjectSchema struct {
	_                struct{}      `json:"-" additionalProperties:"true"`
	Object           string        `json:"object" enum:"object_schema" doc:"Type discriminant. Always object_schema."`
	TargetObject     string        `json:"target_object" enum:"work,company,character,release,tag,engine,series" doc:"Family this schema describes."`
	EntityType       string        `json:"entity_type" enum:"catalog.work,catalog.label,catalog.character,catalog.release,catalog.tag,catalog.engine,catalog.series" doc:"Editing-engine type this family writes as."`
	Include          []string      `json:"include" doc:"include= tokens for this family. Empty array, never null. FULL_SET is a subset."`
	FullSet          []string      `json:"full_set" doc:"Tokens view=full expands. A subset of include. Empty array, never null."`
	CreationDisabled bool          `json:"creation_disabled" doc:"true when new rows of this family cannot be created. Always true on release."`
	Fields           []SchemaField `json:"fields" doc:"Editable fields. Empty array, never null. Actor capabilities are not evaluated."`
}

type SchemaField struct {
	_             struct{} `json:"-" additionalProperties:"true"`
	Key           string   `json:"key" maxLength:"128" pattern:"^[a-z0-9_]+(\.[a-z0-9_]+)+$" doc:"Editing-engine field key. Must not be used as a discriminant."`
	FieldType     string   `json:"field_type" enum:"text,i18nmap,enum,int,date,list,ref,imagehash" doc:"Control type. Not a domain vocabulary."`
	DiffHint      string   `json:"diff_hint" enum:"inline,lines,items,image" doc:"How a diff of this field should be rendered."`
	Deprecated    bool     `json:"deprecated" doc:"true when writes of this key are rejected."`
	MaxSuppressed int      `json:"max_suppressed" minimum:"0" doc:"Cap on this field's suppression set. 0 when the field has none."`
	MaxElements   int      `json:"max_elements" minimum:"0" doc:"Cap on a list field's element count. 0 for scalar fields."`
}
