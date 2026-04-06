package sudoku

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/bits"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

type table struct {
	encode [256][][4]byte
	decode map[uint32]byte
	layout *byteLayout
}

type tableCacheKey struct {
	password    string
	ascii       string
	customTable string
}

var (
	tableCache    sync.Map
	tableSetCache sync.Map

	basePatternsOnce sync.Once
	basePatterns     [][][4]byte
	basePatternsErr  error
)

type byteLayout struct {
	hintMask    byte
	hintValue   byte
	padMarker   byte
	paddingPool []byte
	encodeHint  func(group byte) byte
	encodeGroup func(group byte) byte
	decodeGroup func(b byte) (byte, bool)
}

func (l *byteLayout) isHint(b byte) bool {
	if (b & l.hintMask) == l.hintValue {
		return true
	}
	// ASCII layout maps 0x7f to '\n' to avoid DEL on the wire.
	return l.hintMask == 0x40 && b == '\n'
}

func getTable(config *Config) (*table, error) {
	tables, err := getTables(config)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("empty sudoku table set")
	}
	return tables[0], nil
}

func getTables(config *Config) ([]*table, error) {
	if config == nil {
		return nil, fmt.Errorf("nil sudoku config")
	}

	mode, err := normalizeASCII(config.GetAscii())
	if err != nil {
		return nil, err
	}

	patterns, err := normalizedCustomPatterns(config, mode)
	if err != nil {
		return nil, err
	}

	cacheKey := tableCacheKey{
		password:    config.GetPassword(),
		ascii:       mode,
		customTable: strings.Join(patterns, "\x00"),
	}
	if cached, ok := tableSetCache.Load(cacheKey); ok {
		return cached.([]*table), nil
	}

	tables := make([]*table, 0, len(patterns))
	for _, pattern := range patterns {
		layout, err := resolveLayout(mode, pattern)
		if err != nil {
			return nil, err
		}
		t, err := buildTable(config.GetPassword(), layout)
		if err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}

	actual, _ := tableSetCache.LoadOrStore(cacheKey, tables)
	return actual.([]*table), nil
}

func normalizedCustomPatterns(config *Config, mode string) ([]string, error) {
	if config == nil {
		return []string{""}, nil
	}
	if mode == "prefer_ascii" {
		return []string{""}, nil
	}

	rawPatterns := config.GetCustomTables()
	if len(rawPatterns) == 0 {
		rawPatterns = []string{config.GetCustomTable()}
	}

	patterns := make([]string, 0, len(rawPatterns))
	seen := make(map[string]struct{}, len(rawPatterns))
	for _, raw := range rawPatterns {
		pattern := strings.TrimSpace(raw)
		if pattern != "" {
			var err error
			pattern, err = normalizeCustomTable(pattern)
			if err != nil {
				return nil, err
			}
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		patterns = append(patterns, pattern)
	}

	if len(patterns) == 0 {
		return []string{""}, nil
	}

	return patterns, nil
}

func normalizedPadding(config *Config) (int, int) {
	if config == nil {
		return 0, 0
	}

	pMin := int(config.GetPaddingMin())
	pMax := int(config.GetPaddingMax())

	if pMin > 100 {
		pMin = 100
	}
	if pMax > 100 {
		pMax = 100
	}
	if pMax < pMin {
		pMax = pMin
	}
	return pMin, pMax
}

func normalizeASCII(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "entropy", "prefer_entropy":
		return "prefer_entropy", nil
	case "ascii", "prefer_ascii":
		return "prefer_ascii", nil
	default:
		return "", fmt.Errorf("invalid sudoku ascii mode: %s", mode)
	}
}

func normalizeCustomTable(pattern string) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(pattern))
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if len(cleaned) != 8 {
		return "", fmt.Errorf("customTable must be 8 chars, got %d", len(cleaned))
	}

	var xCount, pCount, vCount int
	for _, ch := range cleaned {
		switch ch {
		case 'x':
			xCount++
		case 'p':
			pCount++
		case 'v':
			vCount++
		default:
			return "", fmt.Errorf("customTable has invalid char %q", ch)
		}
	}
	if xCount != 2 || pCount != 2 || vCount != 4 {
		return "", fmt.Errorf("customTable must contain exactly 2 x, 2 p and 4 v")
	}
	return cleaned, nil
}

func resolveLayout(mode, customTable string) (*byteLayout, error) {
	if mode == "prefer_ascii" {
		return asciiLayout(), nil
	}

	if customTable != "" {
		return customLayout(customTable)
	}
	return entropyLayout(), nil
}

