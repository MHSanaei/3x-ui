//go:build !linux

package tuic

func readProcIO(pid int) (int64, int64, error) {
	return 0, 0, nil
}
