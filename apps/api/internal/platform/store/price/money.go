package price

import (
	"fmt"
	"math"
	"strconv"
)

func Minor(f float64) int64 {
	return int64(math.Round(f * 100))
}

func FormatMinor(m int64) string {
	sign := ""
	n := m
	if n < 0 {
		sign = "-"
		n = -n
	}
	return sign + strconv.FormatInt(n/100, 10) + fmt.Sprintf(".%02d", n%100)
}
