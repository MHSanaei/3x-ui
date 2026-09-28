// Package clientpolicy defines shared client accounting and data-path policy.
package clientpolicy

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

const (
	MultiplierOne Multiplier = 1000
	multiplierMax Multiplier = 1000000
	scale         int64      = 1000
)

var (
	ErrInvalidMultiplier = errors.New("multiplier must be a decimal between 0.001 and 1000 with at most three fractional digits")
	ErrInvalidUsage      = errors.New("usage must be nonnegative and remainder must be between 0 and 999")
	ErrOverflow          = errors.New("client accounting exceeds int64 byte range")
)

type Multiplier int64

func ParseMultiplier(input string) (Multiplier, error) {
	whole, fraction, hasFraction := strings.Cut(input, ".")
	if len(whole) == 0 || len(whole) > 4 || (len(whole) > 1 && whole[0] == '0') ||
		(hasFraction && (len(fraction) == 0 || len(fraction) > 3)) {
		return 0, ErrInvalidMultiplier
	}
	for _, part := range []string{whole, fraction} {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, ErrInvalidMultiplier
			}
		}
	}
	n, err := strconv.ParseInt(whole+fraction+strings.Repeat("0", 3-len(fraction)), 10, 64)
	if err != nil || !Multiplier(n).valid() {
		return 0, ErrInvalidMultiplier
	}
	return Multiplier(n), nil
}

func (m Multiplier) valid() bool {
	return m > 0 && m <= multiplierMax
}

func (m Multiplier) String() string {
	whole := strconv.FormatInt(int64(m)/scale, 10)
	if int64(m)%scale == 0 {
		return whole
	}
	return whole + "." + strings.TrimRight(fmt.Sprintf("%03d", int64(m)%scale), "0")
}

func Charge(raw int64, multiplier Multiplier, remainder int64) (whole, carry int64, err error) {
	if !multiplier.valid() {
		return 0, 0, ErrInvalidMultiplier
	}
	if raw < 0 || remainder < 0 || remainder >= scale {
		return 0, 0, ErrInvalidUsage
	}
	m := int64(multiplier)
	// Dividing before multiplying permits valid results whose intermediate raw*m overflows.
	tail := (raw%scale)*m + remainder
	extra := tail / scale
	if raw/scale > (math.MaxInt64-extra)/m {
		return 0, 0, ErrOverflow
	}
	return (raw/scale)*m + extra, tail % scale, nil
}

func RawAllowance(remaining, remainder int64, multiplier Multiplier) (int64, error) {
	if !multiplier.valid() {
		return 0, ErrInvalidMultiplier
	}
	if remaining < 0 || remainder < 0 || remainder >= scale {
		return 0, ErrInvalidUsage
	}
	if remaining == 0 {
		return 0, nil
	}
	hi, lo := bits.Mul64(uint64(remaining), uint64(scale))
	lo, borrow := bits.Sub64(lo, uint64(remainder), 0)
	hi, _ = bits.Sub64(hi, 0, borrow)
	if hi >= uint64(multiplier) {
		return 0, ErrOverflow
	}
	allowance, _ := bits.Div64(hi, lo, uint64(multiplier))
	if allowance > math.MaxInt64 {
		return 0, ErrOverflow
	}
	return int64(allowance), nil
}
