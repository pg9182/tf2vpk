package internal

import "testing"

func TestMatchGlobParents(t *testing.T) {
	for _, x := range []struct {
		Pattern string
		Path    string
		Match   bool
		Error   bool
	}{
		{"/", "", true, false},
		{"/", "test", true, false},
		{"/", "a/b/c", true, false},
		{"*", "", false, false},
		{"*", "test", true, false},
		{"/test", "test", true, false},
		{"test", "test", true, false},
		{"test", "test1/test", true, false},
		{"/test", "test1/test", false, false},
		{"test", "test1/test", true, false},
		{"test", "test/test1", true, false},
		{"a", "a/b/c", true, false},
		{"b", "a/b/c", true, false},
		{"c", "a/b/c", true, false},
		{"a/b", "a/b/c", true, false},
		{"a/b/c", "a/b/c", true, false},
		{"b/c", "a/b/c", false, false}, // multiple components are treated as anchored
		{"/a/b", "a/b/c", true, false},
		{"/a/b/c", "a/b/c", true, false},
		{"/b/c", "a/b/c", false, false},
		{"*x*", "axa/b/c", true, false},
		{"*x*", "a/xb/c", true, false},
		{"*x*", "a/b/x", true, false},
		{"/*x*", "axa/b/c", true, false},
		{"/*x*", "a/xb/c", false, false},
		{"/*x*", "a/b/x", false, false},
		// note: don't need to test full glob semantics since we use path.Match
	} {
		matched, err := MatchGlobParents(x.Pattern, x.Path)
		t.Log()
		t.Logf("LOG: match(%q, %q) = %t, %v", x.Pattern, x.Path, matched, err)

		if matched != x.Match {
			if x.Match {
				t.Errorf("ERR: match(%q, %q) expected match", x.Pattern, x.Path)
			} else {
				t.Errorf("ERR: match(%q, %q) expected no match", x.Pattern, x.Path)
			}
		}
		if err != nil != x.Error {
			if x.Error {
				t.Errorf("ERR: match(%q, %q) expected error, got nil", x.Pattern, x.Path)
			} else {
				t.Errorf("ERR: match(%q, %q) expected no error, got %v", x.Pattern, x.Path, err)
			}
		}
	}
}