func asciiLayout() *byteLayout {
	padding := make([]byte, 0, 32)
	for i := 0; i < 32; i++ {
		padding = append(padding, byte(0x20+i))
	}

	encodeGroup := func(group byte) byte {
		b := byte(0x40 | (group & 0x3f))
		if b == 0x7f {
			return '\n'
		}
		return b
	}

	return &byteLayout{
		hintMask:    0x40,
		hintValue:   0x40,
		padMarker:   0x3f,
		paddingPool: padding,
		encodeHint:  encodeGroup,
		encodeGroup: encodeGroup,
		decodeGroup: func(b byte) (byte, bool) {
			if b == '\n' {
				return 0x3f, true
			}
			if (b & 0x40) == 0 {
				return 0, false
			}
			return b & 0x3f, true
		},
	}
}

func entropyLayout() *byteLayout {
	padding := make([]byte, 0, 16)
	for i := 0; i < 8; i++ {
		padding = append(padding, byte(0x80+i), byte(0x10+i))
	}

	encodeGroup := func(group byte) byte {
		v := group & 0x3f
		return ((v & 0x30) << 1) | (v & 0x0f)
	}

	return &byteLayout{
		hintMask:    0x90,
		hintValue:   0x00,
		padMarker:   0x80,
		paddingPool: padding,
		encodeHint:  encodeGroup,
		encodeGroup: encodeGroup,
		decodeGroup: func(b byte) (byte, bool) {
			if (b & 0x90) != 0 {
				return 0, false
			}
			return ((b >> 1) & 0x30) | (b & 0x0f), true
		},
	}
}

func customLayout(pattern string) (*byteLayout, error) {
	pattern, err := normalizeCustomTable(pattern)
	if err != nil {
		return nil, err
	}

	var xBits, pBits, vBits []uint8
	for i, c := range pattern {
		bit := uint8(7 - i)
		switch c {
		case 'x':
			xBits = append(xBits, bit)
		case 'p':
			pBits = append(pBits, bit)
		case 'v':
			vBits = append(vBits, bit)
		}
	}

	xMask := byte(0)
	for _, bit := range xBits {
		xMask |= 1 << bit
	}

	encodeGroupWithDropX := func(group byte, dropX int) byte {
		out := xMask
		if dropX >= 0 {
			out &^= 1 << xBits[dropX]
		}

		val := (group >> 4) & 0x03
		pos := group & 0x0f

		if (val & 0x02) != 0 {
			out |= 1 << pBits[0]
		}
		if (val & 0x01) != 0 {
			out |= 1 << pBits[1]
		}
		for i, bit := range vBits {
			if (pos>>(3-uint8(i)))&0x01 == 1 {
				out |= 1 << bit
			}
		}

		return out
	}

	paddingSet := make(map[byte]struct{}, 64)
	padding := make([]byte, 0, 64)
	for drop := range xBits {
		for val := byte(0); val < 4; val++ {
			for pos := byte(0); pos < 16; pos++ {
				group := (val << 4) | pos
				b := encodeGroupWithDropX(group, drop)
				if bits.OnesCount8(b) >= 5 {
					if _, exists := paddingSet[b]; !exists {
						paddingSet[b] = struct{}{}
						padding = append(padding, b)
					}
				}
			}
		}
	}
	sort.Slice(padding, func(i, j int) bool { return padding[i] < padding[j] })
	if len(padding) == 0 {
		return nil, fmt.Errorf("customTable produced empty padding pool")
	}

	decodeGroup := func(b byte) (byte, bool) {
		if (b & xMask) != xMask {
			return 0, false
		}

		var val, pos byte
		if b&(1<<pBits[0]) != 0 {
			val |= 0x02
		}
		if b&(1<<pBits[1]) != 0 {
			val |= 0x01
		}
		for i, bit := range vBits {
			if b&(1<<bit) != 0 {
				pos |= 1 << (3 - uint8(i))
			}
		}

		return ((val & 0x03) << 4) | (pos & 0x0f), true
	}
	encodeGroup := func(group byte) byte {
		return encodeGroupWithDropX(group, -1)
	}

	return &byteLayout{
		hintMask:    xMask,
		hintValue:   xMask,
		padMarker:   padding[0],
		paddingPool: padding,
		encodeHint:  encodeGroup,
		encodeGroup: encodeGroup,
		decodeGroup: decodeGroup,
	}, nil
}

