package handler

import (
	"testing"
	"time"

	"api/internal/platform/apiv2/problem"
	catsvc "api/internal/platform/catalog/service"
)

func TestCalendarWindow(t *testing.T) {
	now := time.Date(2026, 8, 23, 6, 0, 0, 0, time.UTC)
	jstMonth := now.In(calendarJST).Format("2006-01")

	win, err := calendarWindow("", "", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if win.bucket.Kind != catsvc.CalendarMonthBucket || win.bucket.Month == 0 {
		t.Fatalf("default %+v", win.bucket)
	}
	if jstMonth != "2026-08" && jstMonth != "2026-09" {
		t.Fatalf("jst month %s", jstMonth)
	}

	win, err = calendarWindow("2024-06", "", "", "", now)
	if err != nil || win.bucket.Kind != catsvc.CalendarMonthBucket || win.bucket.Year != 2024 || win.bucket.Month != 6 {
		t.Fatalf("month %+v %v", win, err)
	}

	win, err = calendarWindow("", "2020", "", "", now)
	if err != nil || win.bucket.Kind != catsvc.CalendarPendingBucket || win.bucket.Year != 2020 {
		t.Fatalf("year %+v %v", win, err)
	}

	win, err = calendarWindow("", "", "year", "", now)
	if err != nil || win.bucket.Kind != catsvc.CalendarPendingBucket {
		t.Fatalf("precision year %+v %v", win, err)
	}

	win, err = calendarWindow("", "", "", "announced", now)
	if err != nil || win.bucket.Kind != catsvc.CalendarTBABucket {
		t.Fatalf("announced %+v %v", win, err)
	}

	win, err = calendarWindow("", "", "", "unknown", now)
	if err != nil || win.bucket.Kind != catsvc.CalendarTBABucket {
		t.Fatalf("unknown %+v %v", win, err)
	}

	win, err = calendarWindow("2024-06", "", "", "announced", now)
	if err != nil || !win.empty {
		t.Fatalf("dated month has no announced rows %+v %v", win, err)
	}

	win, err = calendarWindow("", "", "", "cancelled", now)
	if err != nil || !win.empty {
		t.Fatalf("cancelled %+v %v", win, err)
	}

	_, err = calendarWindow("2024-13", "", "", "", now)
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("bad month %v", err)
	}
	_, err = calendarWindow("", "", "week", "", now)
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("bad precision %v", err)
	}
	_, err = calendarWindow("", "", "", "tbd", now)
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("bad status %v", err)
	}
}
