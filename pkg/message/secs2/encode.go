package secs2

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Encode serializes a SECS-II Item into binary format per SEMI E5.
//
// Wire format for each item:
//
//	[format_byte] [length_bytes...] [data...]
//
// format_byte: upper 6 bits = format code, lower 2 bits = number of length bytes (1-3)
// length_bytes: 1-3 bytes encoding the data length (big-endian)
// data: the raw item data
func Encode(item *Item) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("cannot encode nil item")
	}
	var buf []byte
	return encodeItem(item, buf)
}

func encodeItem(item *Item, buf []byte) ([]byte, error) {
	switch item.format {
	case FormatList:
		return encodeList(item, buf)
	case FormatASCII:
		return encodeASCII(item, buf)
	case FormatBinary:
		return encodeBinary(item, buf)
	case FormatBoolean:
		return encodeBoolean(item, buf)
	case FormatI1:
		return encodeI1(item, buf)
	case FormatI2:
		return encodeI2(item, buf)
	case FormatI4:
		return encodeI4(item, buf)
	case FormatI8:
		return encodeI8(item, buf)
	case FormatU1:
		return encodeU1(item, buf)
	case FormatU2:
		return encodeU2(item, buf)
	case FormatU4:
		return encodeU4(item, buf)
	case FormatU8:
		return encodeU8(item, buf)
	case FormatF4:
		return encodeF4(item, buf)
	case FormatF8:
		return encodeF8(item, buf)
	default:
		return nil, fmt.Errorf("unsupported format: %d", item.format)
	}
}

// encodeHeader writes the format byte + length bytes.
// dataLen is the byte length of the item's data payload.
func encodeHeader(buf []byte, format Format, dataLen int) ([]byte, error) {
	if dataLen > 0xFFFFFF {
		return nil, fmt.Errorf("data length %d exceeds maximum (16777215)", dataLen)
	}

	var numLenBytes byte
	switch {
	case dataLen <= 0xFF:
		numLenBytes = 1
	case dataLen <= 0xFFFF:
		numLenBytes = 2
	default:
		numLenBytes = 3
	}

	// Format byte: format code | number of length bytes
	formatByte := byte(format) | numLenBytes
	buf = append(buf, formatByte)

	// Length bytes (big-endian)
	switch numLenBytes {
	case 3:
		buf = append(buf, byte(dataLen>>16))
		buf = append(buf, byte(dataLen>>8))
		buf = append(buf, byte(dataLen))
	case 2:
		buf = append(buf, byte(dataLen>>8))
		buf = append(buf, byte(dataLen))
	case 1:
		buf = append(buf, byte(dataLen))
	}

	return buf, nil
}

func encodeList(item *Item, buf []byte) ([]byte, error) {
	count := len(item.values)
	var err error
	buf, err = encodeHeader(buf, FormatList, count)
	if err != nil {
		return nil, err
	}
	for i, v := range item.values {
		child, ok := v.(*Item)
		if !ok {
			return nil, fmt.Errorf("list element %d is not *Item", i)
		}
		buf, err = encodeItem(child, buf)
		if err != nil {
			return nil, fmt.Errorf("list element %d: %w", i, err)
		}
	}
	return buf, nil
}

func encodeASCII(item *Item, buf []byte) ([]byte, error) {
	var s string
	if len(item.values) > 0 {
		s = item.values[0].(string)
	}
	var err error
	buf, err = encodeHeader(buf, FormatASCII, len(s))
	if err != nil {
		return nil, err
	}
	buf = append(buf, []byte(s)...)
	return buf, nil
}

func encodeBinary(item *Item, buf []byte) ([]byte, error) {
	var data []byte
	if len(item.values) > 0 {
		data = item.values[0].([]byte)
	}
	var err error
	buf, err = encodeHeader(buf, FormatBinary, len(data))
	if err != nil {
		return nil, err
	}
	buf = append(buf, data...)
	return buf, nil
}

func encodeBoolean(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values)
	var err error
	buf, err = encodeHeader(buf, FormatBoolean, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		if v.(bool) {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	}
	return buf, nil
}

func encodeI1(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values)
	var err error
	buf, err = encodeHeader(buf, FormatI1, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = append(buf, byte(v.(int8)))
	}
	return buf, nil
}

func encodeI2(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 2
	var err error
	buf, err = encodeHeader(buf, FormatI2, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint16(buf, uint16(v.(int16)))
	}
	return buf, nil
}

func encodeI4(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 4
	var err error
	buf, err = encodeHeader(buf, FormatI4, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint32(buf, uint32(v.(int32)))
	}
	return buf, nil
}

func encodeI8(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 8
	var err error
	buf, err = encodeHeader(buf, FormatI8, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint64(buf, uint64(v.(int64)))
	}
	return buf, nil
}

func encodeU1(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values)
	var err error
	buf, err = encodeHeader(buf, FormatU1, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = append(buf, v.(uint8))
	}
	return buf, nil
}

func encodeU2(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 2
	var err error
	buf, err = encodeHeader(buf, FormatU2, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint16(buf, v.(uint16))
	}
	return buf, nil
}

func encodeU4(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 4
	var err error
	buf, err = encodeHeader(buf, FormatU4, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint32(buf, v.(uint32))
	}
	return buf, nil
}

func encodeU8(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 8
	var err error
	buf, err = encodeHeader(buf, FormatU8, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		buf = binary.BigEndian.AppendUint64(buf, v.(uint64))
	}
	return buf, nil
}

func encodeF4(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 4
	var err error
	buf, err = encodeHeader(buf, FormatF4, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		bits := math.Float32bits(v.(float32))
		buf = binary.BigEndian.AppendUint32(buf, bits)
	}
	return buf, nil
}

func encodeF8(item *Item, buf []byte) ([]byte, error) {
	dataLen := len(item.values) * 8
	var err error
	buf, err = encodeHeader(buf, FormatF8, dataLen)
	if err != nil {
		return nil, err
	}
	for _, v := range item.values {
		bits := math.Float64bits(v.(float64))
		buf = binary.BigEndian.AppendUint64(buf, bits)
	}
	return buf, nil
}
