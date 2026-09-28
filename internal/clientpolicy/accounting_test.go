package clientpolicy

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestMultiplierAcceptsExactPositiveDecimals(t *testing.T) {
	for _, tc := range []struct {
		input string
		units Multiplier
		text  string
	}{
		{"0.001", 1, "0.001"},
		{"0.5", 500, "0.5"},
		{"1", 1000, "1"},
		{"1.000", 1000, "1"},
		{"1.5", 1500, "1.5"},
		{"2", 2000, "2"},
		{"10", 10000, "10"},
		{"999.999", 999999, "999.999"},
		{"1000", 1000000, "1000"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseMultiplier(tc.input)
			if err != nil || got != tc.units || got.String() != tc.text {
				t.Fatalf("ParseMultiplier(%q) = %d, %q, %v; want %d, %q", tc.input, got, got.String(), err, tc.units, tc.text)
			}
		})
	}
}

func TestMultiplierRejectsFreeInvalidAndImpreciseValues(t *testing.T) {
	for _, input := range []string{"", "0", "0.000", "-1", "+1", "NaN", "Infinity", "1e3", "1.0001", "1000.001", "1001", " 1", "1 ", ".5", "1.", "1.2.3", "01", "1/2", "１２", "999999999999999999999999"} {
		t.Run(input, func(t *testing.T) {
			got, err := ParseMultiplier(input)
			if !errors.Is(err, ErrInvalidMultiplier) || got != 0 {
				t.Fatalf("ParseMultiplier(%q) = %d, %v; want 0, ErrInvalidMultiplier", input, got, err)
			}
		})
	}
}

func TestChargePreservesFractionsRegardlessOfBatching(t *testing.T) {
	for _, tc := range []struct {
		multiplier Multiplier
		whole      int64
		carry      int64
	}{
		{500, 500000, 500},
		{1000, 1000001, 0},
		{1500, 1500001, 500},
		{2000, 2000002, 0},
		{10000, 10000010, 0},
	} {
		t.Run(tc.multiplier.String(), func(t *testing.T) {
			for _, batch := range []int64{1, 7, 1024, 1000001} {
				var billed, carry int64
				for remaining := int64(1000001); remaining > 0; {
					n := min(batch, remaining)
					whole, next, err := Charge(n, tc.multiplier, carry)
					if err != nil {
						t.Fatal(err)
					}
					billed += whole
					carry = next
					remaining -= n
				}
				if billed != tc.whole || carry != tc.carry {
					t.Fatalf("batch=%d: billed=(%d,%d); want (%d,%d)", batch, billed, carry, tc.whole, tc.carry)
				}
			}
		})
	}
}

