package entity

import "testing"

func TestPathHasForbiddenChar(t *testing.T) {
	valid := []string{
		"",
		"/",
		"/sub/",
		"/json/",
		"/a/b/c/",
		"/My-Path_123/",
	}
	for _, p := range valid {
		if pathHasForbiddenChar(p) {
			t.Errorf("pathHasForbiddenChar(%q) = true, want false", p)
		}
	}

	invalid := []string{
		"/sub path/",
		"/back\\slash/",
		"/tab\there/",
		"/new\nline/",
		"/\x7f/",
	}
	for _, p := range invalid {
		if !pathHasForbiddenChar(p) {
			t.Errorf("pathHasForbiddenChar(%q) = false, want true", p)
		}
	}
}
