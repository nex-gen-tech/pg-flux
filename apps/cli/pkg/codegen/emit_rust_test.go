package codegen

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// ---------------------------------------------------------------------------
// RustTypeMap unit tests
// ---------------------------------------------------------------------------

func TestRustTypeMap_Primitives(t *testing.T) {
	m := &RustTypeMap{}
	cases := []struct {
		pg       string
		nullable bool
		want     string
	}{
		// integers
		{"smallint", false, "i16"},
		{"int2", false, "i16"},
		{"integer", false, "i32"},
		{"int", false, "i32"},
		{"int4", false, "i32"},
		{"bigint", false, "i64"},
		{"int8", false, "i64"},
		{"smallserial", false, "i16"},
		{"serial", false, "i32"},
		{"bigserial", false, "i64"},
		// floats
		{"real", false, "f32"},
		{"float4", false, "f32"},
		{"double precision", false, "f64"},
		{"float8", false, "f64"},
		// bool
		{"boolean", false, "bool"},
		{"bool", false, "bool"},
		// text
		{"text", false, "String"},
		{"varchar", false, "String"},
		{"character varying", false, "String"},
		{"numeric", false, "String"},
		{"decimal", false, "String"},
		{"inet", false, "String"},
		// bytea
		{"bytea", false, "Vec<u8>"},
		// uuid
		{"uuid", false, "uuid::Uuid"},
		// json
		{"json", false, "serde_json::Value"},
		{"jsonb", false, "serde_json::Value"},
		// timestamps
		{"timestamptz", false, "chrono::DateTime<chrono::Utc>"},
		{"timestamp with time zone", false, "chrono::DateTime<chrono::Utc>"},
		{"timestamp", false, "chrono::NaiveDateTime"},
		{"timestamp without time zone", false, "chrono::NaiveDateTime"},
		{"date", false, "chrono::NaiveDate"},
		{"time", false, "chrono::NaiveTime"},
		{"time without time zone", false, "chrono::NaiveTime"},
		{"timetz", false, "chrono::DateTime<chrono::Utc>"},
	}
	for _, tc := range cases {
		got, _ := m.Map(tc.pg, tc.nullable)
		if got != tc.want {
			t.Errorf("Map(%q, false) = %q, want %q", tc.pg, got, tc.want)
		}
	}
}

func TestRustTypeMap_Nullable(t *testing.T) {
	m := &RustTypeMap{}
	cases := []struct {
		pg   string
		want string
	}{
		{"text", "Option<String>"},
		{"bigint", "Option<i64>"},
		{"uuid", "Option<uuid::Uuid>"},
		{"timestamptz", "Option<chrono::DateTime<chrono::Utc>>"},
		{"jsonb", "Option<serde_json::Value>"},
		{"bool", "Option<bool>"},
	}
	for _, tc := range cases {
		got, _ := m.Map(tc.pg, true)
		if got != tc.want {
			t.Errorf("Map(%q, true) = %q, want %q", tc.pg, got, tc.want)
		}
	}
}

func TestRustTypeMap_Arrays(t *testing.T) {
	m := &RustTypeMap{}
	cases := []struct {
		pg   string
		want string
	}{
		{"text[]", "Vec<String>"},
		{"bigint[]", "Vec<i64>"},
		{"uuid[]", "Vec<uuid::Uuid>"},
		{"jsonb[]", "Vec<serde_json::Value>"},
		{"boolean[]", "Vec<bool>"},
	}
	for _, tc := range cases {
		got, _ := m.Map(tc.pg, false)
		if got != tc.want {
			t.Errorf("Map(%q, false) = %q, want %q", tc.pg, got, tc.want)
		}
	}
}

func TestRustTypeMap_CustomEnum(t *testing.T) {
	m := &RustTypeMap{
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
	got, _ = m.Map("public.user_role", true)
	if got != "Option<UserRole>" {
		t.Errorf("Map(public.user_role, true) = %q, want Option<UserRole>", got)
	}
}

func TestRustTypeMap_Overrides(t *testing.T) {
	m := &RustTypeMap{
		Overrides: map[string]string{
			"numeric": "rust_decimal::Decimal",
		},
	}
	got, _ := m.Map("numeric", false)
	if got != "rust_decimal::Decimal" {
		t.Errorf("Map(numeric) with override = %q, want rust_decimal::Decimal", got)
	}
}

// ---------------------------------------------------------------------------
// RustGenerator full round-trip
// ---------------------------------------------------------------------------

func rustTestState() *schema.SchemaState {
	return &schema.SchemaState{
		EnumValues: map[string][]string{
			"public.user_role": {"admin", "member", "viewer"},
		},
		Tables: map[string]*schema.Table{
			"public.users": {
				Schema: "public",
				Name:   "users",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigserial", NotNull: true, IsPrimaryKey: true},
					{Name: "email", TypeSQL: "text", NotNull: true},
					{Name: "role", TypeSQL: "public.user_role", NotNull: false},
					{Name: "created_at", TypeSQL: "timestamptz", NotNull: false},
					{Name: "meta", TypeSQL: "jsonb"},
					{Name: "profile_id", TypeSQL: "uuid"},
				},
			},
		},
		Views: map[string]*schema.View{
			"public.active_users": {
				Schema: "public",
				Name:   "active_users",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigint"},
					{Name: "email", TypeSQL: "text"},
				},
			},
		},
		CompositeTypes: map[string]*schema.CompositeType{
			"public.address": {
				Schema: "public",
				Name:   "address",
				Attributes: []schema.CompositeAttribute{
					{Name: "street", Type: "text"},
					{Name: "city", Type: "text"},
					{Name: "zip", Type: "text"},
				},
			},
		},
		Domains: map[string]*schema.Domain{
			"public.email_address": {
				Schema:   "public",
				Name:     "email_address",
				BaseType: "text",
			},
		},
	}
}

