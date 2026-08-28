package routepath

import "strings"

// Fiber routes on its own detectionPath — c.Path() lowercased while
// CaseSensitive is off, with trailing slashes trimmed while StrictRouting is
// off — but c.Path() returns the raw path the client typed. Every gate,
// limiter bucket and cache-intent rule that read c.Path() directly was keying
// on something the router had already stopped using: /v2/Catalog/works matched
// the route, missed every prefix compare below it and answered 200 with no
// credential, live in production until 2026-08-28. StrictRouting is still off
// (cmd/oauth registers `sites.Get("/")`), so the trailing-slash half of the
// same trap is reachable today.
func Normalize(raw string) string {
	p := strings.ToLower(raw)
	for len(p) > 1 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	return p
}
