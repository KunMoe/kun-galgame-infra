package devapi

import (
	"strings"
	"testing"
)

func TestGenerateKeyFormat(t *testing.T) {
	for _, prefix := range []string{LivePrefix, TestPrefix} {
		k, err := GenerateKey(prefix)
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		if !strings.HasPrefix(k, prefix) {
			t.Errorf("key %q missing prefix %q", k, prefix)
		}
		body := strings.TrimPrefix(k, prefix)
		if len(body) < 20 {
			t.Errorf("key body too short: %q (%d chars)", body, len(body))
		}
		for _, r := range body {
			if !strings.ContainsRune(base62Alphabet, r) {
				t.Errorf("key body has non-base62 char %q in %q", r, body)
			}
		}
	}
}

func TestGenerateKeyUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		k, err := GenerateKey(LivePrefix)
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		if seen[k] {
			t.Fatalf("duplicate key generated: %q", k)
		}
		seen[k] = true
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	k, _ := GenerateKey(LivePrefix)
	stored := HashKey(k)
	if !strings.HasPrefix(stored, "sha256:") {
		t.Errorf("stored hash missing sha256: prefix: %q", stored)
	}
	if strings.Contains(stored, k) {
		t.Errorf("stored hash must not embed the plaintext key")
	}
	if !VerifyKeyHash(k, stored) {
		t.Errorf("VerifyKeyHash rejected the correct key")
	}
	other, _ := GenerateKey(LivePrefix)
	if VerifyKeyHash(other, stored) {
		t.Errorf("VerifyKeyHash accepted a wrong key")
	}
	if VerifyKeyHash(k, "plaintext-not-hashed") {
		t.Errorf("VerifyKeyHash accepted a non-sha256 stored value")
	}
}

func TestKeyMetadata(t *testing.T) {
	k := "nm_live_a1b2c3d4e5"
	prefix, last4 := KeyMetadata(k)
	if prefix != "nm_live_a1b2" {
		t.Errorf("prefix = %q, want nm_live_a1b2", prefix)
	}
	if last4 != "d4e5" {
		t.Errorf("last4 = %q, want d4e5", last4)
	}
	if !strings.HasPrefix(k, prefix) {
		t.Errorf("prefix %q is not a prefix of the key", prefix)
	}
}

func TestHasKeyPrefix(t *testing.T) {
	cases := map[string]bool{
		"nm_live_abc":          true,
		"nm_test_abc":          true,
		"nmk_live_abc":         true,
		"nmk_test_abc":         true,
		"nm_prod_abc":          false,
		"eyJhbGciOi.jwt.token": false,
		"":                     false,
	}
	for raw, want := range cases {
		if got := HasKeyPrefix(raw); got != want {
			t.Errorf("HasKeyPrefix(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestGenerateV2KeyFormat(t *testing.T) {
	for _, live := range []bool{true, false} {
		k, err := GenerateV2Key(live)
		if err != nil {
			t.Fatalf("GenerateV2Key: %v", err)
		}
		if len(k) != V2KeyLen {
			t.Errorf("len(%q) = %d, want %d", k, len(k), V2KeyLen)
		}
		if !ValidV2Key(k) {
			t.Errorf("ValidV2Key rejected generated key %q", k)
		}
		wantPrefix := V2LivePrefix
		if !live {
			wantPrefix = V2TestPrefix
		}
		if !strings.HasPrefix(k, wantPrefix) {
			t.Errorf("key %q missing prefix %q", k, wantPrefix)
		}
		tampered := k[:len(k)-1] + "0"
		if tampered != k && ValidV2Key(tampered) {
			t.Errorf("ValidV2Key accepted a tampered CRC: %q", tampered)
		}
	}
	if ValidV2Key("nm_live_notav2key") {
		t.Error("v1 key must not pass ValidV2Key")
	}
	if ValidV2Key("nmk_live_short") {
		t.Error("short nmk_ key must fail")
	}
}
