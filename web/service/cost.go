package service

import "math"

// BytesPerGB is the divisor used to convert a client's stored traffic quota
// (which 3x-ui keeps in bytes, despite the "totalGB" field name) into GB for
// pricing.
const BytesPerGB = 1 << 30

// ComputeClientCost returns the wallet credits required to create a client with
// the given traffic quota in bytes:
//
//	cost = base + round( quotaGB * perGB )
//
// where base is the flat per-client fee (clientCost) and perGB is the per-GB
// rate (clientCostPerGB). An unlimited quota (totalBytes == 0) is charged only
// the flat base. The result is never negative.
func ComputeClientCost(base, perGB int, totalBytes int64) int64 {
	cost := int64(base)
	if perGB > 0 && totalBytes > 0 {
		gb := float64(totalBytes) / float64(BytesPerGB)
		cost += int64(math.Round(gb * float64(perGB)))
	}
	if cost < 0 {
		cost = 0
	}
	return cost
}
