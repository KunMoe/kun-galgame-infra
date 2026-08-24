package protocol

import "strings"

func IfNoneMatch(header, etag string) bool {
	if header == "" || etag == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		if stripWeak(candidate) == stripWeak(etag) {
			return true
		}
	}
	return false
}

func IfMatch(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return etag != ""
		}
		if strings.HasPrefix(candidate, "W/") {
			continue
		}
		if candidate == etag {
			return true
		}
	}
	return false
}

func stripWeak(tag string) string {
	return strings.TrimPrefix(tag, "W/")
}
