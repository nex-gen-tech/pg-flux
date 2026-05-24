package codegen

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// ---------------------------------------------------------------------------
// Type mapping tests
// ---------------------------------------------------------------------------

func TestPythonTypeMap_Primitives(t *testing.T) {
	m := &PythonTypeMap{}
	cases := []struct {
		pg       string
		nullable bool
		wantType string
		wantImp  string // single expected import symbol, or ""
	}{
		// text family
		{"text", false, "str", ""},
		{"varchar", false, "str", ""},
		{"char", false, "str", ""},
		{"bpchar", false, "str", ""},
		{"name", false, "str", ""},
		{"tsvector", false, "str", ""},
		// integer family
		{"bigint", false, "int", ""},
		{"int8", false, "int", ""},
		{"int4", false, "int", ""},
		{"int2", false, "int", ""},
		{"smallint", false, "int", ""},
		{"integer", false, "int", ""},
		{"smallserial", false, "int", ""},
		{"serial", false, "int", ""},
		{"bigserial", false, "int", ""},
		// float
		{"float4", false, "float", ""},
		{"float8", false, "float", ""},
		{"real", false, "float", ""},
		{"double precision", false, "float", ""},
		// numeric
		{"numeric", false, "Decimal", "Decimal"},
		{"decimal", false, "Decimal", "Decimal"},
		// bool
		{"boolean", false, "bool", ""},
		{"bool", false, "bool", ""},
		// uuid
		{"uuid", false, "UUID", "UUID"},
		// timestamps
		{"timestamptz", false, "datetime", "datetime"},
		{"timestamp with time zone", false, "datetime", "datetime"},
		{"timestamp", false, "datetime", "datetime"},
		{"timestamp without time zone", false, "datetime", "datetime"},
		{"date", false, "datetime", "datetime"},
		// json
		{"jsonb", false, "dict[str, Any]", "Any"},
		{"json", false, "dict[str, Any]", "Any"},
		// bytea
		{"bytea", false, "bytes", ""},
		// unknown
		{"point", false, "Any", "Any"},
	}
	for _, tc := range cases {
		t.Run(tc.pg+"_nullable=false", func(t *testing.T) {
			got, imps := m.Map(tc.pg, tc.nullable)
			if got != tc.wantType {
				t.Errorf("Map(%q, false) type = %q, want %q", tc.pg, got, tc.wantType)
			}
			if tc.wantImp != "" {
				found := false
				for _, imp := range imps {
					if imp == tc.wantImp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Map(%q, false) imports = %v, want %q to be present", tc.pg, imps, tc.wantImp)
				}
			}
		})
	}
}

func TestPythonTypeMap_Nullable(t *testing.T) {
	m := &PythonTypeMap{}
	cases := []struct {
		pg       string
		wantType string
	}{
		{"text", "Optional[str]"},
		{"bigint", "Optional[int]"},
		{"uuid", "Optional[UUID]"},
		{"timestamptz", "Optional[datetime]"},
		{"jsonb", "Optional[dict[str, Any]]"},
	}
	for _, tc := range cases {
		got, imps := m.Map(tc.pg, true)
		if got != tc.wantType {
			t.Errorf("Map(%q, true) = %q, want %q", tc.pg, got, tc.wantType)
		}
		hasOptional := false
		for _, imp := range imps {
			if imp == "Optional" {
				hasOptional = true
				break
			}
		}
		if !hasOptional {
			t.Errorf("Map(%q, true) imports should contain Optional, got %v", tc.pg, imps)
		}
	}
}

func TestPythonTypeMap_Arrays(t *testing.T) {
	m := &PythonTypeMap{}
	cases := []struct {
		pg       string
		wantType string
	}{
		{"text[]", "list[str]"},
		{"bigint[]", "list[int]"},
		{"uuid[]", "list[UUID]"},
		{"jsonb[]", "list[dict[str, Any]]"},
	}
	for _, tc := range cases {
		got, _ := m.Map(tc.pg, false)
		if got != tc.wantType {
			t.Errorf("Map(%q, false) = %q, want %q", tc.pg, got, tc.wantType)
		}
	}
}

func TestPythonTypeMap_CustomEnum(t *testing.T) {
	m := &PythonTypeMap{
		CustomTypes: map[string]string{
			"public.user_role": "UserRole",
			"user_role":        "UserRole",
		},
	}
	got, _ := m.Map("public.user_role", false)
	if got != "UserRole" {
		t.Errorf("Map(public.user_role) = %q, want UserRole", got)
	}
	got, _ = m.Map("user_role", false)
	if got != "UserRole" {
		t.Errorf("Map(user_role) = %q, want UserRole", got)
	}
	// nullable enum
	got, imps := m.Map("public.user_role", true)
	if got != "Optional[UserRole]" {
		t.Errorf("Map(public.user_role, true) = %q, want Optional[UserRole]", got)
	}
	hasOptional := false
	for _, imp := range imps {
		if imp == "Optional" {
			hasOptional = true
		}
	}
	if !hasOptional {
		t.Errorf("nullable enum should import Optional, got %v", imps)
	}
}

// ---------------------------------------------------------------------------
// CamelCase / PascalCase conversion for Python class names
// ---------------------------------------------------------------------------

