// Package random provides utilities for generating random strings and numbers.
package random

import (
	"crypto/rand"
	"math/big"
)

var (
	numSeq      [10]rune
	lowerSeq    [26]rune
	upperSeq    [26]rune
	numLowerSeq [36]rune
	numUpperSeq [36]rune
	allSeq      [62]rune
)

// init initializes the character sequences used for random string generation.
// It sets up arrays for numbers, lowercase letters, uppercase letters, and combinations.
func init() {
	for i := 0; i < 10; i++ {
		numSeq[i] = rune('0' + i)
	}
	for i := 0; i < 26; i++ {
		lowerSeq[i] = rune('a' + i)
		upperSeq[i] = rune('A' + i)
	}

	copy(numLowerSeq[:], numSeq[:])
	copy(numLowerSeq[len(numSeq):], lowerSeq[:])

	copy(numUpperSeq[:], numSeq[:])
	copy(numUpperSeq[len(numSeq):], upperSeq[:])

	copy(allSeq[:], numSeq[:])
	copy(allSeq[len(numSeq):], lowerSeq[:])
	copy(allSeq[len(numSeq)+len(lowerSeq):], upperSeq[:])
}

// Seq generates a random string of length n containing alphanumeric characters (numbers, lowercase and uppercase letters).
func Seq(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allSeq))))
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		runes[i] = allSeq[idx.Int64()]
	}
	return string(runes)
}

// Num generates a random integer between 0 and n-1.
func Num(n int) int {
	bn := big.NewInt(int64(n))
	r, err := rand.Int(rand.Reader, bn)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(r.Int64())
}
