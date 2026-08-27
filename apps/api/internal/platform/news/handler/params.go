package handler

import (
	"strings"

	"api/internal/platform/news/model"
)

const (
	msgBadLimit = "limit must be an integer in [1,50]"
	msgBadLane  = "lane must be a comma-separated subset of: news, column"
	msgOffline  = "news feed is unavailable"
)

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// lanesKnown rejects an unrecognised lane instead of filtering on it. A typo
// would otherwise match no rows and return a perfectly well-formed empty feed,
// which a consumer reads as "there is no news" rather than "you asked wrong".
func lanesKnown(lanes []string) bool {
	for _, l := range lanes {
		if !model.IsKnownLane(l) {
			return false
		}
	}
	return true
}
