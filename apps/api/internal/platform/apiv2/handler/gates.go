package handler

import (
	"fmt"
	"strings"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/vocab"

	"github.com/danielgtaylor/huma/v2"
)

func CheckG1toG5(doc *huma.OpenAPI) []string {
	var errs []string
	errs = append(errs, checkG2(doc)...)
	errs = append(errs, checkG3(doc)...)
	errs = append(errs, checkG4(doc)...)
	errs = append(errs, checkG5(doc)...)
	return errs
}

func checkG2(doc *huma.OpenAPI) []string {
	var errs []string
	if doc.Info == nil || strings.TrimSpace(doc.Info.Description) == "" {
		errs = append(errs, "G2: info.description is empty")
	}
	for path, item := range doc.Paths {
		for _, op := range pathOps(item) {
			if strings.TrimSpace(op.Description) == "" && strings.TrimSpace(op.Summary) == "" {
				errs = append(errs, fmt.Sprintf("G2: %s %s has no description or summary", op.Method, path))
			}
			for _, p := range op.Parameters {
				if p == nil {
					continue
				}
				if strings.TrimSpace(p.Description) == "" {
					errs = append(errs, fmt.Sprintf("G2: %s %s parameter %s has no description", opMethod(op), path, p.Name))
				}
				errs = append(errs, schemaG2(fmt.Sprintf("%s %s param %s", opMethod(op), path, p.Name), p.Schema)...)
			}
			for status, resp := range op.Responses {
				if resp == nil {
					continue
				}
				if strings.TrimSpace(resp.Description) == "" {
					errs = append(errs, fmt.Sprintf("G2: %s %s response %s has no description", opMethod(op), path, status))
				}
				for mt, content := range resp.Content {
					if content == nil {
						continue
					}
					errs = append(errs, schemaG2(fmt.Sprintf("%s %s %s %s", opMethod(op), path, status, mt), content.Schema)...)
				}
			}
		}
	}
	if doc.Components != nil && doc.Components.Schemas != nil {
		for name, schema := range doc.Components.Schemas.Map() {
			errs = append(errs, schemaG2("schema "+name, schema)...)
		}
	}
	return errs
}

func schemaG2(where string, s *huma.Schema) []string {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		return nil
	}
	var errs []string
	for name, prop := range s.Properties {
		if prop == nil {
			continue
		}
		if prop.Ref == "" && strings.TrimSpace(prop.Description) == "" {
			errs = append(errs, fmt.Sprintf("G2: %s property %s has no description", where, name))
		}
		errs = append(errs, schemaG2(where+"."+name, prop)...)
	}
	if s.Items != nil {
		errs = append(errs, schemaG2(where+"[]", s.Items)...)
	}
	for i, x := range s.OneOf {
		errs = append(errs, schemaG2(fmt.Sprintf("%s.oneOf[%d]", where, i), x)...)
	}
	for i, x := range s.AnyOf {
		errs = append(errs, schemaG2(fmt.Sprintf("%s.anyOf[%d]", where, i), x)...)
	}
	if len(s.Enum) > 0 {
		for _, v := range s.Enum {
			if strings.TrimSpace(fmt.Sprint(v)) == "" {
				errs = append(errs, fmt.Sprintf("G2: %s has an empty enum value", where))
			}
		}
	}
	return errs
}

func checkG3(doc *huma.OpenAPI) []string {
	var errs []string
	seen := map[*huma.Schema]bool{}
	visit := func(s *huma.Schema) {
		walkSchema(s, seen, func(schema *huma.Schema) {
			if schema == nil || schema.Ref != "" || len(schema.Enum) == 0 {
				return
			}
			closed, hasClosed := false, false
			name := ""
			if schema.Extensions != nil {
				if v, ok := schema.Extensions["x-vocabulary-closed"]; ok {
					hasClosed = true
					closed, _ = v.(bool)
				}
				if v, ok := schema.Extensions["x-vocabulary"]; ok {
					name, _ = v.(string)
				}
			}
			if !hasClosed {
				errs = append(errs, "G3: enum is missing x-vocabulary-closed")
				return
			}
			if closed {
				return
			}
			if name == "" {
				errs = append(errs, "G3: open enum is missing x-vocabulary")
				return
			}
			if _, ok := vocab.Lookup(name); !ok {
				errs = append(errs, "G3: open enum x-vocabulary="+name+" has no /v2/vocabularies/"+name)
			}
		})
	}
	for _, item := range doc.Paths {
		for _, op := range pathOps(item) {
			for _, p := range op.Parameters {
				if p != nil {
					visit(p.Schema)
				}
			}
			for _, resp := range op.Responses {
				if resp == nil {
					continue
				}
				for _, content := range resp.Content {
					if content != nil {
						visit(content.Schema)
					}
				}
			}
		}
	}
	if doc.Components != nil && doc.Components.Schemas != nil {
		for _, schema := range doc.Components.Schemas.Map() {
			visit(schema)
		}
	}
	return errs
}

