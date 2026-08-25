package handler

import (
	"context"
	"errors"
	"testing"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
)

func TestBatchEntityIDs(t *testing.T) {
	c := &Catalog{}
	_, _, err := c.batchEntityIDs(context.Background(), collect.Query{Batch: true, IDs: []string{"nope"}}, 0)
	var p *problem.Problem
	if !errors.As(err, &p) || p.Code != problem.CodeInvalidParameter {
		t.Fatalf("invalid id: %v", err)
	}
	ids, missing, err := c.batchEntityIDs(context.Background(), collect.Query{
		Batch: true,
		IDs:   []string{"3"},
		Refs:  []repr.Ref{{Source: "vndb", ExternalID: "g1"}},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("ids=%v", ids)
	}
	if len(missing) != 1 || missing[0] != "vndb:g1" {
		t.Fatalf("missing=%v", missing)
	}
}

func TestFinishListCursorAndMissing(t *testing.T) {
	next := "raw-cursor"
	q := collect.Query{IncludeTotal: true}
	out := finishList([]string{"a"}, &next, 4, q, []string{"9"})
	if out.NextCursor == nil || *out.NextCursor != collect.EncodeCursor("raw-cursor") {
		t.Fatalf("cursor=%v", out.NextCursor)
	}
	if out.Total == nil || *out.Total != 4 {
		t.Fatalf("total=%v", out.Total)
	}
	if out.Missing == nil || (*out.Missing)[0] != "9" {
		t.Fatalf("missing=%v", out.Missing)
	}
	batched := finishList([]string{"a"}, &next, 4, collect.Query{Batch: true}, nil)
	if batched.NextCursor != nil || batched.Missing != nil || batched.Total != nil {
		t.Fatalf("batch extras %+v", batched)
	}
}
