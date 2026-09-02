package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrUnknownKey   = errors.New("settings: unknown key")
	ErrInvalidValue = errors.New("settings: invalid value")
	ErrNoteTooLong  = errors.New("settings: note exceeds 512 characters")
)

type OverrideView struct {
	Value           any    `json:"value"`
	Version         int64  `json:"version"`
	UpdatedByUserID uint   `json:"updated_by_user_id"`
	UpdatedByName   string `json:"updated_by_name"`
	Note            string `json:"note"`
	UpdatedAt       string `json:"updated_at"`
}

type KeyView struct {
	Key       string        `json:"key"`
	Kind      Kind          `json:"kind"`
	DescEN    string        `json:"desc_en"`
	DescZH    string        `json:"desc_zh"`
	Default   any           `json:"default"`
	EnvVar    string        `json:"env_var,omitempty"`
	Enum      []string      `json:"enum,omitempty"`
	Min       *float64      `json:"min,omitempty"`
	Max       *float64      `json:"max,omitempty"`
	Pattern   string        `json:"pattern,omitempty"`
	Effective any           `json:"effective"`
	Source    Source        `json:"source"`
	Override  *OverrideView `json:"override"`
}

type DomainView struct {
	Name    string    `json:"name"`
	TitleZH string    `json:"title_zh"`
	Keys    []KeyView `json:"keys"`
}

type Overview struct {
	Domains  []DomainView `json:"domains"`
	Writable bool         `json:"writable"`
}

type Service struct {
	reg   *Registry
	store *Store
	dist  *Distributor
}

func NewService(reg *Registry, store *Store, dist *Distributor) *Service {
	return &Service{reg: reg, store: store, dist: dist}
}

func (s *Service) Overview(ctx context.Context, writable bool) (*Overview, error) {
	byKey, err := s.overrideByKey(ctx)
	if err != nil {
		return nil, err
	}
	out := &Overview{Writable: writable}
	for _, d := range s.reg.Domains() {
		view := DomainView{Name: d.Name, TitleZH: d.TitleZH, Keys: make([]KeyView, 0, len(d.Keys))}
		for _, e := range d.Keys {
			view.Keys = append(view.Keys, keyView(e, byKey[e.Meta().Name]))
		}
		out.Domains = append(out.Domains, view)
	}
	return out, nil
}

func (s *Service) Set(ctx context.Context, actorID uint, key string, raw json.RawMessage, note string, expectVersion *int64) (*KeyView, error) {
	e, ok := s.reg.Lookup(key)
	if !ok {
		return nil, ErrUnknownKey
	}
	if utf8.RuneCountInString(note) > 512 {
		return nil, ErrNoteTooLong
	}
	v, err := e.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}
	if err := e.Validate(v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}
	if _, err := s.store.Set(ctx, key, raw, note, expectVersion, actorID); err != nil {
		return nil, err
	}
	if err := s.dist.Refresh(ctx); err != nil {
		return nil, err
	}
	s.dist.Announce(ctx)
	return s.viewFor(ctx, e)
}

func (s *Service) Reset(ctx context.Context, actorID uint, key string, note string) (*KeyView, error) {
	e, ok := s.reg.Lookup(key)
	if !ok {
		return nil, ErrUnknownKey
	}
	if utf8.RuneCountInString(note) > 512 {
		return nil, ErrNoteTooLong
	}
	if err := s.store.Reset(ctx, key, note, actorID); err != nil {
		return nil, err
	}
	if err := s.dist.Refresh(ctx); err != nil {
		return nil, err
	}
	s.dist.Announce(ctx)
	return s.viewFor(ctx, e)
}

func (s *Service) Audit(ctx context.Context, limit int) ([]AuditEntry, error) {
	return s.store.RecentAudit(ctx, limit)
}

func (s *Service) overrideByKey(ctx context.Context) (map[string]*OverrideRow, error) {
	rows, err := s.store.Overrides(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]*OverrideRow, len(rows))
	for i := range rows {
		byKey[rows[i].Key] = &rows[i]
	}
	return byKey, nil
}

func (s *Service) viewFor(ctx context.Context, e Entry) (*KeyView, error) {
	byKey, err := s.overrideByKey(ctx)
	if err != nil {
		return nil, err
	}
	v := keyView(e, byKey[e.Meta().Name])
	return &v, nil
}

func keyView(e Entry, row *OverrideRow) KeyView {
	m := e.Meta()
	v := KeyView{
		Key:       m.Name,
		Kind:      m.Kind,
		DescEN:    m.DescEN,
		DescZH:    m.DescZH,
		Default:   e.Default(),
		EnvVar:    m.EnvVar,
		Enum:      m.Enum,
		Min:       m.Min,
		Max:       m.Max,
		Pattern:   m.Pattern,
		Effective: e.Default(),
		Source:    SourceDefault,
	}
	if row == nil {
		return v
	}
	ov := &OverrideView{
		Version:         row.Version,
		UpdatedByUserID: row.UpdatedByUserID,
		UpdatedByName:   row.UpdatedByName,
		Note:            row.Note,
		UpdatedAt:       row.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	decoded, err := e.Decode(json.RawMessage(row.Value))
	if err == nil {
		ov.Value = decoded
		if e.Validate(decoded) == nil {
			v.Effective = decoded
			v.Source = SourceDB
		}
	} else if json.Valid(row.Value) {
		var dumped any
		_ = json.Unmarshal(row.Value, &dumped)
		ov.Value = dumped
	}
	v.Override = ov
	return v
}
