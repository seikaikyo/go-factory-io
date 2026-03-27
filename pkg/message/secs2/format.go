// Package secs2 implements SECS-II (SEMI E5) message encoding and decoding.
//
// SECS-II defines the message format used by semiconductor equipment to exchange
// structured data. Each data item has a format code and length, supporting nested
// lists for complex hierarchical structures.
package secs2

// Format represents a SECS-II item format code.
// The format code occupies the upper 6 bits of the format byte.
type Format byte

const (
	FormatList    Format = 0b000000_00 // 00: List
	FormatBinary  Format = 0b001000_00 // 08: Binary
	FormatBoolean Format = 0b001001_00 // 09: Boolean
	FormatASCII   Format = 0b010000_00 // 16: ASCII
	FormatJIS8    Format = 0b010001_00 // 17: JIS-8 (Japanese)
	FormatI8      Format = 0b011000_00 // 24: 8-byte signed integer
	FormatI1      Format = 0b011001_00 // 25: 1-byte signed integer
	FormatI2      Format = 0b011010_00 // 26: 2-byte signed integer
	FormatI4      Format = 0b011100_00 // 28: 4-byte signed integer
	FormatF8      Format = 0b100000_00 // 32: 8-byte floating point
	FormatF4      Format = 0b100100_00 // 36: 4-byte floating point
	FormatU8      Format = 0b101000_00 // 40: 8-byte unsigned integer
	FormatU1      Format = 0b101001_00 // 41: 1-byte unsigned integer
	FormatU2      Format = 0b101010_00 // 42: 2-byte unsigned integer
	FormatU4      Format = 0b101100_00 // 44: 4-byte unsigned integer
)

// formatMask extracts the format code from the format byte (upper 6 bits).
const formatMask = 0b111111_00

// lengthBitsMask extracts the number of length bytes (lower 2 bits).
const lengthBitsMask = 0b000000_11

// String returns the human-readable name of the format.
func (f Format) String() string {
	switch f {
	case FormatList:
		return "L"
	case FormatBinary:
		return "B"
	case FormatBoolean:
		return "BOOLEAN"
	case FormatASCII:
		return "A"
	case FormatJIS8:
		return "J"
	case FormatI8:
		return "I8"
	case FormatI1:
		return "I1"
	case FormatI2:
		return "I2"
	case FormatI4:
		return "I4"
	case FormatF8:
		return "F8"
	case FormatF4:
		return "F4"
	case FormatU8:
		return "U8"
	case FormatU1:
		return "U1"
	case FormatU2:
		return "U2"
	case FormatU4:
		return "U4"
	default:
		return "UNKNOWN"
	}
}

// itemSize returns the byte size of a single element for the given format.
// Returns 0 for List (variable length) and variable-length formats.
func (f Format) itemSize() int {
	switch f {
	case FormatBoolean, FormatBinary, FormatI1, FormatU1:
		return 1
	case FormatI2, FormatU2:
		return 2
	case FormatI4, FormatU4, FormatF4:
		return 4
	case FormatI8, FormatU8, FormatF8:
		return 8
	default:
		return 0 // variable length (List, ASCII, JIS8)
	}
}
