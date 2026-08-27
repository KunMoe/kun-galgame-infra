package model

import "time"

// tzdata is deliberately not imported: JST has no daylight saving, so a fixed
// +09:00 zone is the whole definition and the binary needs no zone database.
func JST() *time.Location { return time.FixedZone("JST", 9*3600) }

func JSTDay(t time.Time) string { return t.In(JST()).Format("2006-01-02") }

func ParseJSTDay(raw string) (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", raw, JST())
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// DaySpan counts the closed interval [from, to] in days; 1 when they are equal.
func DaySpan(from, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
}
