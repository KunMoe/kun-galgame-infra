package service

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"strings"
)

// PublicFields is a parsed ?fields= selection. The zero value is INACTIVE and
// every Wants call answers true, so a face that never learned about fields=
// keeps loading and rendering everything.
type PublicFields struct {
	active bool
	keys   map[string]struct{}
}

func ParsePublicFields(raw string) PublicFields {
	if strings.TrimSpace(raw) == "" {
		return PublicFields{}
	}
	keys := map[string]struct{}{"id": {}}
	for tok := range strings.SplitSeq(raw, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			keys[tok] = struct{}{}
		}
	}
	return PublicFields{active: true, keys: keys}
}

func (f PublicFields) Active() bool { return f.active }

// Wants reports whether any of these response keys can still reach the wire.
// It is the load gate as well as the render gate: a block whose every key is
// unselected is never queried.
func (f PublicFields) Wants(keys ...string) bool {
	if !f.active {
		return true
	}
	for _, k := range keys {
		if _, ok := f.keys[k]; ok {
			return true
		}
	}
	return false
}

var errNotJSONObject = stderrors.New("catalog: fields projection expects a JSON object")

type jsonMember struct {
	key string
	val json.RawMessage
}

// ProjectObject keeps the selected top-level keys of one marshalled object,
// in their original order and with their values byte-for-byte unchanged.
func (f PublicFields) ProjectObject(raw []byte) ([]byte, error) {
	if !f.active {
		return raw, nil
	}
	members, err := decodeJSONObject(raw)
	if err != nil {
		return nil, err
	}
	kept := make([]jsonMember, 0, len(f.keys))
	for _, m := range members {
		if _, ok := f.keys[m.key]; ok {
			kept = append(kept, m)
		}
	}
	return encodeJSONObject(kept), nil
}

// ProjectItems applies ProjectObject to every element of the envelope's items
// array; the envelope's own keys (items/total/next_cursor/...) are never
// touched, because fields= names the keys of an ITEM.
func (f PublicFields) ProjectItems(raw []byte) ([]byte, error) {
	if !f.active {
		return raw, nil
	}
	members, err := decodeJSONObject(raw)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].key != "items" || string(members[i].val) == "null" {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(members[i].val, &items); err != nil {
			return nil, err
		}
		for j := range items {
			trimmed, perr := f.ProjectObject(items[j])
			if perr != nil {
				return nil, perr
			}
			items[j] = trimmed
		}
		members[i].val = encodeJSONArray(items)
	}
	return encodeJSONObject(members), nil
}

func decodeJSONObject(raw []byte) ([]jsonMember, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotJSONObject
	}
	var out []jsonMember
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, errNotJSONObject
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nil, err
		}
		out = append(out, jsonMember{key: key, val: val})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeJSONObject(members []jsonMember) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(m.key)
		b.Write(key)
		b.WriteByte(':')
		b.Write(m.val)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func encodeJSONArray(items []json.RawMessage) []byte {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(it)
	}
	b.WriteByte(']')
	return b.Bytes()
}
