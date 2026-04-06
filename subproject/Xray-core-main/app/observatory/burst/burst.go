package burst

import (
	"math"
	"time"
)

const (
	rttFailed = time.Duration(math.MaxInt64 - iota)
	rttUntested
	rttUnqualified
)
