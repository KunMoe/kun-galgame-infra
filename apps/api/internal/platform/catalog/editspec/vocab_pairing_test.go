package editspec

import (
	"testing"

	"api/internal/platform/apiv2/vocab"
)

// These sets exist twice by construction: the validator's allowed set in this
// package and the published vocabulary in apiv2/vocab. Neither side can read
// the other's data at runtime, so equality is asserted instead of shared.
func TestVocabulariesMatchAllowedSets(t *testing.T) {
	releaseLang := make(map[string]struct{}, len(olangAllowed)+len(releaseLangExtra))
	for code := range olangAllowed {
		releaseLang[code] = struct{}{}
	}
	for code := range releaseLangExtra {
		releaseLang[code] = struct{}{}
	}
	cases := []struct {
		vocabulary string
		allowed    map[string]struct{}
	}{
		{"olang", olangAllowed},
		{"platform", releasePlatformAllowed},
		{"release_lang", releaseLang},
	}
	for _, c := range cases {
		tokens := vocab.Tokens(c.vocabulary)
		published := make(map[string]struct{}, len(tokens))
		for _, tok := range tokens {
			if _, ok := c.allowed[tok]; !ok {
				t.Errorf("%s: vocabulary publishes %q but the validator rejects it", c.vocabulary, tok)
			}
			published[tok] = struct{}{}
		}
		for code := range c.allowed {
			if _, ok := published[code]; !ok {
				t.Errorf("%s: validator accepts %q but the vocabulary does not publish it", c.vocabulary, code)
			}
		}
	}
}
