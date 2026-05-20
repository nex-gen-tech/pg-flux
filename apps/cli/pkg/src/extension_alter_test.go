package src

import "testing"

func TestExtensionUpdateToVersion(t *testing.T) {
	if g, w := extensionUpdateToVersion(`ALTER EXTENSION myext UPDATE TO '2.0'`), "2.0"; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
}
