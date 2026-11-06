package logview

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
)

const (
	chunkSize = 64 << 10
	// maxLineBytes bounds one entry; the head is kept because it holds the timestamp and fields.
	maxLineBytes = 16 << 10
)

// lineVisitor receives a line valid only for the call; returning false ends the scan at that line.
type lineVisitor func(raw []byte, offset int64, truncated bool) bool

type scanOutcome struct {
	// next is the exclusive end of the bytes not yet visited; 0 means none remain.
	next      int64
	scanned   int64
	exhausted bool
}

// scanBackward visits the lines of r[0:end] newest first, reading fixed-size chunks
// so memory stays flat however large the file is and the tail costs the same to reach.
func scanBackward(ctx context.Context, r io.ReaderAt, end, budget int64, visit lineVisitor) (scanOutcome, error) {
	buf := make([]byte, chunkSize)
	var tail, joined []byte
	tailCut := false
	pos := end
	resume := end
	var out scanOutcome

	emit := func(head []byte, offset int64) bool {
		resume = offset
		raw := head
		if len(tail) > 0 {
			joined = append(append(joined[:0], head...), tail...)
			raw = joined
		}
		cut := tailCut
		tail, tailCut = tail[:0], false
		if len(raw) > maxLineBytes {
			raw, cut = raw[:maxLineBytes], true
		}
		raw = bytes.TrimSuffix(raw, []byte{'\r'})
		return len(raw) == 0 || visit(raw, offset, cut)
	}

	for pos > 0 {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if out.scanned >= budget {
			out.exhausted = true
			out.next = resume
			// One line longer than the budget must not pin the cursor in place.
			if resume >= end {
				out.next = pos
			}
			return out, nil
		}
		start := max(pos-chunkSize, 0)
		data := buf[:pos-start]
		if _, err := r.ReadAt(data, start); err != nil {
			// The prune job can truncate the file mid-read; what follows no longer exists.
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		out.scanned += pos - start
		for {
			i := bytes.LastIndexByte(data, '\n')
			if i < 0 {
				break
			}
			offset := start + int64(i) + 1
			if !emit(data[i+1:], offset) {
				out.next = offset
				return out, nil
			}
			data = data[:i]
		}
		tail, tailCut = prependHead(data, tail, tailCut)
		pos = start
	}
	if len(tail) > 0 {
		if !emit(nil, 0) {
			return out, nil
		}
	}
	return out, nil
}

// prependHead joins a fragment to the part of its line read so far; reading backward
// means each fragment is nearer the line's start, so capping the front keeps the head.
func prependHead(fragment, tail []byte, cut bool) ([]byte, bool) {
	if len(fragment) == 0 {
		return tail, cut
	}
	if len(fragment)+len(tail) > maxLineBytes {
		cut = true
	}
	merged := make([]byte, 0, min(len(fragment)+len(tail), maxLineBytes))
	merged = append(merged, fragment[:min(len(fragment), maxLineBytes)]...)
	merged = append(merged, tail[:min(len(tail), maxLineBytes-len(merged))]...)
	return merged, cut
}

// scanForward serves gzip, which cannot seek. Offsets count decompressed bytes and the
// budget bounds them too, so a decompression bomb costs no more than a large plain file.
func scanForward(ctx context.Context, r io.Reader, budget int64, visit lineVisitor) (scanOutcome, error) {
	br := bufio.NewReaderSize(r, maxLineBytes)
	var out scanOutcome
	skipping := false
	lastCheck := int64(0)
	for {
		if out.scanned-lastCheck >= chunkSize {
			lastCheck = out.scanned
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		if out.scanned >= budget {
			out.exhausted = true
			return out, nil
		}
		chunk, err := br.ReadSlice('\n')
		offset := out.scanned
		out.scanned += int64(len(chunk))
		full := errors.Is(err, bufio.ErrBufferFull)
		if !skipping {
			raw := bytes.TrimRight(chunk, "\r\n")
			if len(raw) > 0 && !visit(raw, offset, full) {
				return out, nil
			}
		}
		skipping = full
		if full {
			continue
		}
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}