func TestPythonTypeName(t *testing.T) {
	g := NewPythonGenerator()
	cases := []struct {
		sch, name, want string
	}{
		// Singularized table/view names
		{"public", "users", "User"},
		{"public", "todo_tags", "TodoTag"},
		{"public", "categories", "Category"},
		{"myapp", "user_profile", "UserProfile"},
		// Exceptions: words ending in 's' that are already singular
		{"public", "todo_priority", "TodoPriority"},
	}
	for _, tc := range cases {
		got := g.typeName(tc.sch, tc.name)
		if got != tc.want {
			t.Errorf("typeName(%q, %q) = %q, want %q", tc.sch, tc.name, got, tc.want)
		}
	}
}

func TestPythonEnumTypeNameNotSingularized(t *testing.T) {
	g := NewPythonGenerator()
	cases := []struct {
		sch, name, want string
	}{
		// Enum names should NOT be singularized
		{"public", "user_roles", "UserRoles"},
		{"public", "todo_priorities", "TodoPriorities"},
		{"public", "status", "Status"},
	}
	for _, tc := range cases {
		got := g.enumTypeName(tc.sch, tc.name)
		if got != tc.want {
			t.Errorf("enumTypeName(%q, %q) = %q, want %q", tc.sch, tc.name, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Full round-trip: SchemaState → models.py string
// ---------------------------------------------------------------------------

func TestPythonGenerateRoundTrip(t *testing.T) {
	s := &schema.SchemaState{
		EnumValues: map[string][]string{
			"public.status": {"active", "inactive", "pending"},
		},
		Tables: map[string]*schema.Table{
			"public.items": {
				Schema: "public",
				Name:   "items",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigserial", NotNull: true, IsPrimaryKey: true, Identity: "by-default"},
					{Name: "name", TypeSQL: "text", NotNull: true},
					{Name: "description", TypeSQL: "text"}, // nullable
					{Name: "status", TypeSQL: "public.status", NotNull: true},
					{Name: "score", TypeSQL: "numeric", NotNull: true, DefaultSQL: "0"},
					{Name: "name_lower", TypeSQL: "text", GeneratedKind: "stored", GeneratedExpr: "lower(name)"},
					{Name: "data", TypeSQL: "jsonb"},
				},
			},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	content, ok := fs["models.py"]
	if !ok {
		t.Fatal("expected models.py in output")
	}
	py := string(content)

	// Header
	assertContains(t, py, "# Code generated by pg-flux. DO NOT EDIT.")
	assertContains(t, py, "from __future__ import annotations")

	// Imports
	assertContains(t, py, "from decimal import Decimal")
	assertContains(t, py, "from enum import Enum")
	assertContains(t, py, "from typing import")
	assertContains(t, py, "Optional")
	assertContains(t, py, "Any")
	assertContains(t, py, "from pydantic import BaseModel")

	// Enum class
	assertContains(t, py, "class Status(str, Enum):")
	assertContains(t, py, `ACTIVE = "active"`)
	assertContains(t, py, `INACTIVE = "inactive"`)
	assertContains(t, py, `PENDING = "pending"`)

	// Table class — singularized: "items" → "Item"
	assertContains(t, py, "class Item(BaseModel):")
	// ORM config on base class
	assertContains(t, py, "model_config = ConfigDict(from_attributes=True)")
	// ConfigDict import
	assertContains(t, py, "from pydantic import BaseModel, ConfigDict")
	// id: identity column → has default → Optional with = None
	assertContains(t, py, "id: Optional[int] = None")
	// name: NOT NULL, no default → plain str
	assertContains(t, py, "name: str")
	// description: nullable → Optional with = None
	assertContains(t, py, "description: Optional[str] = None")
	// status: enum type
	assertContains(t, py, "status: Status")
	// score: NOT NULL with DEFAULT → Optional + = None
	assertContains(t, py, "score: Optional[Decimal] = None")
	// name_lower: generated → Optional + = None + # server-computed
	assertContains(t, py, "name_lower: Optional[str] = None  # server-computed")
	// data: nullable jsonb
	assertContains(t, py, "data: Optional[dict[str, Any]] = None")

	// ItemCreate — server-managed columns excluded (id=identity, name_lower=generated)
	assertContains(t, py, "class ItemCreate(BaseModel):")
	// name is writable and required
	assertContains(t, py, "class ItemCreate(BaseModel):")

	// ItemUpdate — all writable fields as Optional
	assertContains(t, py, "class ItemUpdate(BaseModel):")
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\n\nFull output:\n%s", needle, haystack)
	}
}

// ---------------------------------------------------------------------------
// Enum value uppercasing edge cases
// ---------------------------------------------------------------------------

func TestPythonEnumValueNames(t *testing.T) {
	s := &schema.SchemaState{
		EnumValues: map[string][]string{
			"public.my_enum": {"foo-bar", "hello_world", "ALREADY_UPPER", "simple"},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	assertContains(t, py, `FOO_BAR = "foo-bar"`)
	assertContains(t, py, `HELLO_WORLD = "hello_world"`)
	assertContains(t, py, `ALREADY_UPPER = "ALREADY_UPPER"`)
	assertContains(t, py, `SIMPLE = "simple"`)
}

// ---------------------------------------------------------------------------
// Nil / empty state guard
// ---------------------------------------------------------------------------

func TestPythonGenerateNilState(t *testing.T) {
	g := NewPythonGenerator()
	fs, err := g.Generate(nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("expected empty FileSet for nil state, got %v", fs)
	}
}
