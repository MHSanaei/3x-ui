package common

import (
	"fmt"
)

func FormatTraffic(trafficBytes int64) (size string) {
	if trafficBytes < 1024 {
		return fmt.Sprintf("%.2fB", float64(trafficBytes)/float64(1))
	} else if trafficBytes < (1024 * 1024) {
		return fmt.Sprintf("%.2fKB", float64(trafficBytes)/float64(1024))
	} else if trafficBytes < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fMB", float64(trafficBytes)/float64(1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fGB", float64(trafficBytes)/float64(1024*1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fTB", float64(trafficBytes)/float64(1024*1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2fEB", float64(trafficBytes)/float64(1024*1024*1024*1024*1024))
	}
}
