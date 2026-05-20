package differ

import (
	"reflect"
	"testing"
)

// TestSplitFunctionOptions_quotedSpacesInSET: SET k TO 'string with spaces'
// must produce a single 4-token clause; the previous code used strings.Fields
// and split inside the quoted value.
func TestSplitFunctionOptions_quotedSpacesInSET(t *testing.T) {
	got := splitFunctionOptions("set application_name to 'my app name' immutable")
	want := []string{"set application_name to 'my app name'", "immutable"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quoted spaces in SET value lost: got %#v want %#v", got, want)
	}
}

func TestSplitFunctionOptions_searchPathListWithComma(t *testing.T) {
	// SET search_path TO '$user', public — list values are comma-separated identifiers
	// after the first quoted value.
	got := splitFunctionOptions("set search_path to '$user', public, app_data parallel safe")
	want := []string{"set search_path to '$user', public, app_data", "parallel safe"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("comma-list SET value lost: got %#v want %#v", got, want)
	}
}

func TestSplitFunctionOptions_singleValueSET(t *testing.T) {
	got := splitFunctionOptions("set search_path to public immutable")
	want := []string{"set search_path to public", "immutable"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("simple SET value mis-split: got %#v want %#v", got, want)
	}
}

func TestSplitFunctionOptions_parallelSecurityCost(t *testing.T) {
	got := splitFunctionOptions("parallel safe security definer cost 100 stable")
	want := []string{"parallel safe", "security definer", "cost 100", "stable"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-word clauses: got %#v want %#v", got, want)
	}
}
