package sudoku

import (
	"fmt"
	"math/rand"
)

var perm4 = [24][4]byte{
	{0, 1, 2, 3},
	{0, 1, 3, 2},
	{0, 2, 1, 3},
	{0, 2, 3, 1},
	{0, 3, 1, 2},
	{0, 3, 2, 1},
	{1, 0, 2, 3},
	{1, 0, 3, 2},
	{1, 2, 0, 3},
	{1, 2, 3, 0},
	{1, 3, 0, 2},
	{1, 3, 2, 0},
	{2, 0, 1, 3},
	{2, 0, 3, 1},
	{2, 1, 0, 3},
	{2, 1, 3, 0},
	{2, 3, 0, 1},
	{2, 3, 1, 0},
	{3, 0, 1, 2},
	{3, 0, 2, 1},
	{3, 1, 0, 2},
	{3, 1, 2, 0},
	{3, 2, 0, 1},
	{3, 2, 1, 0},
}

type codec struct {
	tables        []*table
	rng           *rand.Rand
	paddingChance int
	tableIndex    int
}

func newCodec(tables []*table, pMin, pMax int) *codec {
	if len(tables) == 0 {
		tables = nil
	}
	rng := newSeededRand()
	return &codec{
		tables:        tables,
		rng:           rng,
		paddingChance: pickPaddingChance(rng, pMin, pMax),
	}
}

func pickPaddingChance(rng *rand.Rand, pMin, pMax int) int {
	if pMin < 0 {
		pMin = 0
	}
	if pMax < pMin {
		pMax = pMin
	}
	if pMin > 100 {
		pMin = 100
	}
	if pMax > 100 {
		pMax = 100
	}
	if pMax == pMin {
		return pMin
	}
	return pMin + rng.Intn(pMax-pMin+1)
}

func (c *codec) shouldPad() bool {
	if c.paddingChance <= 0 {
		return false
	}
	if c.paddingChance >= 100 {
		return true
	}
	return c.rng.Intn(100) < c.paddingChance
}

func (c *codec) currentTable() *table {
	if len(c.tables) == 0 {
		return nil
	}
	return c.tables[c.tableIndex%len(c.tables)]
}

func (c *codec) randomPadding(t *table) byte {
	pool := t.layout.paddingPool
	return pool[c.rng.Intn(len(pool))]
}

func (c *codec) encode(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}

	out := make([]byte, 0, len(in)*6+8)
	for _, b := range in {
		t := c.currentTable()
		if t == nil {
			return nil, fmt.Errorf("sudoku table set missing")
		}
		if c.shouldPad() {
			out = append(out, c.randomPadding(t))
		}

		enc := t.encode[b]
		if len(enc) == 0 {
			return nil, fmt.Errorf("sudoku encode table missing for byte %d", b)
		}

		hints := enc[c.rng.Intn(len(enc))]
		perm := perm4[c.rng.Intn(len(perm4))]
		for _, idx := range perm {
			if c.shouldPad() {
				out = append(out, c.randomPadding(t))
			}
			out = append(out, hints[idx])
		}
		c.tableIndex++
	}

	if c.shouldPad() {
		if t := c.currentTable(); t != nil {
			out = append(out, c.randomPadding(t))
		}
	}

	return out, nil
}

func decodeBytes(tables []*table, tableIndex *int, in []byte, hintBuf []byte, out []byte) ([]byte, []byte, error) {
	if len(tables) == 0 {
		return hintBuf, out, fmt.Errorf("sudoku table set missing")
	}
	for _, b := range in {
		t := tables[*tableIndex%len(tables)]
		if !t.layout.isHint(b) {
			continue
		}

		hintBuf = append(hintBuf, b)
		if len(hintBuf) < 4 {
			continue
		}

		keyBytes := sort4([4]byte{hintBuf[0], hintBuf[1], hintBuf[2], hintBuf[3]})
		key := packKey(keyBytes)
		decoded, ok := t.decode[key]
		if !ok {
			return hintBuf[:0], out, fmt.Errorf("invalid sudoku hint tuple")
		}

		out = append(out, decoded)
		hintBuf = hintBuf[:0]
		*tableIndex++
	}

	return hintBuf, out, nil
}
