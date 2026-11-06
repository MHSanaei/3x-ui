//go:build windows

package wattunnel

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateWindowsACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "key")
	if err := WritePrivate(path, []byte("private-key")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{dir, path} {
		sd, err := windows.GetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		descriptor := sd.String()
		if !strings.Contains(descriptor, "D:P") || strings.Contains(descriptor, ";;;WD)") || strings.Contains(descriptor, ";;;AU)") || strings.Contains(descriptor, ";;;BU)") {
			t.Fatalf("unrestricted ACL: %s", descriptor)
		}
	}
}
