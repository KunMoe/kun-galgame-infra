package search

import (
	"fmt"
	"time"

	"api/internal/infrastructure/search"

	"github.com/meilisearch/meilisearch-go"
)

const (
	IndexCreditNames = "catalog_credit_names"
	IndexCharacters  = "catalog_characters"
	IndexLabels      = "catalog_labels"
	IndexWorks       = "catalog_works"
	IndexTags        = "catalog_tags"
)

// localizedAttributes pins the CJK language per field pattern (doc 13 invariant
// 1): *_ja → Japanese, *_zh → Chinese, bypassing whatlang autodetection. A
// single field NEVER mixes zh/ja — the projection buckets by the row's lang.
// Latin/other fields are not declared (Meili's default pipeline suffices).
//
// Query-time `locales` discipline (invariant 2): a consumer's search endpoint
// must set locales SERVER-SIDE from the site/input, NEVER pass a client-
// supplied value through — that would silently override these index settings.
// (This step ships no query surface; the rule lives here for the consumers.)
func localizedAttributes() []*meilisearch.LocalizedAttributes {
	return []*meilisearch.LocalizedAttributes{
		{AttributePatterns: []string{"*_ja"}, Locales: []string{"jpn"}},
		{AttributePatterns: []string{"*_zh"}, Locales: []string{"cmn"}},
	}
}

func cjkTypoDisabled(extra ...string) []string {
	return append([]string{"name_ja", "name_zh", "name_other", "aliases_ja", "aliases_zh", "aliases_other"}, extra...)
}

var entityRankingRules = []string{
	"words", "typo", "proximity", "attribute", "sort", "exactness", "popularity:desc",
}

// equalsSeparators pins '=' (U+003D) and '＝' (U+FF1D) as word separators.
// charabia's default separator list covers the whole Sm math-symbol category but
// skips these two: they live in Basic Latin and Halfwidth/Fullwidth Forms, not
// the U+2200–U+22FF operators block. Galgame titles use '＝' as the
// main-title/subtitle delimiter, so without this the split is left to the
// Japanese segmenter's dictionary version — and on the version that attaches
// '＝' to the left token, a bare "ココロネ" query missed "ココロネ＝ペンデュラム！"
// while "ココロネ＝" still matched. Declaring them here makes the boundary
// deterministic regardless of segmenter version.
var equalsSeparators = []string{"=", "＝"}

func creditNamesSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		SearchableAttributes: entityNameSearchable(),
		FilterableAttributes: []string{"entity_type", "source_keys", "person_id"},
		SortableAttributes:   []string{"popularity"},
		RankingRules:         entityRankingRules,
		LocalizedAttributes:  localizedAttributes(),
		SeparatorTokens:      equalsSeparators,
		TypoTolerance: &meilisearch.TypoTolerance{
			Enabled:             true,
			DisableOnAttributes: cjkTypoDisabled(),
		},
	}
}

func entityNameSearchable() []string {
	return []string{
		"name_zh", "name_ja", "name_other",
		"aliases_zh", "aliases_ja", "aliases_other", "latin",
	}
}

func charactersSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		SearchableAttributes: entityNameSearchable(),
		FilterableAttributes: []string{"entity_type", "source_keys"},
		SortableAttributes:   []string{"popularity"},
		RankingRules:         entityRankingRules,
		LocalizedAttributes:  localizedAttributes(),
		SeparatorTokens:      equalsSeparators,
		TypoTolerance:        &meilisearch.TypoTolerance{Enabled: true, DisableOnAttributes: cjkTypoDisabled()},
	}
}

func labelsSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		SearchableAttributes: entityNameSearchable(),
		FilterableAttributes: []string{"entity_type", "source_keys", "kind"},
		SortableAttributes:   []string{"popularity"},
		RankingRules:         entityRankingRules,
		LocalizedAttributes:  localizedAttributes(),
		SeparatorTokens:      equalsSeparators,
		TypoTolerance:        &meilisearch.TypoTolerance{Enabled: true, DisableOnAttributes: cjkTypoDisabled()},
	}
}

var WorksFilterableAttributes = []string{
	"id", "entity_type", "source_keys", "content_rating", "content_limit",
	"claimed", "claim_state", "olang", "tag_ids", "label_ids", "engine_ids", "series_ids",
	"released_ord",
}