func buildTable(password string, layout *byteLayout) (*table, error) {
	patterns, err := getBasePatterns()
	if err != nil {
		return nil, err
	}
	if len(patterns) < 256 {
		return nil, fmt.Errorf("not enough sudoku grids: %d", len(patterns))
	}

	order := make([]int, len(patterns))
	for i := range order {
		order[i] = i
	}

	hash := sha256.Sum256([]byte(password))
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})

	t := &table{
		decode: make(map[uint32]byte, 1<<16),
		layout: layout,
	}
	for b := 0; b < 256; b++ {
		patList := patterns[order[b]]
		if len(patList) == 0 {
			return nil, fmt.Errorf("grid %d has no valid clue set", order[b])
		}

		enc := make([][4]byte, 0, len(patList))
		for _, groups := range patList {
			hints := [4]byte{
				layout.encodeHint(groups[0]),
				layout.encodeHint(groups[1]),
				layout.encodeHint(groups[2]),
				layout.encodeHint(groups[3]),
			}
			sortedHints := sort4(hints)
			key := packKey(sortedHints)
			if old, exists := t.decode[key]; exists && old != byte(b) {
				return nil, fmt.Errorf("decode key collision for byte %d and %d", old, b)
			}
			t.decode[key] = byte(b)
			enc = append(enc, hints)
		}

		t.encode[b] = enc
	}

	return t, nil
}

func getBasePatterns() ([][][4]byte, error) {
	basePatternsOnce.Do(func() {
		basePatterns, basePatternsErr = buildBasePatterns()
	})
	return basePatterns, basePatternsErr
}

type grid [16]byte

func buildBasePatterns() ([][][4]byte, error) {
	grids := generateAllGrids()
	positions := hintPositions()

	patterns := make([][][4]byte, len(grids))
	for _, ps := range positions {
		counts := make(map[uint32]uint16, len(grids))
		keys := make([]uint32, len(grids))
		groupsByGrid := make([][4]byte, len(grids))

		for gi, g := range grids {
			groups := [4]byte{
				clueGroup(g, ps[0]),
				clueGroup(g, ps[1]),
				clueGroup(g, ps[2]),
				clueGroup(g, ps[3]),
			}
			groups = sort4(groups)
			key := packKey(groups)
			keys[gi] = key
			groupsByGrid[gi] = groups
			counts[key]++
		}

		for gi, key := range keys {
			if counts[key] == 1 {
				patterns[gi] = append(patterns[gi], groupsByGrid[gi])
			}
		}
	}

	for gi, list := range patterns {
		if len(list) == 0 {
			return nil, fmt.Errorf("grid %d has no uniquely decodable clue set", gi)
		}
	}

	return patterns, nil
}

func clueGroup(g grid, pos byte) byte {
	// 2 bits of value + 4 bits of position.
	return ((g[pos] - 1) << 4) | (pos & 0x0f)
}

func generateAllGrids() []grid {
	grids := make([]grid, 0, 288)
	var g grid

	var dfs func(idx int)
	dfs = func(idx int) {
		if idx == 16 {
			grids = append(grids, g)
			return
		}

		row := idx / 4
		col := idx % 4
		boxRow := (row / 2) * 2
		boxCol := (col / 2) * 2

		for num := byte(1); num <= 4; num++ {
			valid := true
			for i := 0; i < 4; i++ {
				if g[row*4+i] == num || g[i*4+col] == num {
					valid = false
					break
				}
			}
			if !valid {
				continue
			}

			for r := 0; r < 2 && valid; r++ {
				for c := 0; c < 2; c++ {
					if g[(boxRow+r)*4+(boxCol+c)] == num {
						valid = false
						break
					}
				}
			}
			if !valid {
				continue
			}

			g[idx] = num
			dfs(idx + 1)
			g[idx] = 0
		}
	}

	dfs(0)
	return grids
}

func hintPositions() [][4]byte {
	// C(16, 4) = 1820.
	positions := make([][4]byte, 0, 1820)
	for a := 0; a < 13; a++ {
		for b := a + 1; b < 14; b++ {
			for c := b + 1; c < 15; c++ {
				for d := c + 1; d < 16; d++ {
					positions = append(positions, [4]byte{byte(a), byte(b), byte(c), byte(d)})
				}
			}
		}
	}
	return positions
}

func packKey(in [4]byte) uint32 {
	return uint32(in[0])<<24 | uint32(in[1])<<16 | uint32(in[2])<<8 | uint32(in[3])
}

func sort4(in [4]byte) [4]byte {
	if in[0] > in[1] {
		in[0], in[1] = in[1], in[0]
	}
	if in[2] > in[3] {
		in[2], in[3] = in[3], in[2]
	}
	if in[0] > in[2] {
		in[0], in[2] = in[2], in[0]
	}
	if in[1] > in[3] {
		in[1], in[3] = in[3], in[1]
	}
	if in[1] > in[2] {
		in[1], in[2] = in[2], in[1]
	}
	return in
}

func newSeededRand() *rand.Rand {
	seed := time.Now().UnixNano()
	var seedBytes [8]byte
	if _, err := crypto_rand.Read(seedBytes[:]); err == nil {
		seed = int64(binary.BigEndian.Uint64(seedBytes[:]))
	}
	return rand.New(rand.NewSource(seed))
}
