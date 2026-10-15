//go:build !linux

package psiphon

func killStrayPsiphonProcesses(_ string) int {
	return 0
}
