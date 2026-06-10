package service

import "testing"

func TestValidateClientEmail(t *testing.T) {
	valid := []string{
		"alice",
		"alice@example.com",
		"user-123_test.name",
		"имя",
	}
	for _, email := range valid {
		if err := validateClientEmail(email); err != nil {
			t.Errorf("validateClientEmail(%q) = %v, want nil", email, err)
		}
	}

	invalid := []string{
		"i6dui/",
		"a/b",
		"client with spaces",
		"back\\slash",
		"tab\there",
		"new\nline",
		"\x7fdelete",
	}
	for _, email := range invalid {
		if err := validateClientEmail(email); err == nil {
			t.Errorf("validateClientEmail(%q) = nil, want error", email)
		}
	}
}

func TestValidateClientSubID(t *testing.T) {
	valid := []string{
		"",
		"abc123",
		"sub-id_value",
	}
	for _, subID := range valid {
		if err := validateClientSubID(subID); err != nil {
			t.Errorf("validateClientSubID(%q) = %v, want nil", subID, err)
		}
	}

	invalid := []string{
		"a/b",
		"with space",
		"back\\slash",
		"new\nline",
	}
	for _, subID := range invalid {
		if err := validateClientSubID(subID); err == nil {
			t.Errorf("validateClientSubID(%q) = nil, want error", subID)
		}
	}
}
