package settings_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"api/internal/platform/settings"
)

func wantPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestConstructorPanicsBadName(t *testing.T) {
	wantPanic(t, func() {
		settings.Bool(settings.Meta{Name: "NoDot", DescEN: "e", DescZH: "z"}, false)
	})
	wantPanic(t, func() {
		settings.Bool(settings.Meta{Name: "nodot", DescEN: "e", DescZH: "z"}, false)
	})
	wantPanic(t, func() {
		settings.Bool(settings.Meta{Name: "", DescEN: "e", DescZH: "z"}, false)
	})
}

func TestConstructorPanicsEnumDefaultOutsideEnum(t *testing.T) {
	wantPanic(t, func() {
		settings.Enum(settings.Meta{
			Name: "t.mode", DescEN: "e", DescZH: "z",
			Enum: []string{"shadow", "live"},
		}, "other")
	})
}

func TestConstructorPanicsDefaultOutsideBounds(t *testing.T) {
	wantPanic(t, func() {
		settings.Int(settings.Meta{
			Name: "t.n", DescEN: "e", DescZH: "z",
			Min: settings.F(1), Max: settings.F(10),
		}, 0)
	})
}

func sample(kind settings.Kind) settings.Entry {
	m := settings.Meta{Name: "t.k", DescEN: "e", DescZH: "z"}
	switch kind {
	case settings.KindBool:
		return settings.Bool(m, false)
	case settings.KindInt:
		return settings.Int(m, 0)
	case settings.KindFloat:
		return settings.Float(m, 0)
	case settings.KindString:
		return settings.String(m, "")
	case settings.KindEnum:
		m.Enum = []string{"shadow", "live"}
		return settings.Enum(m, "shadow")
	case settings.KindStringList:
		return settings.StringList(m, []string{})
	default:
		panic(kind)
	}
}

func TestDecodeStrictness(t *testing.T) {
	cases := []struct {
		kind settings.Kind
		in   string
		ok   bool
		want any
	}{
		{settings.KindBool, `true`, true, true},
		{settings.KindBool, `false`, true, false},
		{settings.KindBool, `1`, false, nil},
		{settings.KindBool, `"true"`, false, nil},
		{settings.KindBool, `null`, false, nil},
		{settings.KindInt, `42`, true, int64(42)},
		{settings.KindInt, `-3`, true, int64(-3)},
		{settings.KindInt, `1.5`, false, nil},
		{settings.KindInt, `1.0`, false, nil},
		{settings.KindInt, `"42"`, false, nil},
		{settings.KindInt, `true`, false, nil},
		{settings.KindFloat, `0.5`, true, 0.5},
		{settings.KindFloat, `1`, true, 1.0},
		{settings.KindFloat, `"0.5"`, false, nil},
		{settings.KindFloat, `true`, false, nil},
		{settings.KindString, `"hello"`, true, "hello"},
		{settings.KindString, `1`, false, nil},
		{settings.KindString, `null`, false, nil},
		{settings.KindEnum, `"shadow"`, true, "shadow"},
		{settings.KindEnum, `"nope"`, true, "nope"},
		{settings.KindEnum, `1`, false, nil},
		{settings.KindStringList, `["a","b"]`, true, []string{"a", "b"}},
		{settings.KindStringList, `[]`, true, []string{}},
		{settings.KindStringList, `["a", 1]`, false, nil},
		{settings.KindStringList, `"a"`, false, nil},
		{settings.KindStringList, `null`, false, nil},
	}
	for _, c := range cases {
		e := sample(c.kind)
		got, err := e.Decode(json.RawMessage(c.in))
		if c.ok {
			if err != nil {
				t.Errorf("Decode(%s, %s) err = %v, want nil", c.kind, c.in, err)
				continue
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Decode(%s, %s) = %#v (%T), want %#v", c.kind, c.in, got, got, c.want)
			}
		} else if err == nil {
			t.Errorf("Decode(%s, %s) = %#v, want error", c.kind, c.in, got)
		} else if !strings.Contains(err.Error(), "expected a JSON") {
			t.Errorf("Decode(%s, %s) err = %q, want it to say what JSON was expected", c.kind, c.in, err)
		}
	}
}

