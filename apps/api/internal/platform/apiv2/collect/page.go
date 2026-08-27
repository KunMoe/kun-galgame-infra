package collect

import (
	"encoding/base64"
	"strconv"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
)

func EncodeCursor(key string) string {
	if key == "" {
		return ""
	}
	return repr.Cursor(base64.RawURLEncoding.EncodeToString([]byte(key)))
}

func DecodeCursor(cur string) (string, *problem.Problem) {
	payload, ok := repr.ParseCursor(cur)
	if !ok {
		return "", invalidCursor()
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(b) == 0 {
		return "", invalidCursor()
	}
	return string(b), nil
}

func EncodeOffset(n int) *string {
	if n <= 0 {
		return nil
	}
	e := EncodeCursor(strconv.Itoa(n))
	return &e
}

func DecodeOffset(cur string) (int, *problem.Problem) {
	if cur == "" {
		return 0, nil
	}
	s, err := DecodeCursor(cur)
	if err != nil {
		return 0, err
	}
	n, conv := strconv.Atoi(s)
	if conv != nil || n < 0 {
		return 0, invalidCursor()
	}
	return n, nil
}

func invalidCursor() *problem.Problem {
	p := problem.New(problem.CodeInvalidCursor, "", "", "cursor could not be parsed or is no longer valid.")
	p.Errors = []problem.FieldError{{
		Parameter: "cursor",
		Reason:    problem.ReasonInvalidFormat,
		Detail:    "pass the next_cursor from a previous page of this collection",
	}}
	return p
}

func Slice[T any](all []T, q Query, key func(T) string) repr.List[T] {
	filtered := all
	var missing []string
	if q.Batch {
		filtered, missing = selectIDs(all, q.IDs, key)
		out := repr.NewList(filtered, nil)
		if q.IncludeTotal {
			n := int64(len(filtered))
			out.Total = &n
		}
		if missing != nil {
			m := missing
			out.Missing = &m
		}
		return out
	}

	items, next, err := after(filtered, q.Cursor, q.Limit, key)
	if err != nil {
		return repr.List[T]{}
	}
	out := repr.NewList(items, next)
	if q.IncludeTotal {
		n := int64(len(filtered))
		out.Total = &n
	}
	return out
}

func SliceErr[T any](all []T, q Query, key func(T) string) (repr.List[T], *problem.Problem) {
	if q.Batch {
		return Slice(all, q, key), nil
	}
	if q.Cursor != "" {
		found := false
		for _, it := range all {
			if key(it) == q.Cursor {
				found = true
				break
			}
		}
		if !found {
			return repr.List[T]{}, invalidCursor()
		}
	}
	return Slice(all, q, key), nil
}

func after[T any](all []T, cursorKey string, limit int, key func(T) string) ([]T, *string, *problem.Problem) {
	start := 0
	if cursorKey != "" {
		found := false
		for i, it := range all {
			if key(it) == cursorKey {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return nil, nil, invalidCursor()
		}
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	rest := all[start:]
	if len(rest) <= limit {
		return rest, nil, nil
	}
	page := rest[:limit]
	c := EncodeCursor(key(page[len(page)-1]))
	return page, &c, nil
}

func selectIDs[T any](all []T, ids []string, key func(T) string) ([]T, []string) {
	idx := make(map[string]T, len(all))
	for _, it := range all {
		idx[key(it)] = it
	}
	items := make([]T, 0, len(ids))
	var missing []string
	for _, id := range ids {
		if it, ok := idx[id]; ok {
			items = append(items, it)
		} else {
			missing = append(missing, id)
		}
	}
	return items, missing
}