func TestChargeSettlesSegmentsWithoutRepricingHistory(t *testing.T) {
	first, carry, err := Charge(10<<30, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, carry, err := Charge(5<<30, 2000, carry)
	if err != nil || first+second != 20<<30 || carry != 0 {
		t.Fatalf("segmented 10 GiB + 5 GiB = (%d,%d), %v; want 20 GiB", first+second, carry, err)
	}
	first, carry, err = Charge(1, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	second, carry, err = Charge(1, 2000, carry)
	if err != nil || first+second != 2 || carry != 500 {
		t.Fatalf("fraction across revision = (%d,%d), %v; want (2,500)", first+second, carry, err)
	}
}

func TestChargeChecksFinalOverflowWithoutRejectingLargeIntermediateProduct(t *testing.T) {
	for _, tc := range []struct {
		raw, carry int64
		multiplier Multiplier
		whole, rem int64
		wantErr    error
	}{
		{math.MaxInt64, 0, 1000, math.MaxInt64, 0, nil},
		{math.MaxInt64, 0, 500, 4611686018427387903, 500, nil},
		{math.MaxInt64, 999, 1, 9223372036854776, 806, nil},
		{0, 999, 1000000, 0, 999, nil},
		{math.MaxInt64, 0, 1001, 0, 0, ErrOverflow},
		{math.MaxInt64, 999, 1000000, 0, 0, ErrOverflow},
		{-1, 0, 1000, 0, 0, ErrInvalidUsage},
		{1, -1, 1000, 0, 0, ErrInvalidUsage},
		{1, 1000, 1000, 0, 0, ErrInvalidUsage},
		{1, 0, 0, 0, 0, ErrInvalidMultiplier},
		{1, 0, -1, 0, 0, ErrInvalidMultiplier},
		{1, 0, 1000001, 0, 0, ErrInvalidMultiplier},
	} {
		whole, carry, err := Charge(tc.raw, tc.multiplier, tc.carry)
		if !errors.Is(err, tc.wantErr) || whole != tc.whole || carry != tc.rem {
			t.Errorf("Charge(%d,%d,%d)=(%d,%d,%v); want (%d,%d,%v)", tc.raw, tc.multiplier, tc.carry, whole, carry, err, tc.whole, tc.rem, tc.wantErr)
		}
	}
}

func TestRawAllowanceNeverSpendsFractionalQuotaTwice(t *testing.T) {
	for _, tc := range []struct {
		remaining, carry int64
		multiplier       Multiplier
		want             int64
		wantErr          error
	}{
		{100 << 20, 0, 500, 200 << 20, nil},
		{100 << 20, 0, 1000, 100 << 20, nil},
		{100 << 20, 0, 1500, 69905066, nil},
		{100 << 20, 0, 2000, 50 << 20, nil},
		{100 << 20, 0, 10000, 10485760, nil},
		{1, 0, 500, 2, nil},
		{1, 500, 500, 1, nil},
		{1, 500, 1000, 0, nil},
		{1, 0, 1500, 0, nil},
		{0, 0, 1000, 0, nil},
		{0, 999, 1000, 0, nil},
		{math.MaxInt64, 0, 1000, math.MaxInt64, nil},
		{math.MaxInt64, 0, 1000000, 9223372036854775, nil},
		{math.MaxInt64, 999, 1000, math.MaxInt64 - 1, nil},
		{math.MaxInt64, 0, 500, 0, ErrOverflow},
		{-1, 0, 1000, 0, ErrInvalidUsage},
		{1, 1000, 1000, 0, ErrInvalidUsage},
		{1, -1, 1000, 0, ErrInvalidUsage},
		{1, 0, 0, 0, ErrInvalidMultiplier},
	} {
		got, err := RawAllowance(tc.remaining, tc.carry, tc.multiplier)
		if !errors.Is(err, tc.wantErr) || got != tc.want {
			t.Errorf("RawAllowance(%d,%d,%d)=(%d,%v); want (%d,%v)", tc.remaining, tc.carry, tc.multiplier, got, err, tc.want, tc.wantErr)
		}
	}
}

func FuzzChargeMatchesArbitraryPrecision(f *testing.F) {
	for _, seed := range []struct{ raw, units, carry uint64 }{
		{1, 500, 0}, {math.MaxInt64, 1000, 999}, {999999, 1500, 500}, {math.MaxInt64, 1000000, 0},
	} {
		f.Add(seed.raw, seed.units, seed.carry)
	}
	f.Fuzz(func(t *testing.T, raw, units, carry uint64) {
		n := int64(raw & math.MaxInt64)
		m := Multiplier(units%1000000 + 1)
		r := int64(carry % 1000)
		want := new(big.Int).Mul(big.NewInt(n), big.NewInt(int64(m)))
		want.Add(want, big.NewInt(r))
		rem := new(big.Int)
		want.QuoRem(want, big.NewInt(1000), rem)
		got, gotRem, err := Charge(n, m, r)
		if !want.IsInt64() {
			if !errors.Is(err, ErrOverflow) || got != 0 || gotRem != 0 {
				t.Fatalf("overflow returned (%d,%d,%v)", got, gotRem, err)
			}
			return
		}
		if err != nil || got != want.Int64() || gotRem != rem.Int64() {
			t.Fatalf("charge (%d,%d,%v); want (%s,%s)", got, gotRem, err, want, rem)
		}
	})
}
