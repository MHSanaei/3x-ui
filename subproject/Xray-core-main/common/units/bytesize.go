package units

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

var (
	errInvalidSize = errors.New("invalid size")
	errInvalidUnit = errors.New("invalid or unsupported unit")
)

// ByteSize is the size of bytes
type ByteSize uint64

const (
	_ = iota
	// KB = 1KB
	KB ByteSize = 1 << (10 * iota)
	// MB = 1MB
	MB
	// GB = 1GB
	GB
	// TB = 1TB
	TB
	// PB = 1PB
	PB
	// EB = 1EB
	EB
)

func (b ByteSize) String() string {
	unit := ""
	value := float64(0)
	switch {
	case b == 0:
		return "0"
	case b < KB:
		unit = "B"
		value = float64(b)
	case b < MB:
		unit = "KB"
		value = float64(b) / float64(KB)
	case b < GB:
		unit = "MB"
		value = float64(b) / float64(MB)
	case b < TB:
		unit = "GB"
		value = float64(b) / float64(GB)
	case b < PB:
		unit = "TB"
		value = float64(b) / float64(TB)
	case b < EB:
		unit = "PB"
		value = float64(b) / float64(PB)
	default:
		unit = "EB"
		value = float64(b) / float64(EB)
	}
	result := strconv.FormatFloat(value, 'f', 2, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// Parse parses ByteSize from string
func (b *ByteSize) Parse(s string) error {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	i := strings.IndexFunc(s, unicode.IsLetter)
	if i == -1 {
		return errInvalidUnit
	}

	bytesString, multiple := s[:i], s[i:]
	bytes, err := strconv.ParseFloat(bytesString, 64)
	if err != nil || bytes <= 0 {
		return errInvalidSize
	}
	switch multiple {
	case "B":
		*b = ByteSize(bytes)
	case "K", "KB", "KIB":
		*b = ByteSize(bytes * float64(KB))
	case "M", "MB", "MIB":
		*b = ByteSize(bytes * float64(MB))
	case "G", "GB", "GIB":
		*b = ByteSize(bytes * float64(GB))
	case "T", "TB", "TIB":
		*b = ByteSize(bytes * float64(TB))
	case "P", "PB", "PIB":
		*b = ByteSize(bytes * float64(PB))
	case "E", "EB", "EIB":
		*b = ByteSize(bytes * float64(EB))
	default:
		return errInvalidUnit
	}
	return nil
}
