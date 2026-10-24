package sys

type LXCStats struct {
	Scope               string
	CPU                 float64
	CPUs                int
	SwapUsed, SwapTotal uint64
	Uptime              uint64
	Unavailable         []string
}
