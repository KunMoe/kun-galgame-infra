package problem

import (
	"strings"
	"testing"
)

func TestRegistryClosedAndDisjoint(t *testing.T) {
	if len(Codes) == 0 || len(Reasons) == 0 {
		t.Fatal("registries are empty")
	}
	seen := map[string]string{}
	for _, d := range Codes {
		if !NamePattern.MatchString(d.Code) {
			t.Errorf("code %q fails the name pattern", d.Code)
		}
		if d.Title == "" || d.Description == "" {
			t.Errorf("code %s missing title/description", d.Code)
		}
		if Kebab(d.Code) == "" || CodeFromKebab(Kebab(d.Code)) != d.Code {
			t.Errorf("code %s is not reversible with its type URI suffix %q", d.Code, Kebab(d.Code))
		}
		want := TypeURIPrefix + string(d.Domain) + "/" + Kebab(d.Code)
		if d.TypeURI() != want {
			t.Errorf("type URI %s want %s", d.TypeURI(), want)
		}
		if strings.ContainsAny(d.TypeURI(), "0123456789") && strings.Contains(d.Code, "404") {
			t.Errorf("numeric code leaked into type URI: %s", d.TypeURI())
		}
		if prev, ok := seen[d.Code]; ok {
			t.Errorf("code %s duplicated in %s and %s", d.Code, prev, d.Domain)
		}
		seen[d.Code] = string(d.Domain)
		if d.Status < 400 || d.Status > 599 {
			t.Errorf("code %s has non-error status %d", d.Code, d.Status)
		}
	}
	for _, r := range Reasons {
		if !NamePattern.MatchString(r.Reason) {
			t.Errorf("reason %q fails the name pattern", r.Reason)
		}
		if _, clash := seen[r.Reason]; clash {
			t.Errorf("reason %s collides with a top-level code", r.Reason)
		}
		if r.Title == "" || r.Description == "" {
			t.Errorf("reason %s missing title/description", r.Reason)
		}
	}
}

func TestLookupUnknown(t *testing.T) {
	if _, ok := Lookup("NOT_A_CODE"); ok {
		t.Fatal("unknown code looked up")
	}
	if _, ok := LookupReason("NOT_A_REASON"); ok {
		t.Fatal("unknown reason looked up")
	}
	got, ok := Lookup(CodeRateLimited)
	if !ok || got.Status != 429 {
		t.Fatalf("RATE_LIMITED lookup = %+v ok=%v", got, ok)
	}
}

func TestULIDShape(t *testing.T) {
	id := newULID()
	if len(id) != 26 {
		t.Fatalf("ulid length %d", len(id))
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		ok := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')
		if c == 'I' || c == 'L' || c == 'O' || c == 'U' {
			ok = false
		}
		if !ok {
			t.Fatalf("ulid %q has non-crockford char %q at %d", id, c, i)
		}
	}
}