func TestRustGenerateRoundTrip(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// All expected files present.
	for _, f := range []string{"tables.rs", "enums.rs", "views.rs", "types.rs", "mod.rs"} {
		if _, ok := fs[f]; !ok {
			t.Errorf("expected %s in output, got files: %v", f, fileSetKeys(fs))
		}
	}
}

func TestRustTables(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	tables := string(fs["tables.rs"])

	assertContains(t, tables, "// Code generated by pg-flux. DO NOT EDIT.")
	assertContains(t, tables, "use serde::{Deserialize, Serialize};")
	assertContains(t, tables, "#[derive(Debug, Clone, sqlx::FromRow, Serialize, Deserialize)]")
	assertContains(t, tables, "pub struct User {")
	assertContains(t, tables, "pub id: i64,")
	assertContains(t, tables, "pub email: String,")
	// Nullable role → Option<UserRole>
	assertContains(t, tables, "pub role: Option<UserRole>,")
	// Nullable timestamptz
	assertContains(t, tables, "pub created_at: Option<chrono::DateTime<chrono::Utc>>,")
	// Nullable jsonb
	assertContains(t, tables, "pub meta: Option<serde_json::Value>,")
	// Nullable uuid
	assertContains(t, tables, "pub profile_id: Option<uuid::Uuid>,")
}

func TestRustEnums(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	enums := string(fs["enums.rs"])

	assertContains(t, enums, "#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize, sqlx::Type)]")
	assertContains(t, enums, `#[sqlx(type_name = "user_role")]`)
	assertContains(t, enums, "pub enum UserRole {")
	assertContains(t, enums, `#[sqlx(rename = "admin")]`)
	assertContains(t, enums, `#[serde(rename = "admin")]`)
	assertContains(t, enums, "    Admin,")
	assertContains(t, enums, "    Member,")
	assertContains(t, enums, "    Viewer,")
}

func TestRustViews(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	views := string(fs["views.rs"])

	assertContains(t, views, "#[derive(Debug, Clone, sqlx::FromRow, Serialize, Deserialize)]")
	assertContains(t, views, "pub struct ActiveUser {")
	// View columns are nullable
	assertContains(t, views, "pub id: Option<i64>,")
	assertContains(t, views, "pub email: Option<String>,")
}

func TestRustTypes(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	types := string(fs["types.rs"])

	// Composite type
	assertContains(t, types, "#[derive(Debug, Clone, Serialize, Deserialize)]")
	assertContains(t, types, "pub struct Address {")
	assertContains(t, types, "pub street: String,")
	assertContains(t, types, "pub city: String,")
	assertContains(t, types, "pub zip: String,")

	// Domain → newtype
	assertContains(t, types, "pub struct EmailAddress(pub String);")
}

func TestRustModRs(t *testing.T) {
	g := NewRustGenerator()
	s := rustTestState()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	mod := string(fs["mod.rs"])

	assertContains(t, mod, "pub mod tables;")
	assertContains(t, mod, "pub mod enums;")
	assertContains(t, mod, "pub mod views;")
	assertContains(t, mod, "pub mod types;")
}

