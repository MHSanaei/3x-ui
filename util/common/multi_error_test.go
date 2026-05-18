package common

import (
	"errors"
	"strings"
	"testing"
)

func TestCombine_AllNilReturnsNil(t *testing.T) {
	if err := Combine(); err != nil {
		t.Fatalf("Combine() with no args = %v, want nil", err)
	}
	if err := Combine(nil, nil, nil); err != nil {
		t.Fatalf("Combine(nil, nil, nil) = %v, want nil", err)
	}
}

func TestCombine_SkipsNilErrors(t *testing.T) {
	e1 := errors.New("boom one")
	e2 := errors.New("boom two")

	err := Combine(nil, e1, nil, e2, nil)
	if err == nil {
		t.Fatal("expected non-nil combined error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom one") || !strings.Contains(msg, "boom two") {
		t.Fatalf("combined error %q does not contain both underlying messages", msg)
	}
	if !strings.HasPrefix(msg, "multierr: ") {
		t.Fatalf("combined error %q missing %q prefix", msg, "multierr: ")
	}
}

func TestCombine_SingleErrorStillWrapped(t *testing.T) {
	e := errors.New("only one")
	err := Combine(e)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "only one") {
		t.Fatalf("combined error %q missing underlying message", err.Error())
	}
}
