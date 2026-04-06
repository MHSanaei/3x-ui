package padding

import (
	"math/rand"
)

const (
	paddingChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// padding specifies a half-open range [Min, Max).
type Padding struct {
	Min int
	Max int
}

func (p Padding) String() string {
	n := p.Min + rand.Intn(p.Max-p.Min)
	bs := make([]byte, n)
	for i := range bs {
		bs[i] = paddingChars[rand.Intn(len(paddingChars))]
	}
	return string(bs)
}
