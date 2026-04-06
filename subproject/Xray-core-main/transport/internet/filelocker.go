package internet

import (
	"os"
)

// FileLocker is UDS access lock
type FileLocker struct {
	path string
	file *os.File
}
