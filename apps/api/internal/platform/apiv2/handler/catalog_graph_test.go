package handler

import (
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/dto"
	"api/pkg/imageclient"
)

func TestCompanyGraphFrom(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	in := dto.PublicLabelGraph{
		Nodes: []dto.PublicLabelGraphNode{
			{ID: 1, DisplayName: "Alcot", WorkCount: 3, LogoHash: hash},
			{ID: 4, DisplayName: "No Logo", WorkCount: 1},
		},
		Edges: []dto.PublicLabelGraphEdge{
			{From: 1, To: 2, Relation: "parent"},
			{From: 1, To: 3, Relation: "subsidiary"},
		},
	}
	url := func(h string) string { return imageclient.MainURL("https://img.example.test/image", h, "webp") }

	g := companyGraphFrom(in, nil, url)
	if g.Object != "company_graph" || len(g.Nodes) != 2 || g.Nodes[0].ID != "1" {
		t.Fatalf("%+v", g)
	}
	if g.Nodes[0].Logo != nil {
		t.Fatalf("logo without include=logo: %+v", g.Nodes[0].Logo)
	}
	if len(g.Edges) != 1 || g.Edges[0].FromID != "1" || g.Edges[0].ToID != "2" || g.Edges[0].Relation != "parent" {
		t.Fatalf("edges %+v", g.Edges)
	}

	g = companyGraphFrom(in, []string{"logo"}, url)
	if g.Nodes[0].Logo == nil || g.Nodes[0].Logo.Hash != hash {
		t.Fatalf("logo %+v", g.Nodes[0].Logo)
	}
	if g.Nodes[1].Logo != nil {
		t.Fatalf("logo-less node carries one: %+v", g.Nodes[1].Logo)
	}
}

func TestCompanyGraphUnbound(t *testing.T) {
	_, err := (*Catalog)(nil).GetCompanyGraph(t.Context(), 1, false, nil)
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}
