package problem

import (
	"net/http"
	"testing"
)

func TestMergedLinkHeader(t *testing.T) {
	p := Merged("work", "5678", "req_01AAAAAAAAAAAAAAAAAAAAAAAA", "/v2/catalog/works/1234",
		"work 1234 was merged into work 5678.")
	if p.Code != CodeEntityMerged || p.Status != http.StatusNotFound || p.Object != "work" || p.CurrentID != "5678" {
		t.Fatalf("%+v", p)
	}
	h := p.GetHeaders()
	if h == nil || h.Get("Link") != `</v2/catalog/works/5678>; rel="canonical"` {
		t.Fatalf("Link=%v", h)
	}
	p = Merged("credit_name", "9", "", "", "")
	if p.GetHeaders().Get("Link") != `</v2/catalog/credit-names/9>; rel="canonical"` {
		t.Fatalf("credit-name Link=%v", p.GetHeaders())
	}
	p = Merged("company", "3", "", "", "")
	if p.GetHeaders().Get("Link") != `</v2/catalog/companies/3>; rel="canonical"` {
		t.Fatalf("company Link=%v", p.GetHeaders())
	}
}
