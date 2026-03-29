package validator

import (
	"testing"
	"time"
)

func TestTimingTracker_NormalReply(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())
	tt.OnSend(1, 1, 13, true, false)

	time.Sleep(5 * time.Millisecond)
	result := tt.OnReceive(1)

	if result == nil {
		t.Fatal("expected timing result")
	}
	if result.Status != TimingOK {
		t.Errorf("expected OK, got %s", result.Status)
	}
	if result.TimeoutName != "T3" {
		t.Errorf("expected T3, got %s", result.TimeoutName)
	}
	if result.Duration < 5*time.Millisecond {
		t.Errorf("duration too short: %v", result.Duration)
	}
}

func TestTimingTracker_ControlMessage(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())
	tt.OnSend(2, 0, 0, true, true) // control message

	result := tt.OnReceive(2)
	if result == nil {
		t.Fatal("expected timing result")
	}
	if result.TimeoutName != "T6" {
		t.Errorf("expected T6 for control message, got %s", result.TimeoutName)
	}
}

func TestTimingTracker_NoWBit(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())
	tt.OnSend(3, 1, 14, false, false) // no W-bit

	if tt.PendingCount() != 0 {
		t.Error("should not track non-W-bit messages")
	}
}

func TestTimingTracker_UnmatchedReply(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())
	result := tt.OnReceive(99) // no matching pending
	if result != nil {
		t.Error("expected nil for unmatched reply")
	}
}

func TestTimingTracker_Warning(t *testing.T) {
	cfg := TimingConfig{T3: 10 * time.Millisecond} // very short for testing
	tt := NewTimingTracker(cfg)
	tt.OnSend(1, 1, 1, true, false)

	time.Sleep(9 * time.Millisecond) // > 80% of 10ms
	result := tt.OnReceive(1)

	if result == nil {
		t.Fatal("expected timing result")
	}
	if result.Status != TimingWarning && result.Status != TimingViolation {
		t.Errorf("expected WARNING or VIOLATION at 90%% of limit, got %s (duration=%v, limit=%v)",
			result.Status, result.Duration, result.TimeoutLimit)
	}
}

func TestTimingTracker_CheckPending(t *testing.T) {
	cfg := TimingConfig{T3: 5 * time.Millisecond}
	tt := NewTimingTracker(cfg)
	tt.OnSend(1, 1, 13, true, false)

	time.Sleep(10 * time.Millisecond) // exceed T3
	timedOut := tt.CheckPending()

	if len(timedOut) != 1 {
		t.Fatalf("expected 1 timed out, got %d", len(timedOut))
	}
	if timedOut[0].Status != TimingViolation {
		t.Errorf("expected VIOLATION, got %s", timedOut[0].Status)
	}
}

func TestTimingTracker_SystemByteUniqueness(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())

	// Two different system bytes
	tt.OnSend(1, 1, 1, true, false)
	tt.OnReceive(1)
	tt.OnSend(2, 1, 1, true, false)
	tt.OnReceive(2)

	results := tt.CheckSystemByteUniqueness()
	if MaxLevel(results) != LevelPass {
		t.Error("unique system bytes should pass")
	}
}

func TestTimingTracker_DuplicateSystemByte(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())

	// Same system byte used twice
	tt.OnSend(1, 1, 1, true, false)
	tt.OnReceive(1)
	tt.OnSend(1, 1, 3, true, false) // reuse system byte 1
	tt.OnReceive(1)

	results := tt.CheckSystemByteUniqueness()
	if MaxLevel(results) != LevelFail {
		t.Error("duplicate system bytes should fail")
	}
}

func TestTimingTracker_Reset(t *testing.T) {
	tt := NewTimingTracker(DefaultTimingConfig())
	tt.OnSend(1, 1, 1, true, false)
	tt.OnReceive(1)

	tt.Reset()
	if tt.PendingCount() != 0 {
		t.Error("reset should clear pending")
	}
	if len(tt.Completed()) != 0 {
		t.Error("reset should clear completed")
	}
}
