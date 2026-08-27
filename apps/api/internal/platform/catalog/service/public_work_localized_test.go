package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

func wtRow(lang, title, latin string, kind, provenance int16) WorkTitleRow {
	return WorkTitleRow{Lang: lang, Title: title, Latin: latin, Kind: kind, Provenance: provenance}
}

// The rows below arrive in the order nativeWorkTitles produces them
// (provenance, kind, id): workLocalized is a plain first-row-wins scan and does
// no ordering of its own. That the SQL order really makes a source title beat a
// machine one is pinned against the database in
// TestPublicWorkLocalizedSourceBeatsMachine.
func TestWorkLocalizedElection(t *testing.T) {
	tests := []struct {
		name string
		rows []WorkTitleRow
		want map[string]string
	}{{
		name: "first row wins inside a locale",
		rows: []WorkTitleRow{
			wtRow("ja", "こころ", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
			wtRow("ja", "ココロ", "", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"ja": "こころ"},
	}, {
		name: "locales are independent",
		rows: []WorkTitleRow{
			wtRow("ja", "こころ", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
			wtRow("zh-Hans", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"ja": "こころ", "zh-Hans": "心"},
	}, {
		name: "a row with no declared language answers no locale",
		rows: []WorkTitleRow{
			wtRow("", "Kokoro", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{},
	}, {
		name: "a lang that is not a tag answers no locale",
		rows: []WorkTitleRow{
			wtRow("日本語", "こころ", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{},
	}, {
		name: "a miscased tag is keyed canonically",
		rows: []WorkTitleRow{
			wtRow("zh-hant", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"zh-Hant": "心"},
	}, {
		name: "a bare zh stays bare — it is not guessed into a script",
		rows: []WorkTitleRow{
			wtRow("zh", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"zh": "心"},
	}, {
		name: "casing variants of one tag collapse to one locale",
		rows: []WorkTitleRow{
			wtRow("ZH-hans", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
			wtRow("zh-Hans", "心臓", "", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"zh-Hans": "心"},
	}, {
		name: "locales the four d7 slots cannot express are keyed anyway",
		rows: []WorkTitleRow{
			wtRow("ko", "코코로", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
			wtRow("ru", "Кокоро", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
			wtRow("vi", "Kokoro", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		},
		want: map[string]string{"ko": "코코로", "ru": "Кокоро", "vi": "Kokoro"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := workLocalized(tc.rows)
			if len(got) != len(tc.want) {
				t.Fatalf("localized = %+v, want %+v", got, tc.want)
			}
			for locale, want := range tc.want {
				if got[locale].Value != want {
					t.Fatalf("localized[%q] = %q, want %q", locale, got[locale].Value, want)
				}
			}
		})
	}
}

// The work-detail face loads titles with hints included, so a search_hint row
// reaches the election. Skipping it must happen BEFORE the locale slot is
// claimed — claiming first and skipping after would drop the real title behind
// it and leave the locale unanswered.
func TestWorkLocalizedSearchHintNeverClaimsALocale(t *testing.T) {
	rows := []WorkTitleRow{
		wtRow("zh-Hans", "kokoro 心 检索用", "", model.WorkTitleKindSearchHint, model.WorkTitleProvenanceSource),
		wtRow("zh-Hans", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine),
	}
	got := workLocalized(rows)
	zh, ok := got["zh-Hans"]
	if !ok || zh.Value != "心" {
		t.Fatalf("localized[zh-Hans] = %+v, want the official row behind the hint", zh)
	}
	if !zh.Machine {
		t.Fatalf("localized[zh-Hans] = %+v, want the machine row flagged", zh)
	}
}

func TestWorkLocalizedCarriesTheTitleKindVocabulary(t *testing.T) {
	got := workLocalized([]WorkTitleRow{
		wtRow("ja", "こころ", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		wtRow("en", "Kokoro", "", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource),
		wtRow("ko", "KKR", "", model.WorkTitleKindAbbreviation, model.WorkTitleProvenanceSource),
	})
	for locale, want := range map[string]string{"ja": "official", "en": "alias", "ko": "abbreviation"} {
		if got[locale].Kind != want {
			t.Fatalf("localized[%q].kind = %q, want %q — works speak the work-title vocabulary, not translation|spelling_variant",
				locale, got[locale].Kind, want)
		}
	}
}

func TestWorkLocalizedMachineFlagIsOnlyForMachineRows(t *testing.T) {
	got := workLocalized([]WorkTitleRow{
		wtRow("ja", "こころ", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		wtRow("zh-Hans", "心", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine),
	})
	if got["ja"].Machine {
		t.Fatalf("localized[ja] = %+v, want the source row unflagged", got["ja"])
	}
	if !got["zh-Hans"].Machine {
		t.Fatalf("localized[zh-Hans] = %+v, want the machine row flagged", got["zh-Hans"])
	}
}

func TestWorkLocalizedEmptyIsNeverNil(t *testing.T) {
	got := workLocalized(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("workLocalized(nil) = %#v, want an allocated empty map so the face emits {}", got)
	}
}

func TestWorkLatinComesFromTheDisplayRow(t *testing.T) {
	rows := []WorkTitleRow{
		wtRow("ja", "こころ", "Kokoro", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource),
		wtRow("en", "Heart", "Heart", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource),
	}
	if got := workLatin(rows, "こころ"); got != "Kokoro" {
		t.Fatalf("latin = %q, want the romanisation of the row display_name was taken from", got)
	}
	if got := workLatin(rows, "そら"); got != "" {
		t.Fatalf("latin = %q, want \"\" when no title row carries the display name", got)
	}
	if got := workLatin(nil, "こころ"); got != "" {
		t.Fatalf("latin = %q, want \"\" with no title rows at all", got)
	}
	noLatin := []WorkTitleRow{wtRow("ja", "そら", "", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)}
	if got := workLatin(noLatin, "そら"); got != "" {
		t.Fatalf("latin = %q, want \"\" when the display row records no romanisation", got)
	}
}