func TestRustFunctions(t *testing.T) {
	s := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.get_user": {
				Schema:     "public",
				Name:       "get_user",
				Kind:       "f",
				ReturnType: "record",
				Args: []schema.FunctionArg{
					{Name: "user_id", Type: "bigint"},
				},
				ReturnsTable: []schema.FunctionArg{
					{Name: "id", Type: "bigint"},
					{Name: "email", Type: "text"},
				},
			},
			"public.count_users": {
				Schema:     "public",
				Name:       "count_users",
				Kind:       "f",
				ReturnType: "bigint",
			},
		},
	}
	g := NewRustGenerator()
	fs, err := g.Generate(s, Options{Emit: EmitOptions{Functions: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fs["functions.rs"]; !ok {
		t.Fatal("expected functions.rs in output")
	}
	fns := string(fs["functions.rs"])

	assertContains(t, fns, "pub struct GetUserParams {")
	assertContains(t, fns, "pub user_id: i64,")
	assertContains(t, fns, "pub struct GetUserResult {")
	assertContains(t, fns, "pub id: i64,")
	assertContains(t, fns, "pub email: String,")
	assertContains(t, fns, "pub type CountUsersRow = i64;")
}

func TestRustNilState(t *testing.T) {
	g := NewRustGenerator()
	fs, err := g.Generate(nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("expected empty FileSet for nil state, got %v", fileSetKeys(fs))
	}
}

func TestRustEnumHyphenatedValues(t *testing.T) {
	s := &schema.SchemaState{
		EnumValues: map[string][]string{
			"public.status": {"in-progress", "not-started", "done"},
		},
	}
	g := NewRustGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	enums := string(fs["enums.rs"])

	// Hyphens become underscores in variant names; original value in rename attr.
	assertContains(t, enums, `#[sqlx(rename = "in-progress")]`)
	assertContains(t, enums, "    InProgress,")
	assertContains(t, enums, `#[sqlx(rename = "not-started")]`)
	assertContains(t, enums, "    NotStarted,")
}

// fileSetKeys returns sorted keys for a FileSet (for readable error messages).
func fileSetKeys(fs FileSet) []string {
	keys := make([]string, 0, len(fs))
	for k := range fs {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Python parity: views, composite types, domains, functions
// ---------------------------------------------------------------------------

func TestPythonViews(t *testing.T) {
	s := &schema.SchemaState{
		Views: map[string]*schema.View{
			"public.active_users": {
				Schema: "public",
				Name:   "active_users",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigint"},
					{Name: "email", TypeSQL: "text"},
				},
			},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	assertContains(t, py, "from pydantic import BaseModel")
	assertContains(t, py, "class ActiveUser(BaseModel):")
	// View columns are nullable
	assertContains(t, py, "id: Optional[int] = None")
	assertContains(t, py, "email: Optional[str] = None")
}

func TestPythonCompositeTypes(t *testing.T) {
	s := &schema.SchemaState{
		CompositeTypes: map[string]*schema.CompositeType{
			"public.address": {
				Schema: "public",
				Name:   "address",
				Attributes: []schema.CompositeAttribute{
					{Name: "street", Type: "text"},
					{Name: "zip", Type: "text"},
				},
			},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	assertContains(t, py, "class Address(BaseModel):")
	assertContains(t, py, "street: str")
	assertContains(t, py, "zip: str")
}

func TestPythonDomains(t *testing.T) {
	s := &schema.SchemaState{
		Domains: map[string]*schema.Domain{
			"public.email_address": {
				Schema:   "public",
				Name:     "email_address",
				BaseType: "text",
			},
			"public.user_id": {
				Schema:   "public",
				Name:     "user_id",
				BaseType: "bigint",
			},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	assertContains(t, py, "from typing import")
	assertContains(t, py, "NewType")
	assertContains(t, py, `EmailAddress = NewType("EmailAddress", str)`)
	assertContains(t, py, `UserID = NewType("UserID", int)`)
}

func TestPythonFunctions(t *testing.T) {
	s := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.get_item": {
				Schema:     "public",
				Name:       "get_item",
				Kind:       "f",
				ReturnType: "record",
				Args:       []schema.FunctionArg{{Name: "item_id", Type: "bigint"}},
				ReturnsTable: []schema.FunctionArg{
					{Name: "id", Type: "bigint"},
					{Name: "name", Type: "text"},
				},
			},
			"public.item_count": {
				Schema:     "public",
				Name:       "item_count",
				Kind:       "f",
				ReturnType: "bigint",
			},
		},
	}
	g := NewPythonGenerator()
	fs, err := g.Generate(s, Options{Emit: EmitOptions{Functions: true}})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	assertContains(t, py, "TypedDict")
	assertContains(t, py, "class GetItemParams(TypedDict, total=False):")
	assertContains(t, py, "item_id: int")
	assertContains(t, py, "class GetItemResult(TypedDict):")
	assertContains(t, py, "id: int")
	assertContains(t, py, "name: str")
	assertContains(t, py, "ItemCountRow = int")
}

func TestPythonFunctionsOffByDefault(t *testing.T) {
	s := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.helper": {
				Schema:     "public",
				Name:       "helper",
				Kind:       "f",
				ReturnType: "text",
			},
		},
	}
	g := NewPythonGenerator()
	// Functions=false (default)
	fs, err := g.Generate(s, Options{})
	if err != nil {
		t.Fatal(err)
	}
	py := string(fs["models.py"])

	if strings.Contains(py, "HelperRow") {
		t.Error("functions should not be emitted when Functions=false")
	}
}
