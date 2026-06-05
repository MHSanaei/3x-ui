package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
)

func setupUserTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatal(err)
		}
	})
}

func validRegisterInput() RegisterInput {
	return RegisterInput{
		FullName: "Jane Doe",
		Phone:    "+1 555 123 4567",
		Email:    "jane@example.com",
		Username: "jane_doe",
		Password: "Sup3rSecret",
	}
}

func TestRegisterCreatesUserWithHashedPassword(t *testing.T) {
	setupUserTestDB(t)
	s := &UserService{}

	user, err := s.Register(validRegisterInput())
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if user.Id == 0 {
		t.Fatal("expected persisted user to have an id")
	}
	if user.Password != "" {
		t.Fatal("returned user must not carry the password hash")
	}

	// Confirm the stored password is a bcrypt hash that verifies, not plaintext.
	stored, err := s.CheckUser("jane_doe", "Sup3rSecret", "")
	if err != nil {
		t.Fatalf("CheckUser failed for the new account: %v", err)
	}
	if stored.Email != "jane@example.com" || stored.FullName != "Jane Doe" || stored.Phone != "+1 555 123 4567" {
		t.Fatalf("profile fields not persisted: %+v", stored)
	}
}

func TestRegisterRejectsDuplicateUsernameCaseInsensitive(t *testing.T) {
	setupUserTestDB(t)
	s := &UserService{}

	if _, err := s.Register(validRegisterInput()); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	dup := validRegisterInput()
	dup.Username = "JANE_DOE"
	dup.Email = "other@example.com"
	if _, err := s.Register(dup); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestRegisterRejectsDuplicateEmailCaseInsensitive(t *testing.T) {
	setupUserTestDB(t)
	s := &UserService{}

	if _, err := s.Register(validRegisterInput()); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	dup := validRegisterInput()
	dup.Username = "another_user"
	dup.Email = "JANE@EXAMPLE.COM"
	if _, err := s.Register(dup); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegisterValidatesFields(t *testing.T) {
	setupUserTestDB(t)
	s := &UserService{}

	cases := []struct {
		name    string
		mutate  func(in *RegisterInput)
		wantErr error
	}{
		{"short username", func(in *RegisterInput) { in.Username = "ab" }, ErrInvalidUsername},
		{"bad username chars", func(in *RegisterInput) { in.Username = "jane doe!" }, ErrInvalidUsername},
		{"bad email", func(in *RegisterInput) { in.Email = "not-an-email" }, ErrInvalidEmail},
		{"bad phone", func(in *RegisterInput) { in.Phone = "abc" }, ErrInvalidPhone},
		{"short full name", func(in *RegisterInput) { in.FullName = "J" }, ErrInvalidFullName},
		{"weak password no digit", func(in *RegisterInput) { in.Password = "Password" }, ErrWeakPassword},
		{"weak password too short", func(in *RegisterInput) { in.Password = "Ab1" }, ErrWeakPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validRegisterInput()
			tc.mutate(&in)
			if _, err := s.Register(in); !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	good := []string{"Sup3rSecret", "aB3aaaaa", "LongerPass1"}
	for _, pw := range good {
		if err := ValidatePasswordStrength(pw); err != nil {
			t.Errorf("expected %q to be strong enough, got %v", pw, err)
		}
	}
	bad := []string{"", "short1A", "alllowercase1", "ALLUPPERCASE1", "NoDigitsHere"}
	for _, pw := range bad {
		if err := ValidatePasswordStrength(pw); !errors.Is(err, ErrWeakPassword) {
			t.Errorf("expected %q to be rejected as weak, got %v", pw, err)
		}
	}
}
