package collect

import (
	"encoding/json"
)

func ApplyFields(v any, fields []string) (any, error) {
	if len(fields) == 0 {
		return v, nil
	}
	keep := map[string]bool{}
	for _, f := range fields {
		keep[f] = true
	}
	keep["object"] = true
	keep["id"] = true
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	trim(root, keep)
	return root, nil
}

func trim(v any, keep map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		if obj, _ := t["object"].(string); obj == "list" {
			if items, ok := t["items"].([]any); ok {
				for _, it := range items {
					trim(it, keep)
				}
			}
			return
		}
		for k := range t {
			if !keep[k] {
				delete(t, k)
			}
		}
	case []any:
		for _, it := range t {
			trim(it, keep)
		}
	}
}
