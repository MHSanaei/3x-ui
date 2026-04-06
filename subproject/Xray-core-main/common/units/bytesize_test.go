package units_test

import (
	"testing"

	"github.com/xtls/xray-core/common/units"
)

func TestByteSizes(t *testing.T) {
	size := units.ByteSize(0)
	assertSizeString(t, size, "0")
	size++
	assertSizeValue(t,
		assertSizeString(t, size, "1.00B"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00KB"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00MB"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00GB"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00TB"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00PB"),
		size,
	)
	size <<= 10
	assertSizeValue(t,
		assertSizeString(t, size, "1.00EB"),
		size,
	)
}

func assertSizeValue(t *testing.T, size string, expected units.ByteSize) {
	actual := units.ByteSize(0)
	err := actual.Parse(size)
	if err != nil {
		t.Error(err)
	}
	if actual != expected {
		t.Errorf("expect %s, but got %s", expected, actual)
	}
}

func assertSizeString(t *testing.T, size units.ByteSize, expected string) string {
	actual := size.String()
	if actual != expected {
		t.Errorf("expect %s, but got %s", expected, actual)
	}
	return expected
}
