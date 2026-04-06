package utils

import (
	"math/rand/v2"
)

var (
	// 8 ÷ (397/62)
	h2packCorrectionFactor = 1.2493702770780857
	base62TotalCharsNum    = 62
	base62Chars            = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// H2Base62Pad generates a base62 padding string for HTTP/2 header
// The total len will be slightly longer than the input to match the length after h2(h3 also) header huffman encoding
func H2Base62Pad[T int32 | int64 | int](expectedLen T) string {
	actualLenFloat := float64(expectedLen) * h2packCorrectionFactor
	actualLen := int(actualLenFloat)
	result := make([]byte, actualLen)
	for i := range actualLen {
		result[i] = base62Chars[rand.N(base62TotalCharsNum)]
	}
	return string(result)
}
