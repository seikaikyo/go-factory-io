package secs2

import (
	"runtime"
	"strings"
	"testing"
)

// nestedLists returns the encoding of depth nested lists: depth-1 levels of
// L[1] around an innermost L[0].
func nestedLists(depth int) []byte {
	data := make([]byte, 0, depth*2)
	for i := 0; i < depth-1; i++ {
		data = append(data, 0x01, 0x01)
	}
	return append(data, 0x01, 0x00)
}

// TestDecodeNestingDepthLimit checks that nesting deeper than the limit is
// refused with an error. Without a limit the recursion depth is bounded only
// by the message size, and a deep enough message overflows the goroutine
// stack, which is a fatal error recover cannot catch. The test stays far
// below that point: it only needs to step one level past the limit.
func TestDecodeNestingDepthLimit(t *testing.T) {
	const limit = 64

	if _, err := Decode(nestedLists(limit)); err != nil {
		t.Fatalf("nesting at the limit (%d) should decode: %v", limit, err)
	}

	for _, depth := range []int{limit + 1, 1000} {
		_, err := Decode(nestedLists(depth))
		if err == nil {
			t.Fatalf("nesting depth %d decoded; want an error above %d", depth, limit)
		}
		if !strings.Contains(err.Error(), "depth") {
			t.Errorf("depth %d: error should name the depth limit, got: %v", depth, err)
		}
	}

	// DecodeMulti goes through the same path.
	if _, err := DecodeMulti(nestedLists(limit + 1)); err == nil {
		t.Fatal("DecodeMulti accepted nesting above the limit")
	}
}

// TestDecodeListCountExceedsData checks that a list header declaring more
// children than the remaining bytes can hold is rejected before the child
// slice is allocated. Every child needs at least two bytes (format byte and
// one length byte).
func TestDecodeListCountExceedsData(t *testing.T) {
	// L with a 3-byte length field declaring 16,777,215 children, no data.
	data := []byte{0x03, 0xFF, 0xFF, 0xFF}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := Decode(data)
	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("expected an error for a list count larger than the data")
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 1<<20 {
		t.Errorf("decoder allocated %d bytes for a 4-byte message; the count must be checked before allocating", allocated)
	}

	// Two declared children but bytes for only one.
	if _, err := Decode([]byte{0x01, 0x02, 0xA5, 0x00}); err == nil {
		t.Fatal("expected an error when the data holds fewer children than declared")
	}
	// A count that fits still decodes.
	item, err := Decode([]byte{0x01, 0x02, 0xA5, 0x00, 0xA5, 0x00})
	if err != nil {
		t.Fatalf("two empty U1 children: %v", err)
	}
	if item.Len() != 2 {
		t.Errorf("decoded %d children, want 2", item.Len())
	}
}
