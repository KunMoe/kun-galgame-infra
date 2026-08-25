package handler

import (
	"encoding/json"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/editspec"
	"api/internal/platform/editing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func testSchemaRegistry(t *testing.T) *editing.Registry {
	t.Helper()
	reg := editing.NewRegistry()
	if err := editspec.RegisterAll(reg, nil); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestGetSchemaUnbound(t *testing.T) {
	_, err := (*Catalog)(nil).GetSchema(t.Context(), "work")
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
	_, err = (&Catalog{}).GetSchema(t.Context(), "person")
	p, ok = err.(*problem.Problem)
	if !ok || p.Code != problem.CodeNotFound {
		t.Fatalf("person %v", err)
	}
}

func TestGetSchemaWorkAndRelease(t *testing.T) {
	c := &Catalog{EditTypes: testSchemaRegistry(t)}
	s, err := c.GetSchema(t.Context(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if s.Object != "object_schema" || s.TargetObject != "work" || s.EntityType != editspec.TypeWork {
		t.Fatalf("%+v", s)
	}
	if s.CreationDisabled {
		t.Fatal("work creation is not disabled")
	}
	if len(s.Include) == 0 || len(s.FullSet) == 0 {
		t.Fatalf("include/full_set %+v %+v", s.Include, s.FullSet)
	}
	foundTitles, foundKind := false, false
	for _, f := range s.Fields {
		if f.Key == editspec.FieldWorkTitles {
			foundTitles = true
			if f.FieldType != string(editing.KindList) {
				t.Fatalf("titles field_type %s", f.FieldType)
			}
			if f.MaxElements != editing.DefaultMaxElements && f.MaxElements <= 0 {
				t.Fatalf("titles max_elements %d", f.MaxElements)
			}
		}
		if f.Key == "kind" || f.FieldType == "kind" {
			foundKind = true
		}
	}
	if !foundTitles {
		t.Fatal("missing catalog.work.titles")
	}
	if foundKind {
		t.Fatal("kind must not appear as a field key or field_type")
	}

	rel, err := c.GetSchema(t.Context(), "release")
	if err != nil {
		t.Fatal(err)
	}
	if !rel.CreationDisabled || rel.EntityType != editspec.TypeRelease {
		t.Fatalf("%+v", rel)
	}

	co, err := c.GetSchema(t.Context(), "company")
	if err != nil {
		t.Fatal(err)
	}
	if co.EntityType != editspec.TypeLabel || co.TargetObject != "company" {
		t.Fatalf("%+v", co)
	}
}

func TestCatalogSchemaHTTP(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	SetupWith(app, Options{Catalog: &Catalog{EditTypes: testSchemaRegistry(t)}})

	status, ct, body := do(t, app, httpMethodGet, "/v2/catalog/schemas/work")
	require.Equal(t, 200, status)
	require.Contains(t, ct, "json")
	var s repr.ObjectSchema
	require.NoError(t, json.Unmarshal(body, &s), string(body))
	require.Equal(t, "object_schema", s.Object)
	require.Equal(t, "work", s.TargetObject)
	require.Contains(t, s.Include, "titles")
	require.Contains(t, s.FullSet, "titles")
	require.NotContains(t, s.FullSet, "characters")
	require.False(t, s.CreationDisabled)
	require.NotEmpty(t, s.Fields)
	require.NotContains(t, string(body), `"kind"`)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	_, hasKind := raw["kind"]
	require.False(t, hasKind)
	_, hasPropose := raw["can_propose"]
	require.False(t, hasPropose)
	for _, item := range raw["fields"].([]any) {
		f := item.(map[string]any)
		if _, ok := f["kind"]; ok {
			t.Fatalf("field has kind: %v", f)
		}
		if _, ok := f["can_propose"]; ok {
			t.Fatalf("field has can_propose: %v", f)
		}
		if _, ok := f["field_type"]; !ok {
			t.Fatalf("field missing field_type: %v", f)
		}
	}

	status, _, body = do(t, app, httpMethodGet, "/v2/catalog/schemas/release")
	require.Equal(t, 200, status)
	require.NoError(t, json.Unmarshal(body, &s))
	require.True(t, s.CreationDisabled)

	status, _, body = do(t, app, httpMethodGet, "/v2/catalog/schemas/person")
	require.Equal(t, 404, status)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p), string(body))
	require.Equal(t, problem.CodeNotFound, p.Code)
}

const httpMethodGet = "GET"

func TestCatalogSchemaUnauthenticated(t *testing.T) {
	app := testApp(t)
	status, ct, body := do(t, app, httpMethodGet, "/v2/catalog/schemas/work")
	require.Equal(t, 503, status)
	require.Contains(t, ct, "application/problem+json")
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p), string(body))
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)
	require.NotEqual(t, problem.CodeMissingCredential, p.Code)

	status, _, body = do(t, app, httpMethodGet, "/v2/catalog/schemas/nope")
	require.Equal(t, 404, status)
	require.NoError(t, json.Unmarshal(body, &p), string(body))
	require.Equal(t, problem.CodeNotFound, p.Code)
}
