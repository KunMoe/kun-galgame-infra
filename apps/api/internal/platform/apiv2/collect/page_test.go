package collect

import (
	"encoding/json"
	"testing"

	"api/internal/platform/apiv2/problem"
)

func TestSliceOverfetchOmitsCursorOnExactLastPage(t *testing.T) {
	all := []string{"a", "b", "c", "d"}
	key := func(s string) string { return s }

	q, err := Parse(Raw{Limit: "2"}, VocabSpec())
	if err != nil {
		t.Fatal(err)
	}
	page, perr := SliceErr(all, q, key)
	if perr != nil {
		t.Fatal(perr)
	}
	if len(page.Items) != 2 || page.Items[0] != "a" || page.NextCursor == nil {
		t.Fatalf("first page %+v", page)
	}

	q2, err := Parse(Raw{Limit: "2", Cursor: *page.NextCursor}, VocabSpec())
	if err != nil {
		t.Fatal(err)
	}
	page2, perr := SliceErr(all, q2, key)
	if perr != nil {
		t.Fatal(perr)
	}
	if len(page2.Items) != 2 || page2.Items[0] != "c" {
		t.Fatalf("second page %+v", page2)
	}
	if page2.NextCursor != nil {
		t.Fatalf("exact last page must omit next_cursor, got %v", *page2.NextCursor)
	}
}

func TestSliceInvalidCursor(t *testing.T) {
	all := []string{"a", "b"}
	q, err := Parse(Raw{Cursor: EncodeCursor("nope")}, VocabSpec())
	if err != nil {
		t.Fatal(err)
	}
	_, perr := SliceErr(all, q, func(s string) string { return s })
	if perr == nil || perr.Code != problem.CodeInvalidCursor {
		t.Fatalf("stale cursor: %+v", perr)
	}
	_, err = Parse(Raw{Cursor: "not-a-cursor"}, VocabSpec())
	if err == nil || err.Code != problem.CodeInvalidCursor {
		t.Fatalf("malformed: %+v", err)
	}
}

func TestSliceBatchMissing(t *testing.T) {
	all := []string{"a", "c"}
	q, err := Parse(Raw{IDs: "a,b,c"}, VocabSpec())
	if err != nil {
		t.Fatal(err)
	}
	page, perr := SliceErr(all, q, func(s string) string { return s })
	if perr != nil {
		t.Fatal(perr)
	}
	if page.NextCursor != nil {
		t.Fatal("batch must not paginate")
	}
	if len(page.Items) != 2 || page.Items[0] != "a" || page.Items[1] != "c" {
		t.Fatalf("items=%v", page.Items)
	}
	if page.Missing == nil || len(*page.Missing) != 1 || (*page.Missing)[0] != "b" {
		t.Fatalf("missing=%v", page.Missing)
	}
}

func TestEncodeDecodeOffset(t *testing.T) {
	if EncodeOffset(0) != nil {
		t.Fatal("zero offset omits cursor")
	}
	c := EncodeOffset(20)
	if c == nil {
		t.Fatal("offset 20")
	}
	n, err := DecodeOffset(*c)
	if err != nil || n != 20 {
		t.Fatalf("roundtrip %d %v", n, err)
	}
	if _, err := DecodeOffset("cur_nope"); err == nil {
		t.Fatal("bad cursor")
	}
}

func TestSliceIncludeTotal(t *testing.T) {
	all := []string{"a", "b", "c"}
	q, err := Parse(Raw{Limit: "1", IncludeTotal: "true"}, VocabSpec())
	if err != nil {
		t.Fatal(err)
	}
	page, perr := SliceErr(all, q, func(s string) string { return s })
	if perr != nil {
		t.Fatal(perr)
	}
	if page.Total == nil || *page.Total != 3 {
		t.Fatalf("total=%v", page.Total)
	}
	q2, _ := Parse(Raw{Limit: "1"}, VocabSpec())
	page2, _ := SliceErr(all, q2, func(s string) string { return s })
	if page2.Total != nil {
		t.Fatal("total must be omitted by default")
	}
}

func TestApplyFieldsTrimsListItems(t *testing.T) {
	body := []byte(`{"object":"list","next_cursor":"cur_x","items":[` +
		`{"object":"work","id":"1","name":"n","extra":"x"}]}`)
	out, err := ApplyFields(body, []string{"id", "name"})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	if root["next_cursor"] != "cur_x" {
		t.Fatalf("the list envelope is not an item and must survive: %v", root)
	}
	items := root["items"].([]any)
	it := items[0].(map[string]any)
	if _, ok := it["extra"]; ok {
		t.Fatalf("extra survived: %v", it)
	}
	if it["id"] != "1" || it["object"] != "work" || it["name"] != "n" {
		t.Fatalf("kept keys: %v", it)
	}
}

func TestApplyFieldsAbsentIsByteIdentical(t *testing.T) {
	body := []byte(`{"object":"list","items":[{"object":"work","id":"1","name":"n"}]}`)
	out, err := ApplyFields(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("no fields= must not reshape the body:\n got %s\nwant %s", out, body)
	}
}
