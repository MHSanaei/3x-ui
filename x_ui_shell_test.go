package main

import (
	"os"
	"regexp"
	"testing"
)

// x-ui.sh defines install() as the panel installer. System file installation
// must therefore bypass shell functions explicitly, otherwise these paths can
// recursively start a panel installation/update instead of creating a file.
func TestShellScriptDoesNotShadowSystemInstall(t *testing.T) {
	script, err := os.ReadFile("x-ui.sh")
	if err != nil {
		t.Fatalf("read x-ui.sh: %v", err)
	}

	bareInstall := regexp.MustCompile(`(?m)^\s*install\s+-`)
	if match := bareInstall.Find(script); match != nil {
		t.Fatalf("x-ui.sh contains a bare system install invocation %q; use command install", match)
	}
}
