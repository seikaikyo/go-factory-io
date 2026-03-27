package secs2

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Decode deserializes SECS-II binary data into an Item tree.
func Decode(data []byte) (*Item, error) {
	item, n, err := decodeItem(data, 0)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, fmt.Errorf("trailing data: decoded %d bytes, total %d", n, len(data))
	}
	return item, nil
}

// DecodeMulti decodes multiple items from a byte slice, returning all items found.
func DecodeMulti(data []byte) ([]*Item, error) {
	var items []*Item
	offset := 0
	for offset < len(data) {
		item, n, err := decodeItem(data, offset)
		if err != nil {
			return nil, fmt.Errorf("at offset %d: %w", offset, err)
		}
		items = append(items, item)
		offset = n
	}
	return items, nil
}

// decodeItem decodes one item starting at offset, returns the item and the new offset.
func decodeItem(data []byte, offset int) (*Item, int, error) {
	if offset >= len(data) {
		return nil, offset, fmt.Errorf("unexpected end of data at offset %d", offset)
	}

	// Parse format byte
	formatByte := data[offset]
	format := Format(formatByte & formatMask)
	numLenBytes := int(formatByte & lengthBitsMask)
	offset++

	if numLenBytes == 0 {
		return nil, offset, fmt.Errorf("invalid number of length bytes (0) at offset %d", offset-1)
	}

	// Parse length bytes
	if offset+numLenBytes > len(data) {
		return nil, offset, fmt.Errorf("not enough data for length bytes at offset %d", offset)
	}

	var dataLen int
	switch numLenBytes {
	case 1:
		dataLen = int(data[offset])
	case 2:
		dataLen = int(data[offset])<<8 | int(data[offset+1])
	case 3:
		dataLen = int(data[offset])<<16 | int(data[offset+1])<<8 | int(data[offset+2])
	}
	offset += numLenBytes

	// Decode based on format
	switch format {
	case FormatList:
		return decodeList(data, offset, dataLen)
	case FormatASCII:
		return decodeASCII(data, offset, dataLen)
	case FormatBinary:
		return decodeBinary(data, offset, dataLen)
	case FormatBoolean:
		return decodeBoolean(data, offset, dataLen)
	case FormatI1:
		return decodeI1(data, offset, dataLen)
	case FormatI2:
		return decodeI2(data, offset, dataLen)
	case FormatI4:
		return decodeI4(data, offset, dataLen)
	case FormatI8:
		return decodeI8(data, offset, dataLen)
	case FormatU1:
		return decodeU1(data, offset, dataLen)
	case FormatU2:
		return decodeU2(data, offset, dataLen)
	case FormatU4:
		return decodeU4(data, offset, dataLen)
	case FormatU8:
		return decodeU8(data, offset, dataLen)
	case FormatF4:
		return decodeF4(data, offset, dataLen)
	case FormatF8:
		return decodeF8(data, offset, dataLen)
	default:
		return nil, offset, fmt.Errorf("unsupported format 0x%02x at offset %d", byte(format), offset)
	}
}

func decodeList(data []byte, offset, count int) (*Item, int, error) {
	items := make([]interface{}, count)
	var err error
	for i := range count {
		var child *Item
		child, offset, err = decodeItem(data, offset)
		if err != nil {
			return nil, offset, fmt.Errorf("list element %d: %w", i, err)
		}
		items[i] = child
	}
	return &Item{format: FormatList, values: items}, offset, nil
}

func decodeASCII(data []byte, offset, dataLen int) (*Item, int, error) {
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for ASCII: need %d, have %d", dataLen, len(data)-offset)
	}
	s := string(data[offset : offset+dataLen])
	offset += dataLen
	item := &Item{format: FormatASCII}
	if dataLen > 0 {
		item.values = []interface{}{s}
	}
	return item, offset, nil
}

func decodeBinary(data []byte, offset, dataLen int) (*Item, int, error) {
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for Binary: need %d, have %d", dataLen, len(data)-offset)
	}
	b := make([]byte, dataLen)
	copy(b, data[offset:offset+dataLen])
	offset += dataLen
	item := &Item{format: FormatBinary}
	if dataLen > 0 {
		item.values = []interface{}{b}
	}
	return item, offset, nil
}

