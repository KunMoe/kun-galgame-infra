package mcpface

import (
	"encoding/json"
	"strings"
)

type specDoc struct {
	Paths map[string]specPathItem `json:"paths"`
}

type specPathItem struct {
	Get        *specOp     `json:"get"`
	Parameters []specParam `json:"parameters"`
}

type specOp struct {
	OperationID string      `json:"operationId"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Parameters  []specParam `json:"parameters"`
}

type specParam struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

type ToolDesc struct {
	Name        string
	Path        string
	Description string
	Params      []string
	Required    []string
	NeedsKey    bool
}

func mcpToolPrefixes(path string) bool {
	return strings.HasPrefix(path, "/v2/catalog") ||
		strings.HasPrefix(path, "/v2/news") ||
		strings.HasPrefix(path, "/v2/problems") ||
		strings.HasPrefix(path, "/v2/vocabularies")
}

func httpNeedsKey(path string) bool {
	if strings.HasPrefix(path, "/v2/problems") || strings.HasPrefix(path, "/v2/vocabularies") {
		return false
	}
	if strings.HasPrefix(path, "/v2/news") {
		return false
	}
	if path == "/v2/catalog/stats" || strings.HasPrefix(path, "/v2/catalog/schemas/") {
		return false
	}
	return strings.HasPrefix(path, "/v2/catalog/")
}

func ToolsFromSpec(raw []byte) ([]ToolDesc, error) {
	var doc specDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := make([]ToolDesc, 0, len(doc.Paths))
	for path, item := range doc.Paths {
		if item.Get == nil || !mcpToolPrefixes(path) {
			continue
		}
		op := item.Get
		name := op.OperationID
		if name == "" {
			continue
		}
		desc := strings.TrimSpace(op.Description)
		if desc == "" {
			desc = strings.TrimSpace(op.Summary)
		}
		params := append([]specParam{}, item.Parameters...)
		params = append(params, op.Parameters...)
		td := ToolDesc{
			Name: name, Path: path, Description: desc, NeedsKey: httpNeedsKey(path),
		}
		seen := map[string]bool{}
		for _, p := range params {
			if p.Name == "" || p.In == "header" || seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			td.Params = append(td.Params, p.Name)
			if p.Required || p.In == "path" {
				td.Required = append(td.Required, p.Name)
			}
		}
		out = append(out, td)
	}
	return out, nil
}
