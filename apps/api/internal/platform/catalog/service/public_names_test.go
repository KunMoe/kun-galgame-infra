package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

func alias(name, lang string, kind int16, primary, display bool) displayAlias {
	return displayAlias{Name: name, Lang: lang, Kind: kind, IsPrimary: primary, IsDisplay: display}
}

func TestRichAliasesExcludesDisplayAndDedupes(t *testing.T) {
	rows := []displayAlias{
		alias("緒方剛志", "ja", model.AliasKindTranslation, false, true),
		alias("绪方刚", "zh-Hans", model.AliasKindTranslation, false, false),
		alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, true, false),
		alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, false, false),
		alias("绪方刚志", "", model.AliasKindSpellingVariant, false, false),
	}
	got := richAliases(rows)
	if len(got) != 3 {
		t.Fatalf("richAliases = %+v, want 3 entries (display row dropped, value+lang dedup)", got)
	}
	if got[0].Value != "绪方刚" || got[0].Lang != "zh-Hans" || got[0].Kind != "translation" {
		t.Fatalf("richAliases[0] = %+v", got[0])
	}
	if got[1].Value != "绪方刚志" || got[1].Lang != "zh-Hans" {
		t.Fatalf("richAliases[1] = %+v", got[1])
	}
	if got[2].Value != "绪方刚志" || got[2].Lang != "" || got[2].Kind != "spelling_variant" {
		t.Fatalf("richAliases[2] = %+v, want the lang-less spelling kept as its own row", got[2])
	}
}

func TestRichAliasesEmptyIsNeverNil(t *testing.T) {
	got := richAliases(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("richAliases(nil) = %#v, want an allocated empty slice so the face emits []", got)
	}
}

func TestLocalizedNamesElection(t *testing.T) {
	tests := []struct {
		name string
		rows []displayAlias
		want map[string]string
	}{{
		name: "is_primary_for_locale wins even against an earlier row",
		rows: []displayAlias{
			alias("绪方刚", "zh-Hans", model.AliasKindTranslation, false, false),
			alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, true, false),
		},
		want: map[string]string{"zh-Hans": "绪方刚志"},
	}, {
		name: "with no primary, translation outranks spelling_variant",
		rows: []displayAlias{
			alias("Ogata", "en", model.AliasKindSpellingVariant, false, false),
			alias("Takeshi Ogata", "en", model.AliasKindTranslation, false, false),
		},
		want: map[string]string{"en": "Takeshi Ogata"},
	}, {
		name: "all else equal the arrival order decides, deterministically",
		rows: []displayAlias{
			alias("绪方刚", "zh-Hans", model.AliasKindTranslation, false, false),
			alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, false, false),
		},
		want: map[string]string{"zh-Hans": "绪方刚"},
	}, {
		name: "a value equal to the display name is kept",
		rows: []displayAlias{
			alias("美坂栞", "zh-Hans", model.AliasKindTranslation, false, true),
		},
		want: map[string]string{"zh-Hans": "美坂栞"},
	}, {
		name: "a row with no declared language answers no locale",
		rows: []displayAlias{
			alias("绪方刚志", "", model.AliasKindTranslation, true, false),
		},
		want: map[string]string{},
	}, {
		name: "a lang that is not a tag answers no locale",
		rows: []displayAlias{
			alias("株式会社ジャスト", "日语", model.AliasKindSpellingVariant, false, false),
		},
		want: map[string]string{},
	}, {
		name: "a miscased tag is keyed canonically",
		rows: []displayAlias{
			alias("Fulano", "pt-br", model.AliasKindSpellingVariant, false, false),
		},
		want: map[string]string{"pt-BR": "Fulano"},
	}, {
		name: "casing variants of one tag collapse to one locale",
		rows: []displayAlias{
			alias("绪方刚", "ZH-hans", model.AliasKindSpellingVariant, false, false),
			alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, false, false),
		},
		want: map[string]string{"zh-Hans": "绪方刚志"},
	}, {
		name: "locales are independent",
		rows: []displayAlias{
			alias("绪方刚志", "zh-Hans", model.AliasKindTranslation, false, false),
			alias("Ogata", "en", model.AliasKindSpellingVariant, false, false),
		},
		want: map[string]string{"zh-Hans": "绪方刚志", "en": "Ogata"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := localizedNames(tc.rows)
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

func TestLocalizedNamesMachineFillsOnlyEmptyLocales(t *testing.T) {
	machine := alias("爱丽丝", "zh-Hans", model.AliasKindTranslation, false, false)
	machine.Provenance = model.AliasProvenanceMachine
	sourced := alias("アリス", "ja", model.AliasKindTranslation, true, true)

	got := localizedNames([]displayAlias{machine, sourced})
	zh, ok := got["zh-Hans"]
	if !ok || zh.Value != "爱丽丝" || !zh.Machine {
		t.Fatalf("localized[zh-Hans] = %+v, want the machine fill-in flagged machine=true", zh)
	}
	if got["ja"].Value != "アリス" || got["ja"].Machine {
		t.Fatalf("localized[ja] = %+v, want the sourced ja name unflagged", got["ja"])
	}
	rich := richAliases([]displayAlias{machine, sourced})
	if len(rich) != 1 || rich[0].Value != "爱丽丝" || !rich[0].Machine {
		t.Fatalf("aliases[] = %+v, want the machine name listed with machine=true", rich)
	}
}

func TestLocalizedNamesSourceBeatsMachineInSameLocale(t *testing.T) {
	machine := alias("美坂香里", "zh-Hans", model.AliasKindTranslation, true, false)
	machine.Provenance = model.AliasProvenanceMachine
	sourced := alias("美坂香裡", "zh-Hans", model.AliasKindTranslation, false, false)

	for _, rows := range [][]displayAlias{{machine, sourced}, {sourced, machine}} {
		got := localizedNames(rows)
		zh := got["zh-Hans"]
		if zh.Value != "美坂香裡" || zh.Machine {
			t.Fatalf("localized[zh-Hans] = %+v, want the source-provenance name to win regardless of arrival order or is_primary", zh)
		}
	}
}

func TestLocalizedNamesEmptyIsNeverNil(t *testing.T) {
	got := localizedNames(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("localizedNames(nil) = %#v, want an allocated empty map so the face emits {}", got)
	}
}

func TestCanonicalLocale(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ja", "ja"},
		{"zh-Hans", "zh-Hans"},
		{"pt-br", "pt-BR"},
		{"ZH-HANS", "zh-Hans"},
		{"zh-hant-hk", "zh-Hant-HK"},
		{"es-419", "es-419"},
		{"", ""},
		{"日语", ""},
		{"zh_Hans", ""},
		{"zh-", ""},
		{"123", ""},
		{"toolongsubtag", ""},
	}
	for _, tc := range tests {
		got, ok := canonicalLocale(tc.in)
		if tc.want == "" {
			if ok {
				t.Errorf("canonicalLocale(%q) = %q, true; want it rejected", tc.in, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Errorf("canonicalLocale(%q) = %q, %v; want %q, true", tc.in, got, ok, tc.want)
		}
	}
}

func TestAliasKindKeyNeverInventsAKind(t *testing.T) {
	if got := aliasKindKey(model.AliasKindTranslation); got != "translation" {
		t.Fatalf("translation rendered as %q", got)
	}
	if got := aliasKindKey(model.AliasKindSpellingVariant); got != "spelling_variant" {
		t.Fatalf("spelling_variant rendered as %q", got)
	}
	if got := aliasKindKey(99); got != "unknown_99" {
		t.Fatalf("unknown kind rendered as %q, want a visibly unknown value", got)
	}
}
