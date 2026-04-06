package sudoku

import (
	"fmt"
	"io"
	"net"
)

type packedEncoder struct {
	layouts    []*byteLayout
	codec      *codec
	groupIndex int
}

func newPackedEncoder(tables []*table, pMin, pMax int) *packedEncoder {
	layouts := make([]*byteLayout, 0, len(tables))
	for _, t := range tables {
		layouts = append(layouts, t.layout)
	}
	if len(layouts) == 0 {
		layouts = append(layouts, entropyLayout())
	}
	return &packedEncoder{
		layouts: layouts,
		codec:   newCodec(nil, pMin, pMax),
	}
}

func (e *packedEncoder) encode(p []byte) ([]byte, error) {
	out := make([]byte, 0, len(p)*2+8)
	var bitBuf uint64
	var bitCount uint8

	for _, b := range p {
		bitBuf = (bitBuf << 8) | uint64(b)
		bitCount += 8

		for bitCount >= 6 {
			bitCount -= 6
			layout := e.layouts[e.groupIndex%len(e.layouts)]
			group := byte(bitBuf >> bitCount)
			out = e.maybePad(out, layout)
			out = append(out, layout.encodeGroup(group&0x3f))
			e.groupIndex++
			if bitCount > 0 {
				bitBuf &= (uint64(1) << bitCount) - 1
			} else {
				bitBuf = 0
			}
		}
	}

	if bitCount > 0 {
		layout := e.layouts[e.groupIndex%len(e.layouts)]
		group := byte(bitBuf << (6 - bitCount))
		out = e.maybePad(out, layout)
		out = append(out, layout.encodeGroup(group&0x3f))
		e.groupIndex++
		nextLayout := e.layouts[e.groupIndex%len(e.layouts)]
		out = append(out, nextLayout.padMarker)
	}

	out = e.maybePad(out, e.layouts[e.groupIndex%len(e.layouts)])
	return out, nil
}

func (e *packedEncoder) maybePad(out []byte, layout *byteLayout) []byte {
	if !e.codec.shouldPad() {
		return out
	}
	if len(layout.paddingPool) == 1 {
		return append(out, layout.paddingPool[0])
	}
	for {
		b := layout.paddingPool[e.codec.rng.Intn(len(layout.paddingPool))]
		if b != layout.padMarker {
			return append(out, b)
		}
	}
}

type packedStreamDecoder struct {
	layouts    []*byteLayout
	groupIndex int
	bitBuf     uint64
	bitCount   int
}

func (d *packedStreamDecoder) decodeChunk(in []byte, pending []byte) ([]byte, error) {
	var err error
	d.bitBuf, d.bitCount, d.groupIndex, pending, err = decodePackedBytes(
		d.layouts,
		in,
		d.bitBuf,
		d.bitCount,
		d.groupIndex,
		pending,
	)
	return pending, err
}

func (d *packedStreamDecoder) reset() {
	d.bitBuf = 0
	d.bitCount = 0
}

func NewPackedTCPConn(raw net.Conn, config *Config) (net.Conn, error) {
	reader, writer, err := newPackedReaderWriter(raw, config)
	if err != nil {
		return nil, err
	}
	return newWrappedConn(raw, reader, writer), nil
}

func newPackedReaderWriter(raw net.Conn, config *Config) (io.Reader, io.Writer, error) {
	tables, err := getTables(config)
	if err != nil {
		return nil, nil, err
	}

	pMin, pMax := normalizedPadding(config)
	encoder := newPackedEncoder(tables, pMin, pMax)
	decoder := &packedStreamDecoder{
		layouts: tablesToLayouts(tables),
	}
	return newStreamReader(raw, decoder), newStreamWriter(raw, encoder.encode), nil
}

func tablesToLayouts(tables []*table) []*byteLayout {
	layouts := make([]*byteLayout, 0, len(tables))
	for _, t := range tables {
		layouts = append(layouts, t.layout)
	}
	if len(layouts) == 0 {
		layouts = append(layouts, entropyLayout())
	}
	return layouts
}

func decodePackedBytes(
	layouts []*byteLayout,
	in []byte,
	bitBuf uint64,
	bitCount int,
	groupIndex int,
	out []byte,
) (uint64, int, int, []byte, error) {
	if len(layouts) == 0 {
		return bitBuf, bitCount, groupIndex, out, fmt.Errorf("sudoku layout set missing")
	}
	for _, b := range in {
		layout := layouts[groupIndex%len(layouts)]
		if !layout.isHint(b) {
			if b == layout.padMarker {
				bitBuf = 0
				bitCount = 0
			}
			continue
		}

		group, ok := layout.decodeGroup(b)
		if !ok {
			return bitBuf, bitCount, groupIndex, out, fmt.Errorf("invalid packed sudoku byte: %d", b)
		}
		groupIndex++

		bitBuf = (bitBuf << 6) | uint64(group)
		bitCount += 6

		for bitCount >= 8 {
			bitCount -= 8
			out = append(out, byte(bitBuf>>bitCount))
			if bitCount > 0 {
				bitBuf &= (uint64(1) << bitCount) - 1
			} else {
				bitBuf = 0
			}
		}
	}

	return bitBuf, bitCount, groupIndex, out, nil
}
