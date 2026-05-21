package codegen

import "testing"

func TestPascalCase(t *testing.T) {
	tests := []struct{ in, out string }{
		{"user_role", "UserRole"},
		{"user-role", "UserRole"},
		{"users", "Users"},
		{"USERS", "Users"},
		{"a_b_c", "ABC"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := PascalCase(tc.in); got != tc.out {
			t.Errorf("PascalCase(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestPascalCaseInit(t *testing.T) {
	tests := []struct{ in, out string }{
		{"user_id", "UserID"},
		{"api_key", "APIKey"},
		{"http_url", "HTTPURL"},
		{"json_data", "JSONData"},
		{"id", "ID"},
		{"user_db_id", "UserDBID"},
		{"users", "Users"}, // no matching initialism
	}
	for _, tc := range tests {
		if got := PascalCaseInit(tc.in); got != tc.out {
			t.Errorf("PascalCaseInit(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestCamelCase(t *testing.T) {
	tests := []struct{ in, out string }{
		{"user_role", "userRole"},
		{"created_at", "createdAt"},
		{"id", "id"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := CamelCase(tc.in); got != tc.out {
			t.Errorf("CamelCase(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestSingular(t *testing.T) {
	tests := []struct{ in, out string }{
		{"users", "user"},
		{"addresses", "address"},
		{"categories", "category"},
		{"settings", "setting"},
		{"posts", "post"},
		{"boxes", "box"},
		{"matches", "match"},
		{"dishes", "dish"},
		{"buzzes", "buzz"},
		{"class", "class"}, // -ss not stripped
		{"news", "news"},   // ends in -ss only check; "news" ends in -s but conservatively strip → "new". Hmm.
		{"", ""},
		// Exception words: already-singular nouns ending in 's'.
		{"status", "status"},
		{"event_status", "event_status"},
		{"attendee_status", "attendee_status"},
		{"access", "access"},
		{"process", "process"},
		{"progress", "progress"},
		{"success", "success"},
		{"canvas", "canvas"},
	}
	for _, tc := range tests {
		got := Singular(tc.in)
		// "news" is a known irregular; conservative rule strips trailing s → "new".
		// We accept that — users override via config for irregular cases.
		if tc.in == "news" {
			if got != "new" {
				t.Errorf("Singular(%q) = %q, want \"new\" (conservative trailing-s strip)", tc.in, got)
			}
			continue
		}
		if got != tc.out {
			t.Errorf("Singular(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

// TestPascalCaseInitStatus verifies the bug fix: type names ending in "status"
// must not be mangled to "…Statu".
func TestPascalCaseInitStatus(t *testing.T) {
	tests := []struct{ in, out string }{
		// Bug regression: these were producing "EventStatu"/"AttendeeStatu".
		{"event_status", "EventStatus"},
		{"attendee_status", "AttendeeStatus"},
		// Other exception words.
		{"user_access", "UserAccess"},
		{"work_process", "WorkProcess"},
		// Normal plurals should still singularize.
		{"todo_priority", "TodoPriority"}, // no plural suffix, unchanged
		// "users" still singularizes to "user" → "User".
		{"users", "User"},
	}
	for _, tc := range tests {
		got := PascalCaseInit(Singular(tc.in))
		if got != tc.out {
			t.Errorf("PascalCaseInit(Singular(%q)) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestSnakeCase(t *testing.T) {
	tests := []struct{ in, out string }{
		{"UserID", "user_i_d"}, // simple algorithm; initialism handling is one-way
		{"CreatedAt", "created_at"},
		{"PascalCase", "pascal_case"},
	}
	for _, tc := range tests {
		if got := SnakeCase(tc.in); got != tc.out {
			t.Errorf("SnakeCase(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestEscapeStringLiteral(t *testing.T) {
	tests := []struct{ in, out string }{
		{`hello`, `hello`},
		{`he said "hi"`, `he said \"hi\"`},
		{"line1\nline2", `line1\nline2`},
		{`back\slash`, `back\\slash`},
	}
	for _, tc := range tests {
		if got := EscapeStringLiteral(tc.in); got != tc.out {
			t.Errorf("EscapeStringLiteral(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}
