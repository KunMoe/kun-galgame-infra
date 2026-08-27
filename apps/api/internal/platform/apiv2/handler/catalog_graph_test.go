package handler

import (
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/dto"
)

func TestCompanyGraphFrom(t *testing.T) {
	g := companyGraphFrom(dto.PublicLabelGraph{
		Nodes: []dto.PublicLabelGraphNode{
			{ID: 1, DisplayName: "Alcot", WorkCount: 3},
		},
		Edges: []dto.PublicLabelGraphEdge{
			{From: 1, To: 2, Relation: "parent"},
			{From: 1, To: 3, Relation: "subsidiary"},
		},
	})
	if g.Object != "company_graph" || len(g.Nodes) != 1 || g.Nodes[0].ID != "1" {
		t.Fatalf("%+v", g)
	}
	if len(g.Edges) != 1 || g.Edges[0].FromID != "1" || g.Edges[0].ToID != "2" || g.Edges[0].Relation != "parent" {
		t.Fatalf("edges %+v", g.Edges)
	}
}

func TestCompanyGraphUnbound(t *testing.T) {
	_, err := (*Catalog)(nil).GetCompanyGraph(t.Context(), 1, false)
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}
