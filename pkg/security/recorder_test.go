package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestMessageRecorder(t *testing.T) {
	var buf bytes.Buffer
	rec := NewMessageRecorder(&buf, 256)

	rec.Record(DirInbound, 1, 1, 1, 100, []byte{0x01, 0x02, 0x03})
	rec.Record(DirOutbound, 1, 2, 1, 100, []byte{0x41, 0x42})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), output)
	}

	if !strings.Contains(lines[0], "IN S1F1") {
		t.Errorf("line 0: %q, want IN S1F1", lines[0])
	}
	if !strings.Contains(lines[0], "010203") {
		t.Errorf("line 0 missing hex: %q", lines[0])
	}
	if !strings.Contains(lines[1], "OUT S1F2") {
		t.Errorf("line 1: %q, want OUT S1F2", lines[1])
	}
}

func TestMessageRecorderDisabled(t *testing.T) {
	var buf bytes.Buffer
	rec := NewMessageRecorder(&buf, 256)
	rec.SetEnabled(false)

	rec.Record(DirInbound, 1, 1, 1, 100, []byte{0x01})

	if buf.Len() != 0 {
		t.Error("disabled recorder should not write")
	}
}

func TestMessageRecorderTruncate(t *testing.T) {
	var buf bytes.Buffer
	rec := NewMessageRecorder(&buf, 4) // Only 4 bytes max

	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}
	rec.Record(DirInbound, 6, 11, 1, 1, data)

	output := buf.String()
	// Should contain hex of first 4 bytes only: 00010203
	if !strings.Contains(output, "00010203") {
		t.Errorf("missing truncated hex in: %q", output)
	}
	// Should show full length
	if !strings.Contains(output, "len=100") {
		t.Errorf("missing full length in: %q", output)
	}
}

func TestMessageRecordString(t *testing.T) {
	rec := MessageRecord{
		Direction: DirInbound,
		Stream:    6,
		Function:  11,
		SessionID: 1,
		SystemID:  42,
		DataLen:   10,
		RawHex:    "deadbeef",
	}
	s := rec.String()
	if !strings.Contains(s, "IN S6F11") {
		t.Errorf("String: %q", s)
	}
}
