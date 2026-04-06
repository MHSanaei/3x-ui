// Package crypto provides common crypto libraries for Xray.
package crypto // import "github.com/xtls/xray-core/common/crypto"

import (
	"crypto/rand"
	"math/big"

	"github.com/xtls/xray-core/common"
)

// [,)
func RandBetween(from int64, to int64) int64 {
	if from == to {
		return from
	}
	if from > to {
		from, to = to, from
	}
	bigInt, _ := rand.Int(rand.Reader, big.NewInt(to-from))
	return from + bigInt.Int64()
}

// [,]
func RandBytesBetween(b []byte, from, to byte) {
	common.Must2(rand.Read(b))

	if from > to {
		from, to = to, from
	}

	if to-from == 255 {
		return
	}

	for i := range b {
		b[i] = from + b[i]%(to-from+1)
	}
}
