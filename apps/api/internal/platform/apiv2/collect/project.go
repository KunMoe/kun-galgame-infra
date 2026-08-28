package collect

import (
	"encoding/json"
	"strings"
)

// ApplyFields trims a rendered v2 body to the requested top-level keys. It
// takes bytes because the only caller is the one post-response hook that also
// computes the ETag: projecting anywhere else would be one implementation per
// face, which is the drift this whole parameter's absence came from.
//
// Unknown tokens are already 400 UNKNOWN_FIELD at parse time, so this does not
// validate; it only projects. object and id are never dropped — a projected
// item that cannot say what it is is not a smaller representation, it is an
// unusable one.
func ApplyFields(body []byte, fields []string) ([]byte, error) {
	if len(fields) == 0 || len(body) == 0 {
		return body, nil
	}
	keep := map[string]bool{"object": true, "id": true}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			keep[f] = true
		}
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	trim(root, keep)
	return json.Marshal(root)
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
