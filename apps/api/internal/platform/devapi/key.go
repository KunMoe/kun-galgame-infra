package devapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
	"strings"
)

const (
	LivePrefix = "nm_live_"
	TestPrefix = "nm_test_"

	keyHashPrefix  = "sha256:"
	keyRandomBytes = 24
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func GenerateKey(prefix string) (string, error) {
	b := make([]byte, keyRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base62Encode(b), nil
}

func HashKey(plaintext string) string {
	return keyHashPrefix + hashHex(plaintext)
}

func hashHex(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func VerifyKeyHash(presented, storedHash string) bool {
	h, ok := strings.CutPrefix(storedHash, keyHashPrefix)
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashHex(presented)), []byte(h)) == 1
}

func KeyMetadata(plaintext string) (prefix, last4 string) {
	env, body := "", plaintext
	switch {
	case strings.HasPrefix(plaintext, V2LivePrefix):
		env, body = V2LivePrefix, plaintext[len(V2LivePrefix):]
	case strings.HasPrefix(plaintext, V2TestPrefix):
		env, body = V2TestPrefix, plaintext[len(V2TestPrefix):]
	case strings.HasPrefix(plaintext, LivePrefix):
		env, body = LivePrefix, plaintext[len(LivePrefix):]
	case strings.HasPrefix(plaintext, TestPrefix):
		env, body = TestPrefix, plaintext[len(TestPrefix):]
	}
	prefix = env
	if len(body) >= 4 {
		prefix += body[:4]
	} else {
		prefix += body
	}
	last4 = plaintext
	if len(plaintext) >= 4 {
		last4 = plaintext[len(plaintext)-4:]
	}
	return prefix, last4
}

func HasKeyPrefix(raw string) bool {
	return HasV1KeyPrefix(raw) || IsV2KeyPrefix(raw)
}

func base62Encode(b []byte) string {
	n := new(big.Int).SetBytes(b)
	if n.Sign() == 0 {
		return "0"
	}
	base := big.NewInt(62)
	mod := new(big.Int)
	buf := make([]byte, 0, 33)
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		buf = append(buf, base62Alphabet[mod.Int64()])
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
