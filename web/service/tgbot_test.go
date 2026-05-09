package service

import (
	"reflect"
	"testing"
)

func TestLoginAttemptDoesNotCarryPassword(t *testing.T) {
	typ := reflect.TypeOf(LoginAttempt{})
	if _, ok := typ.FieldByName("Password"); ok {
		t.Fatal("LoginAttempt must not carry attempted passwords")
	}
}
