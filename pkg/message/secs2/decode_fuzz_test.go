package secs2

import "testing"

// FuzzDecode feeds arbitrary bytes to the decoder. It must never panic, and
// any item it accepts must encode again and decode back without error.
func FuzzDecode(f *testing.F) {
	seeds := []*Item{
		NewList(),
		NewASCII("hello"),
		NewBinary([]byte{0x00, 0x80}),
		NewBoolean(true, false),
		NewI1(-1), NewI2(-2), NewI4(-4), NewI8(-8),
		NewU1(1), NewU2(2), NewU4(4), NewU8(8),
		NewF4(1.5), NewF8(2.5),
		NewList(NewU4(1001), NewList(NewASCII("A"), NewList(NewU4()))),
	}
	for _, it := range seeds {
		data, err := Encode(it)
		if err != nil {
			f.Fatalf("seed encode: %v", err)
		}
		f.Add(data)
	}
	f.Add(nestedLists(maxDecodeDepth))
	f.Add(nestedLists(maxDecodeDepth + 1))
	f.Add([]byte{0x03, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		item, err := Decode(data)
		if err != nil {
			return
		}
		out, err := Encode(item)
		if err != nil {
			t.Fatalf("decoded item does not encode: %v", err)
		}
		if _, err := Decode(out); err != nil {
			t.Fatalf("re-encoded item does not decode: %v", err)
		}
		_ = item.String()
	})
}
