package handler

import (
	"context"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
	catmodel "api/internal/platform/catalog/model"
)

func (c *Catalog) GetCompanyGraph(ctx context.Context, id int64, nsfw bool, include []string) (repr.CompanyGraph, error) {
	if c == nil || c.Public == nil {
		return repr.CompanyGraph{}, problem.New(problem.CodeServiceUnavailable, "", "", "catalog read is not bound.")
	}
	g, found, err := c.Public.LabelRelationGraph(ctx, id, nsfw)
	if err != nil {
		return repr.CompanyGraph{}, err
	}
	if !found {
		return repr.CompanyGraph{}, c.mergedOrNotFound(ctx, catmodel.EntityTypeLabel, "company", id)
	}
	return companyGraphFrom(g, include, c.imageURL), nil
}

func companyGraphFrom(g dto.PublicLabelGraph, include []string, logoURL func(string) string) repr.CompanyGraph {
	wantLogo := false
	for _, t := range include {
		if t == "logo" {
			wantLogo = true
		}
	}
	nodes := make([]repr.CompanyGraphNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		node := repr.CompanyGraphNode{
			Object: "company", ID: repr.ID(n.ID), DisplayName: n.DisplayName,
			Localized: localizedFrom(n.Localized), WorkCount: n.WorkCount,
		}
		if wantLogo && n.LogoHash != "" && logoURL != nil {
			node.Logo = imageFromPublicMeta(logoURL(n.LogoHash), n.LogoMeta, "")
		}
		nodes = append(nodes, node)
	}
	edges := make([]repr.CompanyGraphEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		rel := e.Relation
		switch rel {
		case "parent", "imprint", "spawned", "succeeded_by":
		default:
			continue
		}
		edges = append(edges, repr.CompanyGraphEdge{
			FromID: repr.ID(e.From), ToID: repr.ID(e.To), Relation: rel,
		})
	}
	return repr.CompanyGraph{Object: "company_graph", Nodes: nodes, Edges: edges}
}
