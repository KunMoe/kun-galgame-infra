package settings

import (
	"fmt"
	"strings"
)

type Domain struct {
	Name    string
	TitleZH string
	Keys    []Entry
}

type Registry struct {
	domains []Domain
	byName  map[string]Entry
}

func NewRegistry(domains ...Domain) *Registry {
	r := &Registry{domains: domains, byName: make(map[string]Entry)}
	for _, d := range domains {
		for _, e := range d.Keys {
			name := e.Meta().Name
			seg, rest, ok := strings.Cut(name, ".")
			if !ok || rest == "" || seg != d.Name {
				panic(fmt.Sprintf("settings: key %q does not start with %q", name, d.Name+"."))
			}
			if _, exists := r.byName[name]; exists {
				panic("settings: duplicate key " + name)
			}
			r.byName[name] = e
		}
	}
	return r
}

func (r *Registry) Domains() []Domain { return r.domains }

func (r *Registry) Entries() []Entry {
	var out []Entry
	for _, d := range r.domains {
		out = append(out, d.Keys...)
	}
	return out
}

func (r *Registry) Lookup(name string) (Entry, bool) {
	e, ok := r.byName[name]
	return e, ok
}
