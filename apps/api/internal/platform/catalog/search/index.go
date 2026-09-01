package search

const (
	IndexCreditNames = "catalog_credit_names"
	IndexCharacters  = "catalog_characters"
	IndexLabels      = "catalog_labels"
	IndexWorks       = "catalog_works"
	IndexTags        = "catalog_tags"
)

var WorksFilterableAttributes = []string{
	"id", "entity_type", "source_keys", "content_rating", "content_limit",
	"claimed", "claim_state", "olang", "tag_ids", "label_ids", "engine_ids", "series_ids",
	"released_ord",
}
