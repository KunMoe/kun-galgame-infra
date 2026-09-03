package settings

import (
	"encoding/json"
	"log/slog"
	"reflect"
)

type Change struct {
	Key       string
	Old, New  any
	OldSource Source
	NewSource Source
}

func LoadEnv(reg *Registry, getenv func(string) string) map[string]any {
	out := make(map[string]any)
	for _, e := range reg.Entries() {
		m := e.Meta()
		if m.EnvVar == "" {
			continue
		}
		s := getenv(m.EnvVar)
		if s == "" {
			continue
		}
		v, err := e.ParseEnv(s)
		if err == nil {
			err = e.Validate(v)
		}
		if err != nil {
			slog.Warn("settings: env floor ignored", "key", m.Name, "env", m.EnvVar, "err", err)
			continue
		}
		out[m.Name] = v
	}
	return out
}

func Resolve(reg *Registry, rows map[string]json.RawMessage, env map[string]any) []Change {
	for k := range rows {
		if _, ok := reg.Lookup(k); !ok {
			slog.Warn("settings: override row names an unknown key; ignored", "key", k)
		}
	}

	var changes []Change
	for _, e := range reg.Entries() {
		name := e.Meta().Name
		old := e.Current()
		oldSrc := e.Source()

		applied := false
		if raw, ok := rows[name]; ok {
			v, err := e.Decode(raw)
			if err == nil {
				err = e.Validate(v)
			}
			if err != nil {
				slog.Warn("settings: override row ignored", "key", name, "err", err)
			} else {
				e.apply(v, SourceDB)
				applied = true
			}
		}
		if !applied {
			if v, ok := env[name]; ok {
				e.apply(v, SourceEnv)
			} else {
				e.apply(e.Default(), SourceDefault)
			}
		}

		if !reflect.DeepEqual(old, e.Current()) || oldSrc != e.Source() {
			changes = append(changes, Change{
				Key:       name,
				Old:       old,
				New:       e.Current(),
				OldSource: oldSrc,
				NewSource: e.Source(),
			})
		}
	}
	return changes
}

type SiteChange struct {
	Key      string
	SiteID   uint
	Old, New any
}

func ResolveSites(reg *Registry, rows map[string]map[uint]json.RawMessage) []SiteChange {
	var changes []SiteChange
	for _, e := range reg.Entries() {
		m := e.Meta()
		if !m.SiteScoped {
			continue
		}
		next := map[uint]any{}
		for siteID, raw := range rows[m.Name] {
			v, err := e.Decode(raw)
			if err == nil {
				err = e.Validate(v)
			}
			if err != nil {
				slog.Warn("settings: site override row ignored", "key", m.Name, "site", siteID, "err", err)
				continue
			}
			next[siteID] = v
		}
		prev := e.siteValues()
		for siteID, v := range next {
			if old, ok := prev[siteID]; !ok || !reflect.DeepEqual(old, v) {
				changes = append(changes, SiteChange{Key: m.Name, SiteID: siteID, Old: prev[siteID], New: v})
			}
		}
		for siteID, old := range prev {
			if _, ok := next[siteID]; !ok {
				changes = append(changes, SiteChange{Key: m.Name, SiteID: siteID, Old: old, New: nil})
			}
		}
		e.applySites(next)
	}
	return changes
}
