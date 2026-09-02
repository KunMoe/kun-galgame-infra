package jobs

import (
	"sort"

	"api/internal/platform/settings/keys"
)

type Registry struct {
	jobs map[string]Job
}

func NewRegistry() *Registry {
	return &Registry{jobs: make(map[string]Job)}
}

func (r *Registry) Register(j Job) {
	if _, ok := keys.Job(j.Name); !ok {
		panic("jobs: " + j.Name + " has no settings keys; add it to internal/platform/settings/keys/jobs.go")
	}
	r.jobs[j.Name] = j
}

func (r *Registry) Get(name string) (Job, bool) {
	j, ok := r.jobs[name]
	return j, ok
}

func (r *Registry) List() []Job {
	out := make([]Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Name < out[k].Name })
	return out
}
