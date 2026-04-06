package internet

// Acquire lock
func (fl *FileLocker) Acquire() error {
	return nil
}

// Release lock
func (fl *FileLocker) Release() {
	return
}
