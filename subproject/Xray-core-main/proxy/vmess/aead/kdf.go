package aead

import (
	"crypto/hmac"
	"crypto/sha256"
	"hash"
)

type hash2 struct {
	hash.Hash
}

func KDF(key []byte, path ...string) []byte {
	hmacf := hmac.New(sha256.New, []byte(KDFSaltConstVMessAEADKDF))

	for _, v := range path {
		first := true
		hmacf = hmac.New(func() hash.Hash {
			if first {
				first = false
				return hash2{hmacf}
			}
			return hmacf
		}, []byte(v))
	}
	hmacf.Write(key)
	return hmacf.Sum(nil)
}

func KDF16(key []byte, path ...string) []byte {
	r := KDF(key, path...)
	return r[:16]
}
