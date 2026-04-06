package buf

import (
	"io"

	"github.com/xtls/xray-core/common/bytespool"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

const (
	// Size of a regular buffer.
	Size = 8192
)

var ErrBufferFull = errors.New("buffer is full")

var pool = bytespool.GetPool(Size)

// ownership represents the data owner of the buffer.
type ownership uint8

const (
	managed ownership = iota
	unmanaged
	bytespools
)

// Buffer is a recyclable allocation of a byte array. Buffer.Release() recycles
// the buffer into an internal buffer pool, in order to recreate a buffer more
// quickly.
type Buffer struct {
	v         []byte
	start     int32
	end       int32
	ownership ownership
	UDP       *net.Destination
}

// New creates a Buffer with 0 length and 8K capacity, managed.
func New() *Buffer {
	buf := pool.Get().([]byte)
	if cap(buf) >= Size {
		buf = buf[:Size]
	} else {
		buf = make([]byte, Size)
	}

	return &Buffer{
		v: buf,
	}
}

// NewExisted creates a standard size Buffer with an existed bytearray, managed.
func NewExisted(b []byte) *Buffer {
	if cap(b) < Size {
		panic("Invalid buffer")
	}

	oLen := len(b)
	if oLen < Size {
		b = b[:Size]
	}

	return &Buffer{
		v:   b,
		end: int32(oLen),
	}
}

// FromBytes creates a Buffer with an existed bytearray, unmanaged.
func FromBytes(b []byte) *Buffer {
	return &Buffer{
		v:         b,
		end:       int32(len(b)),
		ownership: unmanaged,
	}
}

// StackNew creates a new Buffer object on stack, managed.
// This method is for buffers that is released in the same function.
func StackNew() Buffer {
	buf := pool.Get().([]byte)
	if cap(buf) >= Size {
		buf = buf[:Size]
	} else {
		buf = make([]byte, Size)
	}

	return Buffer{
		v: buf,
	}
}

// NewWithSize creates a Buffer with 0 length and capacity with at least the given size, bytespool's.
func NewWithSize(size int32) *Buffer {
	return &Buffer{
		v:         bytespool.Alloc(size),
		ownership: bytespools,
	}
}

// Release recycles the buffer into an internal buffer pool.
func (b *Buffer) Release() {
	if b == nil || b.v == nil || b.ownership == unmanaged {
		return
	}

	p := b.v
	b.v = nil
	b.Clear()

	switch b.ownership {
	case managed:
		if cap(p) == Size {
			pool.Put(p)
		}
	case bytespools:
		bytespool.Free(p)
	}
	b.UDP = nil
}

// Clear clears the content of the buffer, results an empty buffer with
// Len() = 0.
func (b *Buffer) Clear() {
	b.start = 0
	b.end = 0
}

// Byte returns the bytes at index.
func (b *Buffer) Byte(index int32) byte {
	return b.v[b.start+index]
}

// SetByte sets the byte value at index.
func (b *Buffer) SetByte(index int32, value byte) {
	b.v[b.start+index] = value
}

// Bytes returns the content bytes of this Buffer.
func (b *Buffer) Bytes() []byte {
	return b.v[b.start:b.end]
}

// Extend increases the buffer size by n bytes, and returns the extended part.
// It panics if result size is larger than size of this buffer.
func (b *Buffer) Extend(n int32) []byte {
	end := b.end + n
	if end > int32(len(b.v)) {
		panic("extending out of bound")
	}
	ext := b.v[b.end:end]
	b.end = end
	clear(ext)
	return ext
}

// BytesRange returns a slice of this buffer with given from and to boundary.
func (b *Buffer) BytesRange(from, to int32) []byte {
	if from < 0 {
		from += b.Len()
	}
	if to < 0 {
		to += b.Len()
	}
	return b.v[b.start+from : b.start+to]
}

// BytesFrom returns a slice of this Buffer starting from the given position.
func (b *Buffer) BytesFrom(from int32) []byte {
	if from < 0 {
		from += b.Len()
	}
	return b.v[b.start+from : b.end]
}

// BytesTo returns a slice of this Buffer from start to the given position.
func (b *Buffer) BytesTo(to int32) []byte {
	if to < 0 {
		to += b.Len()
	}
	if to < 0 {
		to = 0
	}
	return b.v[b.start : b.start+to]
}

// Check makes sure that 0 <= b.start <= b.end.
func (b *Buffer) Check() {
	if b.start < 0 {
		b.start = 0
	}
	if b.end < 0 {
		b.end = 0
	}
	if b.start > b.end {
		b.start = b.end
	}
}

// Resize cuts the buffer at the given position.
func (b *Buffer) Resize(from, to int32) {
	oldEnd := b.end
	if from < 0 {
		from += b.Len()
	}
	if to < 0 {
		to += b.Len()
	}
	if to < from {
		panic("Invalid slice")
	}
	b.end = b.start + to
	b.start += from
	b.Check()
	if b.end > oldEnd {
		clear(b.v[oldEnd:b.end])
	}
}

// Advance cuts the buffer at the given position.
func (b *Buffer) Advance(from int32) {
	if from < 0 {
		from += b.Len()
	}
	b.start += from
	b.Check()
}

// Len returns the length of the buffer content.
func (b *Buffer) Len() int32 {
	if b == nil {
		return 0
	}
	return b.end - b.start
}

// Cap returns the capacity of the buffer content.
func (b *Buffer) Cap() int32 {
	if b == nil {
		return 0
	}
	return int32(len(b.v))
}

// Available returns the available capacity of the buffer content.
func (b *Buffer) Available() int32 {
	if b == nil {
		return 0
	}
	return int32(len(b.v)) - b.end
}

// IsEmpty returns true if the buffer is empty.
func (b *Buffer) IsEmpty() bool {
	return b.Len() == 0
}

// IsFull returns true if the buffer has no more room to grow.
func (b *Buffer) IsFull() bool {
	return b != nil && b.end == int32(len(b.v))
}

// Write implements Write method in io.Writer.
func (b *Buffer) Write(data []byte) (int, error) {
	nBytes := copy(b.v[b.end:], data)
	b.end += int32(nBytes)
	if nBytes < len(data) {
		return nBytes, ErrBufferFull
	}
	return nBytes, nil
}

// WriteByte writes a single byte into the buffer.
func (b *Buffer) WriteByte(v byte) error {
	if b.IsFull() {
		return ErrBufferFull
	}
	b.v[b.end] = v
	b.end++
	return nil
}

// WriteString implements io.StringWriter.
func (b *Buffer) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

// ReadByte implements io.ByteReader
func (b *Buffer) ReadByte() (byte, error) {
	if b.start == b.end {
		return 0, io.EOF
	}

	nb := b.v[b.start]
	b.start++
	return nb, nil
}

// ReadBytes implements bufio.Reader.ReadBytes
func (b *Buffer) ReadBytes(length int32) ([]byte, error) {
	if b.end-b.start < length {
		return nil, io.EOF
	}

	nb := b.v[b.start : b.start+length]
	b.start += length
	return nb, nil
}

// Read implements io.Reader.Read().
func (b *Buffer) Read(data []byte) (int, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}
	nBytes := copy(data, b.v[b.start:b.end])
	if int32(nBytes) == b.Len() {
		b.Clear()
	} else {
		b.start += int32(nBytes)
	}
	return nBytes, nil
}

// ReadFrom implements io.ReaderFrom.
func (b *Buffer) ReadFrom(reader io.Reader) (int64, error) {
	n, err := reader.Read(b.v[b.end:])
	b.end += int32(n)
	return int64(n), err
}

// ReadFullFrom reads exact size of bytes from given reader, or until error occurs.
func (b *Buffer) ReadFullFrom(reader io.Reader, size int32) (int64, error) {
	end := b.end + size
	if end > int32(len(b.v)) {
		v := end
		return 0, errors.New("out of bound: ", v)
	}
	n, err := io.ReadFull(reader, b.v[b.end:end])
	b.end += int32(n)
	return int64(n), err
}

// String returns the string form of this Buffer.
func (b *Buffer) String() string {
	return string(b.Bytes())
}
