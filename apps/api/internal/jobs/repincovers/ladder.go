package repincovers

import (
	"sort"

	"api/internal/platform/catalog/model"
)

const upscaleTarget = 1080

const nsfwSexualFloor = model.SexualExplicit

const tierIneligible = 99

func tier(kind string) int {
	switch kind {
	case "dig", "main":
		return 0
	case "pkgfront":
		return 1
	case "":
		return 2
	default:
		return tierIneligible
	}
}

func isPortrait(w, h int) bool { return w > 0 && h > 0 && int64(h)*20 > int64(w)*21 }

type Cover struct {
	ID        int64
	WorkID    int64
	Hash      string
	Kind      string
	SourceKey string
	Sexual    int16
	SortOrder int
	Pinned    bool
	Width     int
	Height    int
	DimsKnown bool
}

func (c Cover) LongEdge() int {
	if c.Width > c.Height {
		return c.Width
	}
	return c.Height
}

type Action int

const (
	ActionNone Action = iota
	ActionDirectPin
	ActionUpscale
	ActionDeferredNSFW
)

func (a Action) String() string {
	switch a {
	case ActionDirectPin:
		return "direct_pin"
	case ActionUpscale:
		return "upscale"
	case ActionDeferredNSFW:
		return "nsfw_deferred"
	default:
		return "none"
	}
}

type Plan struct {
	WorkID int64
	Old    *Cover
	New    *Cover
	Action Action
}

func selectWinner(covers []Cover) *Cover {
	eligible := make([]Cover, 0, len(covers))
	for _, c := range covers {
		if c.DimsKnown && isPortrait(c.Width, c.Height) && tier(c.Kind) < tierIneligible {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if ta, tb := tier(a.Kind), tier(b.Kind); ta != tb {
			return ta < tb
		}
		if a.LongEdge() != b.LongEdge() {
			return a.LongEdge() > b.LongEdge()
		}
		return a.Hash < b.Hash
	})
	best := eligible[0]
	return &best
}

func planWork(workID int64, covers []Cover) Plan {
	p := Plan{WorkID: workID}
	for i := range covers {
		if covers[i].Pinned && p.Old == nil {
			p.Old = &covers[i]
		}
	}
	p.New = selectWinner(covers)
	switch {
	case p.New == nil:
		return p
	case p.Old != nil && p.Old.Hash == p.New.Hash:
		return p
	case p.New.Sexual >= nsfwSexualFloor:
		p.Action = ActionDeferredNSFW
	case p.New.LongEdge() >= upscaleTarget:
		p.Action = ActionDirectPin
	default:
		p.Action = ActionUpscale
	}
	return p
}
