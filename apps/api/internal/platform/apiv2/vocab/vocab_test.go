package vocab

import "testing"

func TestLookupRoundTrip(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("no vocabularies")
	}
	seen := map[string]bool{}
	for _, v := range all {
		if v.Object != "vocabulary" || v.Name == "" {
			t.Errorf("bad vocabulary %+v", v)
		}
		if seen[v.Name] {
			t.Errorf("duplicate vocabulary %s", v.Name)
		}
		seen[v.Name] = true
		if v.Values == nil {
			t.Errorf("%s values is nil", v.Name)
		}
		got, ok := Lookup(v.Name)
		if !ok || got.Name != v.Name || len(got.Values) != len(v.Values) {
			t.Errorf("lookup %s = %+v ok=%v", v.Name, got, ok)
		}
		valSeen := map[string]bool{}
		for _, item := range v.Values {
			if item.Value == "" || item.DisplayName == "" {
				t.Errorf("%s has an empty value/display_name", v.Name)
			}
			if valSeen[item.Value] {
				t.Errorf("%s duplicates value %s", v.Name, item.Value)
			}
			valSeen[item.Value] = true
		}
	}
	if _, ok := Lookup("not-a-vocab"); ok {
		t.Fatal("unknown vocabulary looked up")
	}
	src, ok := Lookup("sources")
	if !ok || src.Closed {
		t.Fatalf("sources should be open: %+v", src)
	}
	medium, ok := Lookup("medium")
	if !ok || !medium.Closed || len(medium.Values) != 7 {
		t.Fatalf("medium: %+v ok=%v", medium, ok)
	}
}
