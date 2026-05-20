package differ

import "testing"

// Catalog pg_get_constraintdef often omits the public. prefix on the referenced
// table; the parser may emit it. Fingerprinting must still match.
func TestTableConstraintFingerprint_PublicSchemaFK(t *testing.T) {
	typicalCatalog := "FOREIGN KEY (parent_id) REFERENCES a_parents (id)"
	fromParser := "FOREIGN KEY (parent_id) REFERENCES public.a_parents (id)"
	a := tableConstraintDefFingerprint("public", "b_children", "fk_b_children_parent", typicalCatalog)
	b := tableConstraintDefFingerprint("public", "b_children", "fk_b_children_parent", fromParser)
	if a != b {
		t.Fatalf("fingerprint: %q vs %q", a, b)
	}
}

// pg_get_viewdef may not qualify righthand relations with public; schema files often do.
func TestCreateStmtFingerprint_ViewPublicOmission(t *testing.T) {
	fromFile := `CREATE VIEW public.v_parents AS
    SELECT id, name FROM public.a_parents;`
	fromCat := `CREATE VIEW public.v_parents AS SELECT id, name FROM a_parents`
	a := createStmtDefFingerprint(fromFile)
	b := createStmtDefFingerprint(fromCat)
	if a != b {
		t.Fatalf("fingerprint: %q vs %q", a, b)
	}
}
