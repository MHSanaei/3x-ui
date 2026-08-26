//go:build !linux

package tuic

func killStrayTuicProcesses(_ string) int { return 0 }
