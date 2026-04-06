package scenarios

import (
	"testing"
)

func TestMain(m *testing.M) {
	genTestBinaryPath()
	defer testBinaryCleanFn()

	m.Run()
}
