package devapi

import (
	"crypto/rand"
	"hash/crc32"
	"math/big"
	"strings"
)

const (
	V2LivePrefix = "nmk_live_"
	V2TestPrefix = "nmk_test_"
	v2BodyLen    = 22
	v2CRCLen     = 6
	V2KeyLen     = 37
)

func GenerateV2Key(live bool) (string, error) {
	prefix := V2LivePrefix
	if !live {
		prefix = V2TestPrefix
	}
	body, err := randomBase62(v2BodyLen)
	if err != nil {
		return "", err
	}
	return prefix + body + crc32Base62(body), nil
}

func IsV2KeyPrefix(raw string) bool {
	return strings.HasPrefix(raw, V2LivePrefix) || strings.HasPrefix(raw, V2TestPrefix)
}

func HasV1KeyPrefix(raw string) bool {
	return strings.HasPrefix(raw, LivePrefix) || strings.HasPrefix(raw, TestPrefix)
}

func ValidV2Key(raw string) bool {
	if len(raw) != V2KeyLen || !IsV2KeyPrefix(raw) {
		return false
	}
	rest := raw[len(V2LivePrefix):]
	body, crc := rest[:v2BodyLen], rest[v2BodyLen:]
	if len(crc) != v2CRCLen {
		return false
	}
	for _, r := range body {
		if !strings.ContainsRune(base62Alphabet, r) {
			return false
		}
	}
	return crc == crc32Base62(body)
}

func randomBase62(n int) (string, error) {
	b := make([]byte, n)
	mod := big.NewInt(int64(len(base62Alphabet)))
	for i := 0; i < n; i++ {
		k, err := rand.Int(rand.Reader, mod)
		if err != nil {
			return "", err
		}
		b[i] = base62Alphabet[k.Int64()]
	}
	return string(b), nil
}

func crc32Base62(body string) string {
	sum := crc32.ChecksumIEEE([]byte(body))
	x := new(big.Int).SetUint64(uint64(sum))
	if x.Sign() == 0 {
		return strings.Repeat("0", v2CRCLen)
	}
	base := big.NewInt(62)
	mod := new(big.Int)
	var buf []byte
	for x.Sign() > 0 {
		x.DivMod(x, base, mod)
		buf = append(buf, base62Alphabet[mod.Int64()])
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	s := string(buf)
	if len(s) > v2CRCLen {
		s = s[len(s)-v2CRCLen:]
	}
	for len(s) < v2CRCLen {
		s = "0" + s
	}
	return s
}
