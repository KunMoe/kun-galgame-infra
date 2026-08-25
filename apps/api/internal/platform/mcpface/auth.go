package mcpface

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const keyPrefix = "nmk_"

const devPortalURL = "https://developer.nextmoe.dev"

func bearerToken(header http.Header) (token string, ok bool) {
	if header == nil {
		return "", false
	}
	v, found := strings.CutPrefix(header.Get("Authorization"), "Bearer ")
	if !found {
		return "", false
	}
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, keyPrefix) {
		return "", false
	}
	if !validV2KeyForm(v) {
		return "", false
	}
	return v, true
}

func validV2KeyForm(raw string) bool {
	if len(raw) != 37 {
		return false
	}
	return strings.HasPrefix(raw, "nmk_live_") || strings.HasPrefix(raw, "nmk_test_")
}

func keyFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:8]
}