func TestParseEnv(t *testing.T) {
	b := settings.Bool(settings.Meta{Name: "t.b", DescEN: "e", DescZH: "z"}, false)
	got, err := b.ParseEnv("true")
	if err != nil || got != true {
		t.Errorf("bool true = %v, %v", got, err)
	}
	if _, err := b.ParseEnv("nope"); err == nil {
		t.Error("bool nope: want error")
	}

	n := settings.Int(settings.Meta{Name: "t.n", DescEN: "e", DescZH: "z"}, 0)
	got, err = n.ParseEnv("42")
	if err != nil || got != int64(42) {
		t.Errorf("int 42 = %v, %v", got, err)
	}
	if _, err := n.ParseEnv("1.5"); err == nil {
		t.Error("int 1.5: want error")
	}

	f := settings.Float(settings.Meta{Name: "t.f", DescEN: "e", DescZH: "z"}, 0)
	got, err = f.ParseEnv("0.5")
	if err != nil || got != 0.5 {
		t.Errorf("float 0.5 = %v, %v", got, err)
	}

	s := settings.String(settings.Meta{Name: "t.s", DescEN: "e", DescZH: "z"}, "")
	got, err = s.ParseEnv(" verbatim ")
	if err != nil || got != " verbatim " {
		t.Errorf("string = %q, %v", got, err)
	}

	en := settings.Enum(settings.Meta{Name: "t.e", DescEN: "e", DescZH: "z", Enum: []string{"shadow", "live"}}, "shadow")
	got, err = en.ParseEnv("live")
	if err != nil || got != "live" {
		t.Errorf("enum = %v, %v", got, err)
	}

	list := settings.StringList(settings.Meta{Name: "t.l", DescEN: "e", DescZH: "z"}, []string{})
	got, err = list.ParseEnv("a, b, , c")
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("list trim/drop = %#v, %v", got, err)
	}
	got, err = list.ParseEnv("  ")
	if err != nil || !reflect.DeepEqual(got, []string{}) {
		t.Errorf("list whitespace = %#v, %v; want empty list", got, err)
	}
}

func TestValidate(t *testing.T) {
	n := settings.Int(settings.Meta{
		Name: "t.n", DescEN: "e", DescZH: "z",
		Min: settings.F(1), Max: settings.F(10),
	}, 5)
	if err := n.Validate(int64(0)); err == nil || !strings.Contains(err.Error(), "below the minimum") {
		t.Errorf("below min: %v", err)
	}
	if err := n.Validate(int64(11)); err == nil || !strings.Contains(err.Error(), "above the maximum") {
		t.Errorf("above max: %v", err)
	}
	if err := n.Validate(int64(1)); err != nil {
		t.Errorf("min bound: %v", err)
	}
	if err := n.Validate(true); err == nil {
		t.Error("type mismatch: want error")
	}

	rate := settings.Float(settings.Meta{
		Name: "trust.scan_sample_rate", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(0.05),
	}, 0)
	err := rate.Validate(0.9)
	if err == nil || err.Error() != "trust.scan_sample_rate: 0.9 is above the maximum 0.05" {
		t.Errorf("float max: %v", err)
	}

	en := settings.Enum(settings.Meta{
		Name: "t.mode", DescEN: "e", DescZH: "z",
		Enum: []string{"shadow", "live"},
	}, "shadow")
	if err := en.Validate("other"); err == nil {
		t.Error("enum: want error")
	}

	list := settings.StringList(settings.Meta{
		Name: "t.list", DescEN: "e", DescZH: "z",
		Pattern: `^[a-z0-9_-]+:[a-z0-9_-]+$`,
	}, []string{})
	if err := list.Validate([]string{"kungal:forum_topic"}); err != nil {
		t.Errorf("list ok: %v", err)
	}
	if err := list.Validate([]string{"not-a-pair"}); err == nil {
		t.Error("list pattern: want error")
	}
	if err := list.Validate([]string{"a:b", " "}); err == nil {
		t.Error("empty list item: want error")
	}
}

func TestGetReturnsDefaultBeforeApply(t *testing.T) {
	k := settings.Int(settings.Meta{Name: "t.n", DescEN: "e", DescZH: "z"}, 7)
	if k.Get() != 7 {
		t.Errorf("Get() = %d, want 7", k.Get())
	}
	if k.Source() != settings.SourceDefault {
		t.Errorf("Source() = %q, want default", k.Source())
	}
}

func TestOverrideRestoresOnCleanup(t *testing.T) {
	k := settings.Int(settings.Meta{Name: "t.n", DescEN: "e", DescZH: "z"}, 7)
	t.Run("override", func(t *testing.T) {
		settings.Override(t, k, int64(99))
		if k.Get() != 99 {
			t.Fatalf("Get() = %d, want 99", k.Get())
		}
		if k.Source() != settings.SourceDB {
			t.Fatalf("Source() = %q, want db", k.Source())
		}
	})
	if k.Get() != 7 {
		t.Errorf("Get() after cleanup = %d, want 7", k.Get())
	}
	if k.Source() != settings.SourceDefault {
		t.Errorf("Source() after cleanup = %q, want default", k.Source())
	}
}
