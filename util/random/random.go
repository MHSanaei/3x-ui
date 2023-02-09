package random

import (
	"math/rand"
	"time"
)

var numSeq [10]rune
var lowerSeq [26]rune
var upperSeq [26]rune
var numLowerSeq [36]rune
var numUpperSeq [36]rune
var allSeq [62]rune

func init() {
	rand.Seed(time.Now().UnixNano())

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

func Seq(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		runes[i] = allSeq[rand.Intn(len(allSeq))]
	}
	return string(runes)
}
