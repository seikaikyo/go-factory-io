package secs2

import (
	"bytes"
	"math"
	"testing"
)

// --- Round-trip tests: encode then decode, verify equality ---

func TestRoundTripASCII(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"hello", "Hello SECS-II"},
		{"long", string(make([]byte, 300))}, // >255 bytes, needs 2-byte length
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := NewASCII(tt.input)
			data, err := Encode(item)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := Decode(data)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			got, err := decoded.ToASCII()
			if err != nil {
				t.Fatalf("toASCII: %v", err)
			}
			if got != tt.input {
				t.Errorf("got %q, want %q", got, tt.input)
			}
		})
	}
}

func TestRoundTripBinary(t *testing.T) {
	input := []byte{0x00, 0xFF, 0xAB, 0xCD}
	item := NewBinary(input)
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := decoded.ToBinary()
	if err != nil {
		t.Fatalf("toBinary: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("got %x, want %x", got, input)
	}
}

func TestRoundTripBoolean(t *testing.T) {
	input := []bool{true, false, true, true}
	item := NewBoolean(input...)
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := decoded.ToBooleans()
	if err != nil {
		t.Fatalf("toBooleans: %v", err)
	}
	for i, v := range got {
		if v != input[i] {
			t.Errorf("element %d: got %v, want %v", i, v, input[i])
		}
	}
}

func TestRoundTripSignedIntegers(t *testing.T) {
	t.Run("I1", func(t *testing.T) {
		item := NewI1(-128, 0, 127)
		roundTripInt(t, item, []int64{-128, 0, 127})
	})
	t.Run("I2", func(t *testing.T) {
		item := NewI2(-32768, 0, 32767)
		roundTripInt(t, item, []int64{-32768, 0, 32767})
	})
	t.Run("I4", func(t *testing.T) {
		item := NewI4(-2147483648, 0, 2147483647)
		roundTripInt(t, item, []int64{-2147483648, 0, 2147483647})
	})
	t.Run("I8", func(t *testing.T) {
		item := NewI8(math.MinInt64, 0, math.MaxInt64)
		roundTripInt(t, item, []int64{math.MinInt64, 0, math.MaxInt64})
	})
}

func TestRoundTripUnsignedIntegers(t *testing.T) {
	t.Run("U1", func(t *testing.T) {
		item := NewU1(0, 128, 255)
		roundTripUint(t, item, []uint64{0, 128, 255})
	})
	t.Run("U2", func(t *testing.T) {
		item := NewU2(0, 1000, 65535)
		roundTripUint(t, item, []uint64{0, 1000, 65535})
	})
	t.Run("U4", func(t *testing.T) {
		item := NewU4(0, 100000, 4294967295)
		roundTripUint(t, item, []uint64{0, 100000, 4294967295})
	})
	t.Run("U8", func(t *testing.T) {
		item := NewU8(0, 1000000, math.MaxUint64)
		roundTripUint(t, item, []uint64{0, 1000000, math.MaxUint64})
	})
}

func TestRoundTripFloats(t *testing.T) {
	t.Run("F4", func(t *testing.T) {
		item := NewF4(0.0, 1.5, -3.14)
		data, err := Encode(item)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		got, err := decoded.ToFloat64s()
		if err != nil {
			t.Fatalf("toFloat64s: %v", err)
		}
		want := []float64{0.0, 1.5, -3.14}
		for i, v := range got {
			if math.Abs(v-want[i]) > 0.01 {
				t.Errorf("element %d: got %f, want %f", i, v, want[i])
			}
		}
	})
	t.Run("F8", func(t *testing.T) {
		item := NewF8(0.0, math.Pi, -math.E)
		data, err := Encode(item)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		got, err := decoded.ToFloat64s()
		if err != nil {
			t.Fatalf("toFloat64s: %v", err)
		}
		want := []float64{0.0, math.Pi, -math.E}
		for i, v := range got {
			if v != want[i] {
				t.Errorf("element %d: got %f, want %f", i, v, want[i])
			}
		}
	})
}

func TestRoundTripList(t *testing.T) {
	// S1F13: Establish Communication Request
	// L,2
	//   <A "MDLN">       <- model name
	//   <A "SOFTREV">    <- software revision
	item := NewList(
		NewASCII("MDLN"),
		NewASCII("SOFTREV"),
	)

	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Format() != FormatList {
		t.Fatalf("expected List, got %s", decoded.Format())
	}
	if decoded.Len() != 2 {
		t.Fatalf("expected 2 items, got %d", decoded.Len())
	}

	mdln, _ := decoded.ItemAt(0).ToASCII()
	if mdln != "MDLN" {
		t.Errorf("MDLN: got %q, want %q", mdln, "MDLN")
	}
	softrev, _ := decoded.ItemAt(1).ToASCII()
	if softrev != "SOFTREV" {
		t.Errorf("SOFTREV: got %q, want %q", softrev, "SOFTREV")
	}
}

func TestRoundTripNestedList(t *testing.T) {
	// Nested structure like S2F33 Define Report:
	// L,2
	//   <U4 100>            <- DATAID
	//   L,1                 <- Report list
	//     L,2
	//       <U4 1>          <- RPTID
	//       L,2             <- VID list
	//         <U4 1001>
	//         <U4 1002>
	item := NewList(
		NewU4(100),
		NewList(
			NewList(
				NewU4(1),
				NewList(
					NewU4(1001),
					NewU4(1002),
				),
			),
		),
	)

	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Navigate: root -> [1] -> [0] -> [1] -> [0]
	vid, _ := decoded.ItemAt(1).ItemAt(0).ItemAt(1).ItemAt(0).ToUint64s()
	if vid[0] != 1001 {
		t.Errorf("VID: got %d, want 1001", vid[0])
	}
}

func TestEmptyList(t *testing.T) {
	item := NewList()
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Len() != 0 {
		t.Errorf("expected empty list, got %d items", decoded.Len())
	}
}

func TestEmptyASCII(t *testing.T) {
	item := NewASCII("")
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	s, _ := decoded.ToASCII()
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

// --- Encoding format verification ---

func TestEncodeHeaderFormat(t *testing.T) {
	// U1 with value 42: format byte should be 0xA5 (FormatU1=0xA4 | 1 length byte)
	item := NewU1(42)
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Expected: [0xA5, 0x01, 0x2A]
	//   0xA5 = 10100101 = FormatU1(10100100) | 1 length byte
	//   0x01 = data length 1
	//   0x2A = 42
	expected := []byte{0xA5, 0x01, 0x2A}
	if !bytes.Equal(data, expected) {
		t.Errorf("got %x, want %x", data, expected)
	}
}

func TestEncodeTwoByteLengthField(t *testing.T) {
	// ASCII string of 256 bytes needs 2-byte length
	s := string(make([]byte, 256))
	item := NewASCII(s)
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Format byte: FormatASCII(0x40) | 2 = 0x42
	if data[0] != 0x42 {
		t.Errorf("format byte: got 0x%02x, want 0x42", data[0])
	}
	// Length: 0x01, 0x00 = 256
	if data[1] != 0x01 || data[2] != 0x00 {
		t.Errorf("length bytes: got [0x%02x, 0x%02x], want [0x01, 0x00]", data[1], data[2])
	}
}

// --- Error cases ---

func TestDecodeTrailingData(t *testing.T) {
	item := NewU1(1)
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Append garbage
	data = append(data, 0xFF)
	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for trailing data")
	}
}

func TestDecodeEmptyData(t *testing.T) {
	_, err := Decode([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestDecodeTruncated(t *testing.T) {
	// U4 header says 4 bytes but only 2 present
	data := []byte{0xB1, 0x04, 0x00, 0x01}
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for truncated data")
	}
}

func TestEncodeNilItem(t *testing.T) {
	_, err := Encode(nil)
	if err == nil {
		t.Fatal("expected error for nil item")
	}
}

// --- String representation ---

func TestItemString(t *testing.T) {
	item := NewList(
		NewASCII("Equipment1"),
		NewU4(42),
	)
	s := item.String()
	if len(s) == 0 {
		t.Fatal("expected non-empty string")
	}
	t.Log(s) // Visual check
}

// --- Codec adapter ---

func TestCodecInterface(t *testing.T) {
	codec := NewCodec()
	item := NewList(NewASCII("test"), NewU4(123))

	data, err := codec.Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := codec.Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	result := decoded.(*Item)
	if result.Format() != FormatList || result.Len() != 2 {
		t.Errorf("unexpected result: format=%s, len=%d", result.Format(), result.Len())
	}
}

func TestCodecInvalidBody(t *testing.T) {
	codec := NewCodec()
	_, err := codec.Encode("not an item")
	if err != ErrInvalidBody {
		t.Errorf("expected ErrInvalidBody, got %v", err)
	}
}

// --- Helpers ---

func roundTripInt(t *testing.T, item *Item, want []int64) {
	t.Helper()
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := decoded.ToInt64s()
	if err != nil {
		t.Fatalf("toInt64s: %v", err)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("element %d: got %d, want %d", i, v, want[i])
		}
	}
}

func roundTripUint(t *testing.T, item *Item, want []uint64) {
	t.Helper()
	data, err := Encode(item)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := decoded.ToUint64s()
	if err != nil {
		t.Fatalf("toUint64s: %v", err)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("element %d: got %d, want %d", i, v, want[i])
		}
	}
}

// --- Benchmark ---

func BenchmarkEncodeS6F11(b *testing.B) {
	// Simulate S6F11 Event Report with 10 variables
	vars := make([]*Item, 10)
	for i := range 10 {
		vars[i] = NewU4(uint32(i * 100))
	}
	item := NewList(
		NewU4(1),            // DATAID
		NewU4(1001),         // CEID
		NewList(             // RPT list
			NewList(         // RPT 1
				NewU4(1),    // RPTID
				NewList(vars...), // Variables
			),
		),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_, _ = Encode(item)
	}
}

func BenchmarkDecodeS6F11(b *testing.B) {
	vars := make([]*Item, 10)
	for i := range 10 {
		vars[i] = NewU4(uint32(i * 100))
	}
	item := NewList(
		NewU4(1),
		NewU4(1001),
		NewList(
			NewList(
				NewU4(1),
				NewList(vars...),
			),
		),
	)
	data, _ := Encode(item)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_, _ = Decode(data)
	}
}
