package main

import (
	"strings"
	"testing"
)

func TestCommandHelpListsEncryptTokens(t *testing.T) {
	if help := commandHelp(); !strings.Contains(help, "encrypt-tokens") {
		t.Fatalf("command help omits encrypt-tokens:\n%s", help)
	}
}