func checkG4(doc *huma.OpenAPI) []string {
	var errs []string
	for path, item := range doc.Paths {
		for _, op := range pathOps(item) {
			if len(op.Responses) == 0 {
				errs = append(errs, fmt.Sprintf("G4: %s %s has no responses", opMethod(op), path))
				continue
			}
			if _, ok := op.Responses["default"]; ok {
				nonDefault := 0
				for status := range op.Responses {
					if status != "default" && status != "200" && status != "201" && status != "204" {
						nonDefault++
					}
				}
				if nonDefault == 0 {
					errs = append(errs, fmt.Sprintf("G4: %s %s only declares {2xx, default}", opMethod(op), path))
				}
			}
			if strings.Contains(path, "{") {
				if _, ok := op.Responses["404"]; !ok {
					errs = append(errs, fmt.Sprintf("G4: %s %s has a path parameter but does not declare 404", opMethod(op), path))
				}
			}
			for status, resp := range op.Responses {
				if status == "200" || status == "201" || status == "204" || status == "default" {
					continue
				}
				if resp == nil || resp.Content["application/problem+json"] == nil {
					errs = append(errs, fmt.Sprintf("G4: %s %s response %s is not application/problem+json", opMethod(op), path, status))
				}
			}
			for _, p := range op.Parameters {
				if p == nil {
					continue
				}
				if p.In == "path" && !p.Required {
					errs = append(errs, fmt.Sprintf("G4: %s %s path parameter %s is not required", opMethod(op), path, p.Name))
				}
			}
		}
	}
	return errs
}

func checkG5(doc *huma.OpenAPI) []string {
	var errs []string
	seen := map[*huma.Schema]bool{}
	checkSchema := func(s *huma.Schema) {
		walkSchema(s, seen, func(schema *huma.Schema) {
			if schema == nil || schema.Properties == nil {
				return
			}
			code, ok := schema.Properties["code"]
			if !ok || code == nil {
				return
			}
			resolved := code
			if code.Ref != "" && doc.Components != nil && doc.Components.Schemas != nil {
				if r := doc.Components.Schemas.SchemaFromRef(code.Ref); r != nil {
					resolved = r
				}
			}
			if resolved.Type == "integer" || resolved.Type == "number" {
				errs = append(errs, "G5: error schema property code is numeric")
			}
		})
	}
	for _, item := range doc.Paths {
		for _, op := range pathOps(item) {
			for status, resp := range op.Responses {
				if status == "200" || status == "201" || status == "204" || resp == nil {
					continue
				}
				for _, content := range resp.Content {
					if content != nil {
						checkSchema(deref(doc, content.Schema))
					}
				}
			}
		}
	}
	for _, d := range problem.Codes {
		if problem.CodeFromKebab(problem.Kebab(d.Code)) != d.Code {
			errs = append(errs, "G5: code "+d.Code+" is not reversible with its type URI suffix")
		}
		if !strings.HasPrefix(d.TypeURI(), problem.TypeURIPrefix) {
			errs = append(errs, "G5: "+d.Code+" type URI is not under developer.nextmoe.dev/problems")
		}
	}
	return errs
}

func deref(doc *huma.OpenAPI, s *huma.Schema) *huma.Schema {
	if s == nil || s.Ref == "" || doc.Components == nil || doc.Components.Schemas == nil {
		return s
	}
	if r := doc.Components.Schemas.SchemaFromRef(s.Ref); r != nil {
		return r
	}
	return s
}

func walkSchema(s *huma.Schema, seen map[*huma.Schema]bool, fn func(*huma.Schema)) {
	if s == nil || seen[s] {
		return
	}
	seen[s] = true
	fn(s)
	for _, p := range s.Properties {
		walkSchema(p, seen, fn)
	}
	walkSchema(s.Items, seen, fn)
	for _, x := range s.OneOf {
		walkSchema(x, seen, fn)
	}
	for _, x := range s.AnyOf {
		walkSchema(x, seen, fn)
	}
	for _, x := range s.AllOf {
		walkSchema(x, seen, fn)
	}
	walkSchema(s.Not, seen, fn)
}

func opMethod(op *huma.Operation) string {
	if op.Method != "" {
		return op.Method
	}
	return "?"
}