var WorksSortableAttributes = []string{"popularity", "released_ord", "updated_ts"}

var WorksTitleSearchable = []string{
	"name_zh", "name_ja", "name_other",
	"aliases_zh", "aliases_ja", "aliases_other", "latin",
}

var worksIntroSearchable = []string{"intro_zh", "intro_ja", "intro_other"}

func worksSearchable() []string {
	return append(append([]string{}, WorksTitleSearchable...), worksIntroSearchable...)
}

const worksMaxTotalHits int64 = 500_000

func worksSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		SearchableAttributes: worksSearchable(),
		FilterableAttributes: WorksFilterableAttributes,
		SortableAttributes:   WorksSortableAttributes,
		RankingRules:         entityRankingRules,
		SeparatorTokens:      equalsSeparators,
		TypoTolerance: &meilisearch.TypoTolerance{
			Enabled: true, DisableOnAttributes: cjkTypoDisabled(worksIntroSearchable...),
		},
		Pagination: &meilisearch.Pagination{MaxTotalHits: worksMaxTotalHits},
	}
}

func tagsSettings() *meilisearch.Settings {
	return &meilisearch.Settings{
		SearchableAttributes: []string{"name_zh", "name_ja", "name_other", "latin"},
		FilterableAttributes: []string{"entity_type", "source_keys", "kind", "tier"},
		SortableAttributes:   []string{"popularity"},
		RankingRules:         entityRankingRules,
		LocalizedAttributes:  localizedAttributes(),
		SeparatorTokens:      equalsSeparators,
		TypoTolerance:        &meilisearch.TypoTolerance{Enabled: true, DisableOnAttributes: cjkTypoDisabled()},
	}
}

func indexSpecs() []struct {
	uid      string
	settings *meilisearch.Settings
} {
	return []struct {
		uid      string
		settings *meilisearch.Settings
	}{
		{IndexCreditNames, creditNamesSettings()},
		{IndexCharacters, charactersSettings()},
		{IndexLabels, labelsSettings()},
		{IndexWorks, worksSettings()},
		{IndexTags, tagsSettings()},
	}
}

func EnsureIndexes(client *search.Client) error {
	for _, spec := range indexSpecs() {
		fullUID := client.IndexUID(spec.uid)
		if task, err := client.Svc().CreateIndex(&meilisearch.IndexConfig{Uid: fullUID, PrimaryKey: "id"}); err != nil {
			if !isAlreadyExists(err) {
				return fmt.Errorf("create index %s: %w", fullUID, err)
			}
		} else if _, err := client.Svc().WaitForTask(task.TaskUID, 50*time.Millisecond); err != nil {
			return fmt.Errorf("wait create %s: %w", fullUID, err)
		}
		task, err := client.Index(spec.uid).UpdateSettings(spec.settings)
		if err != nil {
			return fmt.Errorf("update settings %s: %w", fullUID, err)
		}
		if _, err := client.Svc().WaitForTask(task.TaskUID, 50*time.Millisecond); err != nil {
			return fmt.Errorf("wait settings %s: %w", fullUID, err)
		}
		if spec.settings.LocalizedAttributes == nil {
			// meilisearch-go tags every Settings field `omitempty`, so a spec
			// that declares NO localizedAttributes PATCHes nothing and an index
			// created under an older spec would silently keep its old pins
			// forever. Reset explicitly, or this settings block stops being the
			// declared terminal state it claims to be.
			task, err := client.Index(spec.uid).ResetLocalizedAttributes()
			if err != nil {
				return fmt.Errorf("reset localized attributes %s: %w", fullUID, err)
			}
			if _, err := client.Svc().WaitForTask(task.TaskUID, 50*time.Millisecond); err != nil {
				return fmt.Errorf("wait reset localized attributes %s: %w", fullUID, err)
			}
		}
	}
	return nil
}

func isAlreadyExists(err error) bool {
	if msErr, ok := err.(*meilisearch.Error); ok {
		return msErr.StatusCode == 409 || msErr.MeilisearchApiError.Code == "index_already_exists"
	}
	return false
}
