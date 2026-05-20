package schema

import "testing"

func TestNormalizeTypeForCompare(t *testing.T) {
	cases := []struct{ in, want string }{
		{"int", "integer"},
		{"int4", "integer"},
		{" INTEGER ", "integer"},
		{"pg_catalog.int4", "integer"},
		{"bool", "boolean"},
		{"float8", "double precision"},
	}
	for _, tc := range cases {
		if g := NormalizeTypeForCompare(tc.in); g != tc.want {
			t.Errorf("NormalizeTypeForCompare(%q) = %q, want %q", tc.in, g, tc.want)
		}
	}
}

func TestBuildFunctionIdentity(t *testing.T) {
	if g := BuildFunctionIdentity("public", "f", "int, text"); g != "public.f(integer, text)" {
		t.Errorf("got %q", g)
	}
}

func TestFunctionDependencyKey(t *testing.T) {
	if g := FunctionDependencyKey("public.f(integer)"); g != "public.f" {
		t.Errorf("got %q", g)
	}
	if g := FunctionDependencyKey("f"); g != "f" {
		t.Errorf("got %q", g)
	}
}
