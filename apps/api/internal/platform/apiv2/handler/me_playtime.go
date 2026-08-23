package handler

import (
	"context"
	"strings"
	"time"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/model"
	catsvc "api/internal/platform/catalog/service"
)

func (c *Catalog) ListPlaytimes(ctx context.Context, q collect.Query, workIDs []string) (repr.List[repr.UserPlaytime], error) {
	if c == nil || c.Playtime == nil {
		return repr.List[repr.UserPlaytime]{}, problem.New(problem.CodeServiceUnavailable, "", "", "playtimes are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.List[repr.UserPlaytime]{}, err
	}
	if len(workIDs) > 0 {
		if len(workIDs) > 100 {
			return repr.List[repr.UserPlaytime]{}, problem.New(problem.CodeTooManyIDs, "", "", "work_ids named more than 100 items.")
		}
		items := make([]repr.UserPlaytime, 0, len(workIDs))
		var missing []string
		for _, s := range workIDs {
			id, ok := repr.ParseID(s)
			if !ok {
				p := problem.New(problem.CodeInvalidParameter, "", "", "work_ids values must be decimal catalog ids.")
				p.Errors = []problem.FieldError{{Parameter: "work_ids", Reason: problem.ReasonInvalidFormat, Detail: s}}
				return repr.List[repr.UserPlaytime]{}, p
			}
			row, gerr := c.Playtime.GetMine(ctx, uid, id)
			if gerr != nil {
				return repr.List[repr.UserPlaytime]{}, gerr
			}
			if row == nil {
				missing = append(missing, s)
				continue
			}
			items = append(items, repr.UserPlaytime{Object: "playtime", WorkID: repr.ID(row.WorkID), Minutes: row.Minutes})
		}
		return finishList(items, nil, 0, collect.Query{Batch: true, IncludeTotal: false}, missing), nil
	}
	since := time.Time{}
	if q.Cursor != "" {
		t, perr := time.Parse(time.RFC3339, q.Cursor)
		if perr != nil {
			return repr.List[repr.UserPlaytime]{}, collectInvalidCursor()
		}
		since = t
	}
	limit := q.Limit
	if limit <= 0 {
		limit = collect.DefaultLimit
	}
	rows, lerr := c.Playtime.ListMine(ctx, uid, since, limit+1)
	if lerr != nil {
		return repr.List[repr.UserPlaytime]{}, lerr
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		s := rows[len(rows)-1].UpdatedAt.UTC().Format(time.RFC3339)
		next = &s
	}
	items := make([]repr.UserPlaytime, 0, len(rows))
	for _, r := range rows {
		items = append(items, repr.UserPlaytime{Object: "playtime", WorkID: repr.ID(r.WorkID), Minutes: r.Minutes})
	}
	return finishList(items, next, 0, q, nil), nil
}

func (c *Catalog) GetPlaytime(ctx context.Context, workID int64) (repr.UserPlaytime, error) {
	if c == nil || c.Playtime == nil {
		return repr.UserPlaytime{}, problem.New(problem.CodeServiceUnavailable, "", "", "playtimes are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return repr.UserPlaytime{}, err
	}
	row, gerr := c.Playtime.GetMine(ctx, uid, workID)
	if gerr != nil {
		return repr.UserPlaytime{}, gerr
	}
	if row == nil {
		return repr.UserPlaytime{}, problem.New(problem.CodeNotFound, "", "", "No playtime on this work.")
	}
	return repr.UserPlaytime{Object: "playtime", WorkID: repr.ID(row.WorkID), Minutes: row.Minutes}, nil
}

func (c *Catalog) PutPlaytime(ctx context.Context, workID int64, minutes int) (repr.UserPlaytime, error) {
	if c == nil || c.Playtime == nil {
		return repr.UserPlaytime{}, problem.New(problem.CodeServiceUnavailable, "", "", "playtimes are not bound.")
	}
	uid, client, err := requireUser(ctx)
	if err != nil {
		return repr.UserPlaytime{}, err
	}
	rec, gerr := c.Playtime.Report(ctx, catsvc.PlaytimeReport{
		ActorUID: uid, WorkID: workID, ClientID: client, Minutes: minutes,
		Status: model.PlaytimeStatusPlaying,
	})
	if gerr != nil {
		return repr.UserPlaytime{}, playtimeErr(gerr)
	}
	return repr.UserPlaytime{Object: "playtime", WorkID: repr.ID(rec.WorkID), Minutes: rec.Minutes}, nil
}

func (c *Catalog) DeletePlaytime(ctx context.Context, workID int64) error {
	if c == nil || c.Playtime == nil {
		return problem.New(problem.CodeServiceUnavailable, "", "", "playtimes are not bound.")
	}
	uid, _, err := requireUser(ctx)
	if err != nil {
		return err
	}
	return playtimeErr(c.Playtime.DeleteMine(ctx, uid, workID))
}

func playtimeErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case err == catsvc.ErrPlaytimeMinutesRange:
		p := problem.New(problem.CodeValidationFailed, "", "", "minutes is out of range.")
		p.Errors = []problem.FieldError{{Pointer: "/minutes", Reason: problem.ReasonOutOfRange, Detail: err.Error()}}
		return p
	case err == catsvc.ErrPlaytimeWorkUnavailable:
		return problem.New(problem.CodeNotFound, "", "", "work is not available for playtime.")
	case err == catsvc.ErrPlaytimeActorRequired, err == catsvc.ErrPlaytimeClientRequired:
		return problem.New(problem.CodeUserIdentityRequired, "", "", err.Error())
	}
	return err
}

func splitWorkIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
