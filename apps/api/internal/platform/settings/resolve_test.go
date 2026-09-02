package settings_test

import (
	"encoding/json"
	"testing"

	"api/internal/platform/settings"
)

func resolveFixture() (
	*settings.Registry,
	*settings.Key[bool],
	*settings.Key[int64],
	*settings.Key[string],
	*settings.Key[[]string],
) {
	flag := settings.Bool(settings.Meta{
		Name: "t.flag", EnvVar: "T_FLAG", DescEN: "e", DescZH: "z",
	}, false)
	count := settings.Int(settings.Meta{
		Name: "t.count", EnvVar: "T_COUNT", DescEN: "e", DescZH: "z",
		Min: settings.F(0), Max: settings.F(10),
	}, 1)
	mode := settings.Enum(settings.Meta{
		Name: "t.mode", EnvVar: "T_MODE", DescEN: "e", DescZH: "z",
		Enum: []string{"a", "b"},
	}, "a")
	list := settings.StringList(settings.Meta{
		Name: "t.list", EnvVar: "T_LIST", DescEN: "e", DescZH: "z",
	}, []string{})
	reg := settings.NewRegistry(settings.Domain{
		Name: "t", TitleZH: "t",
		Keys: []settings.Entry{flag, count, mode, list},
	})
	return reg, flag, count, mode, list
}

func TestResolvePrecedenceDefaultEnvDB(t *testing.T) {
	reg, flag, count, mode, list := resolveFixture()
	env := settings.LoadEnv(reg, func(k string) string {
		switch k {
		case "T_FLAG":
			return "true"
		case "T_COUNT":
			return "3"
		case "T_MODE":
			return "b"
		case "T_LIST":
			return "x, y"
		default:
			return ""
		}
	})

	changes := settings.Resolve(reg, nil, env)
	if len(changes) != 4 {
		t.Fatalf("env apply changes = %d, want 4: %+v", len(changes), changes)
	}
	if flag.Get() != true || flag.Source() != settings.SourceEnv {
		t.Errorf("flag after env = %v %q", flag.Get(), flag.Source())
	}
	if count.Get() != 3 || count.Source() != settings.SourceEnv {
		t.Errorf("count after env = %v %q", count.Get(), count.Source())
	}
	if mode.Get() != "b" || mode.Source() != settings.SourceEnv {
		t.Errorf("mode after env = %v %q", mode.Get(), mode.Source())
	}
	if got := list.Get(); len(got) != 2 || got[0] != "x" || got[1] != "y" || list.Source() != settings.SourceEnv {
		t.Errorf("list after env = %#v %q", got, list.Source())
	}

	rows := map[string]json.RawMessage{
		"t.flag":  json.RawMessage(`false`),
		"t.count": json.RawMessage(`8`),
	}
	changes = settings.Resolve(reg, rows, env)
	if flag.Get() != false || flag.Source() != settings.SourceDB {
		t.Errorf("flag after db = %v %q", flag.Get(), flag.Source())
	}
	if count.Get() != 8 || count.Source() != settings.SourceDB {
		t.Errorf("count after db = %v %q", count.Get(), count.Source())
	}
	if mode.Get() != "b" || mode.Source() != settings.SourceEnv {
		t.Errorf("mode without db row must stay env, got %v %q", mode.Get(), mode.Source())
	}
	if len(changes) != 2 {
		t.Errorf("db apply changes = %d, want 2: %+v", len(changes), changes)
	}

	changes = settings.Resolve(reg, rows, env)
	if len(changes) != 0 {
		t.Errorf("identical Resolve changes = %+v, want none", changes)
	}
}

func TestResolveInvalidDBFallsToEnv(t *testing.T) {
	reg, _, count, _, _ := resolveFixture()
	env := settings.LoadEnv(reg, func(k string) string {
		if k == "T_COUNT" {
			return "4"
		}
		return ""
	})
	_ = settings.Resolve(reg, nil, env)
	if count.Get() != 4 {
		t.Fatalf("precondition: env count = %d", count.Get())
	}

	rows := map[string]json.RawMessage{"t.count": json.RawMessage(`true`)}
	_ = settings.Resolve(reg, rows, env)
	if count.Get() != 4 || count.Source() != settings.SourceEnv {
		t.Errorf("invalid db must fall to env, got %v %q", count.Get(), count.Source())
	}

	rows["t.count"] = json.RawMessage(`99`)
	_ = settings.Resolve(reg, rows, env)
	if count.Get() != 4 || count.Source() != settings.SourceEnv {
		t.Errorf("out-of-bounds db must fall to env, got %v %q", count.Get(), count.Source())
	}
}

func TestLoadEnvDropsInvalid(t *testing.T) {
	reg, flag, count, _, _ := resolveFixture()
	env := settings.LoadEnv(reg, func(k string) string {
		switch k {
		case "T_FLAG":
			return "nope"
		case "T_COUNT":
			return "4"
		default:
			return ""
		}
	})
	if _, ok := env["t.flag"]; ok {
		t.Errorf("invalid env bool must be omitted, env=%v", env)
	}
	if env["t.count"] != int64(4) {
		t.Errorf("valid env int = %v", env["t.count"])
	}
	_ = settings.Resolve(reg, nil, env)
	if flag.Get() != false || flag.Source() != settings.SourceDefault {
		t.Errorf("flag after dropped env = %v %q", flag.Get(), flag.Source())
	}
	if count.Get() != 4 || count.Source() != settings.SourceEnv {
		t.Errorf("count after env = %v %q", count.Get(), count.Source())
	}
}

func TestResolveUnknownRowIgnored(t *testing.T) {
	reg, flag, _, _, _ := resolveFixture()
	rows := map[string]json.RawMessage{
		"nope.missing": json.RawMessage(`true`),
		"t.flag":       json.RawMessage(`true`),
	}
	changes := settings.Resolve(reg, rows, nil)
	if !flag.Get() {
		t.Error("known row must still apply")
	}
	for _, c := range changes {
		if c.Key == "nope.missing" {
			t.Error("unknown row must not appear in changes")
		}
	}
}
