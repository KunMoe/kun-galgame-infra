package handler

import (
	"context"
	"strings"
	"time"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/parse"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catsvc "api/internal/platform/catalog/service"
)

var calendarJST = time.FixedZone("Asia/Tokyo", 9*60*60)

var calendarPrecision = []string{"day", "month", "year"}
var calendarStatus = []string{"released", "dated", "announced", "cancelled", "unknown"}

func (c *Catalog) ListCalendar(ctx context.Context, q collect.Query, month, year, precision, status string) (repr.List[repr.Work], error) {
	if c == nil || c.Public == nil {
		return repr.List[repr.Work]{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	if q.Batch {
		return repr.List[repr.Work]{}, feedNoBatch("calendar")
	}
	win, err := calendarWindow(month, year, precision, status, time.Now())
	if err != nil {
		return repr.List[repr.Work]{}, err
	}
	if win.empty {
		return finishList([]repr.Work{}, nil, 0, q, nil), nil
	}
	f := catsvc.CalendarFilter{NSFW: q.NSFW, Include: calendarWorksInclude(q.Include)}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	data, lerr := c.Public.CalendarPage(ctx, win.bucket, f, q.Cursor, limit)
	if lerr != nil {
		return repr.List[repr.Work]{}, listCursorErr(lerr)
	}
	items := make([]repr.Work, 0, len(data.Items))
	for _, it := range data.Items {
		items = append(items, workFromListItem(it, q.Include))
	}
	var total int64
	if q.IncludeTotal {
		n, _, merr := c.Public.CalendarMeta(ctx, win.bucket, f)
		if merr != nil {
			return repr.List[repr.Work]{}, merr
		}
		total = n
	}
	return finishList(items, data.NextCursor, total, q, nil), nil
}

type calendarWin struct {
	bucket catsvc.CalendarBucket
	empty  bool
}

func calendarWindow(month, year, precision, status string, now time.Time) (calendarWin, *problem.Problem) {
	if precision != "" {
		if _, err := parse.Enum(precision, "precision", calendarPrecision); err != nil {
			return calendarWin{}, err
		}
	}
	if status != "" {
		if _, err := parse.Enum(status, "status", calendarStatus); err != nil {
			return calendarWin{}, err
		}
	}
	if status == "cancelled" {
		return calendarWin{empty: true}, nil
	}

	jst := now.In(calendarJST)
	month = strings.TrimSpace(month)
	year = strings.TrimSpace(year)

	undated := status == "announced" || status == "unknown"
	if undated && month == "" && precision != "day" && precision != "month" {
		return calendarWin{bucket: catsvc.CalendarBucket{Kind: catsvc.CalendarTBABucket}}, nil
	}
	if undated && month != "" {
		return calendarWin{empty: true}, nil
	}

	if precision == "year" || (year != "" && month == "") {
		y := jst.Year()
		if year != "" {
			t, perr := time.Parse("2006", year)
			if perr != nil || len(year) != 4 {
				p := problem.New(problem.CodeInvalidParameter, "", "", "year must be YYYY.")
				p.Errors = []problem.FieldError{{Parameter: "year", Reason: problem.ReasonInvalidFormat, Detail: "expected YYYY"}}
				return calendarWin{}, p
			}
			y = t.Year()
		}
		return calendarWin{bucket: catsvc.CalendarBucket{Kind: catsvc.CalendarPendingBucket, Year: y}}, nil
	}

	m := jst.Format("2006-01")
	if month != "" {
		m = month
	}
	t, perr := time.Parse("2006-01", m)
	if perr != nil || len(m) != 7 {
		p := problem.New(problem.CodeInvalidParameter, "", "", "month must be YYYY-MM.")
		p.Errors = []problem.FieldError{{Parameter: "month", Reason: problem.ReasonInvalidFormat, Detail: "expected YYYY-MM"}}
		return calendarWin{}, p
	}
	return calendarWin{bucket: catsvc.CalendarBucket{
		Kind: catsvc.CalendarMonthBucket, Year: t.Year(), Month: int(t.Month()),
	}}, nil
}

func calendarWorksInclude(tokens []string) catsvc.WorksListInclude {
	var raw []string
	for _, t := range tokens {
		switch t {
		case "companies":
			raw = append(raw, "labels")
		case "intros", "ratings", "covers", "refs":
			raw = append(raw, t)
		}
	}
	return catsvc.ParseWorksListInclude(strings.Join(raw, ","))
}
