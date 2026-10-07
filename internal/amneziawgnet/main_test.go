package amneziawgnet

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	wildcardBindHost = "127.0.0.1"
	os.Exit(m.Run())
}