func decodeBoolean(data []byte, offset, dataLen int) (*Item, int, error) {
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for Boolean")
	}
	vals := make([]interface{}, dataLen)
	for i := range dataLen {
		vals[i] = data[offset+i] != 0
	}
	offset += dataLen
	return &Item{format: FormatBoolean, values: vals}, offset, nil
}

func decodeI1(data []byte, offset, dataLen int) (*Item, int, error) {
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for I1")
	}
	vals := make([]interface{}, dataLen)
	for i := range dataLen {
		vals[i] = int8(data[offset+i])
	}
	offset += dataLen
	return &Item{format: FormatI1, values: vals}, offset, nil
}

func decodeI2(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%2 != 0 {
		return nil, offset, fmt.Errorf("I2 data length %d is not a multiple of 2", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for I2")
	}
	count := dataLen / 2
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = int16(binary.BigEndian.Uint16(data[offset+i*2:]))
	}
	offset += dataLen
	return &Item{format: FormatI2, values: vals}, offset, nil
}

func decodeI4(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%4 != 0 {
		return nil, offset, fmt.Errorf("I4 data length %d is not a multiple of 4", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for I4")
	}
	count := dataLen / 4
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = int32(binary.BigEndian.Uint32(data[offset+i*4:]))
	}
	offset += dataLen
	return &Item{format: FormatI4, values: vals}, offset, nil
}

func decodeI8(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%8 != 0 {
		return nil, offset, fmt.Errorf("I8 data length %d is not a multiple of 8", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for I8")
	}
	count := dataLen / 8
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = int64(binary.BigEndian.Uint64(data[offset+i*8:]))
	}
	offset += dataLen
	return &Item{format: FormatI8, values: vals}, offset, nil
}

func decodeU1(data []byte, offset, dataLen int) (*Item, int, error) {
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for U1")
	}
	vals := make([]interface{}, dataLen)
	for i := range dataLen {
		vals[i] = data[offset+i]
	}
	offset += dataLen
	return &Item{format: FormatU1, values: vals}, offset, nil
}

func decodeU2(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%2 != 0 {
		return nil, offset, fmt.Errorf("U2 data length %d is not a multiple of 2", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for U2")
	}
	count := dataLen / 2
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = binary.BigEndian.Uint16(data[offset+i*2:])
	}
	offset += dataLen
	return &Item{format: FormatU2, values: vals}, offset, nil
}

func decodeU4(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%4 != 0 {
		return nil, offset, fmt.Errorf("U4 data length %d is not a multiple of 4", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for U4")
	}
	count := dataLen / 4
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = binary.BigEndian.Uint32(data[offset+i*4:])
	}
	offset += dataLen
	return &Item{format: FormatU4, values: vals}, offset, nil
}

func decodeU8(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%8 != 0 {
		return nil, offset, fmt.Errorf("U8 data length %d is not a multiple of 8", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for U8")
	}
	count := dataLen / 8
	vals := make([]interface{}, count)
	for i := range count {
		vals[i] = binary.BigEndian.Uint64(data[offset+i*8:])
	}
	offset += dataLen
	return &Item{format: FormatU8, values: vals}, offset, nil
}

func decodeF4(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%4 != 0 {
		return nil, offset, fmt.Errorf("F4 data length %d is not a multiple of 4", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for F4")
	}
	count := dataLen / 4
	vals := make([]interface{}, count)
	for i := range count {
		bits := binary.BigEndian.Uint32(data[offset+i*4:])
		vals[i] = math.Float32frombits(bits)
	}
	offset += dataLen
	return &Item{format: FormatF4, values: vals}, offset, nil
}

func decodeF8(data []byte, offset, dataLen int) (*Item, int, error) {
	if dataLen%8 != 0 {
		return nil, offset, fmt.Errorf("F8 data length %d is not a multiple of 8", dataLen)
	}
	if offset+dataLen > len(data) {
		return nil, offset, fmt.Errorf("not enough data for F8")
	}
	count := dataLen / 8
	vals := make([]interface{}, count)
	for i := range count {
		bits := binary.BigEndian.Uint64(data[offset+i*8:])
		vals[i] = math.Float64frombits(bits)
	}
	offset += dataLen
	return &Item{format: FormatF8, values: vals}, offset, nil
}
