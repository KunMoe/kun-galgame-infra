package settings_test

import (
	"testing"

	"api/internal/platform/settings"
)

func TestRegistryDuplicateNamePanics(t *testing.T) {
	a := settings.Bool(settings.Meta{Name: "t.flag", DescEN: "e", DescZH: "z"}, false)
	b := settings.Bool(settings.Meta{Name: "t.flag", DescEN: "e", DescZH: "z"}, true)
	wantPanic(t, func() {
		settings.NewRegistry(settings.Domain{
			Name: "t", TitleZH: "t", Keys: []settings.Entry{a, b},
		})
	})
}

func TestRegistryMisprefixedKeyPanics(t *testing.T) {
	k := settings.Bool(settings.Meta{Name: "other.flag", DescEN: "e", DescZH: "z"}, false)
	wantPanic(t, func() {
		settings.NewRegistry(settings.Domain{
			Name: "t", TitleZH: "t", Keys: []settings.Entry{k},
		})
	})
}

func TestRegistryLookup(t *testing.T) {
	k := settings.Bool(settings.Meta{Name: "t.flag", DescEN: "e", DescZH: "z"}, false)
	reg := settings.NewRegistry(settings.Domain{
		Name: "t", TitleZH: "t", Keys: []settings.Entry{k},
	})
	got, ok := reg.Lookup("t.flag")
	if !ok || got != k {
		t.Fatalf("Lookup hit = %v, %v", got, ok)
	}
	if _, ok := reg.Lookup("t.missing"); ok {
		t.Fatal("Lookup miss must be false")
	}
}
