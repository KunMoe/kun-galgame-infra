package collect

import (
	"testing"

	"api/internal/platform/apiv2/problem"
)

func TestWorkFullSetIsSubsetOfInclude(t *testing.T) {
	allowed := map[string]bool{}
	for _, tkn := range WorkInclude {
		allowed[tkn] = true
	}
	if len(WorkInclude) != 18 {
		t.Fatalf("include vocab %d, want 18", len(WorkInclude))
	}
	for _, tkn := range WorkFullSet {
		if !allowed[tkn] {
			t.Fatalf("FULL_SET token %s is not in include vocab", tkn)
		}
	}
}

func TestParseViewIncludeUnion(t *testing.T) {
	q, err := Parse(Raw{View: "full", Include: "characters"}, WorkSpec())
	if err != nil {
		t.Fatal(err)
	}
	if q.View != "full" {
		t.Fatalf("view=%s", q.View)
	}
	if !contains(q.Include, "titles") || !contains(q.Include, "characters") {
		t.Fatalf("full ∪ include: %v", q.Include)
	}
	_, err = Parse(Raw{View: "compact"}, WorkSpec())
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("bad view: %+v", err)
	}
	_, err = Parse(Raw{Include: "nope"}, WorkSpec())
	if err == nil || err.Code != problem.CodeUnknownInclude {
		t.Fatalf("bad include: %+v", err)
	}
}

func TestParseLimitAndCursorExclusiveWithIDs(t *testing.T) {
	_, err := Parse(Raw{Limit: "101"}, WorkSpec())
	if err == nil || err.Code != problem.CodeLimitTooLarge {
		t.Fatalf("limit: %+v", err)
	}
	_, err = Parse(Raw{IDs: "1,2", Cursor: EncodeCursor("1")}, WorkSpec())
	if err == nil || err.Code != problem.CodeMutuallyExclusiveParameters {
		t.Fatalf("ids+cursor: %+v", err)
	}
	q, err := Parse(Raw{IDs: "1,2", Limit: "5"}, WorkSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !q.Batch || q.Limit != 0 {
		t.Fatalf("batch must ignore limit: %+v", q)
	}
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "x"
	}
	raw := ids[0]
	for i := 1; i < 101; i++ {
		raw += "," + ids[i]
	}
	_, err = Parse(Raw{IDs: raw}, WorkSpec())
	if err == nil || err.Code != problem.CodeTooManyIDs {
		t.Fatalf("101 ids: %+v", err)
	}
}

func TestParseFieldsCanonicalAndUnknown(t *testing.T) {
	q, err := Parse(Raw{Fields: "cover,id,display_name,cover"}, WorkSpec())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Fields) < 3 || q.Fields[0] != "cover" && q.Fields[0] != "display_name" && q.Fields[0] != "id" && q.Fields[0] != "object" {
		// just check object and id injected and cover not duplicated
	}
	seen := map[string]int{}
	for _, f := range q.Fields {
		seen[f]++
		if seen[f] > 1 {
			t.Fatalf("duplicate field %s", f)
		}
	}
	if seen["object"] != 1 || seen["id"] != 1 || seen["cover"] != 1 {
		t.Fatalf("fields=%v", q.Fields)
	}
	for i := 1; i < len(q.Fields); i++ {
		if q.Fields[i-1] > q.Fields[i] {
			t.Fatalf("fields not sorted: %v", q.Fields)
		}
	}
	_, err = Parse(Raw{Fields: "nope"}, WorkSpec())
	if err == nil || err.Code != problem.CodeUnknownField {
		t.Fatalf("unknown field: %+v", err)
	}
}

func TestParseRefsAndIncludeTotal(t *testing.T) {
	q, err := Parse(Raw{Refs: "vndb:v19658,dlsite:RJ01234567", IncludeTotal: "true"}, WorkSpec())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Refs) != 2 || q.Refs[0].Source != "vndb" || q.Refs[1].ExternalID != "RJ01234567" {
		t.Fatalf("refs=%v", q.Refs)
	}
	if !q.IncludeTotal || !q.Batch {
		t.Fatalf("total/batch %+v", q)
	}
	_, err = Parse(Raw{Refs: "not-a-ref"}, WorkSpec())
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("bad ref: %+v", err)
	}
	_, err = Parse(Raw{IncludeTotal: "yes"}, WorkSpec())
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("include_total: %+v", err)
	}
}
