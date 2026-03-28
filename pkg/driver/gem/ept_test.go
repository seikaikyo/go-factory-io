package gem

import (
	"testing"
	"time"
)

func TestEPTStateTransitions(t *testing.T) {
	tracker := NewEPTTracker()

	if tracker.State() != EPTNonScheduled {
		t.Errorf("initial state = %s, want NON_SCHEDULED", tracker.State())
	}

	tracker.SetState(EPTIdle)
	if tracker.State() != EPTIdle {
		t.Errorf("state = %s, want IDLE", tracker.State())
	}

	tracker.SetState(EPTBusy)
	tracker.SetState(EPTIdle)

	transitions := tracker.Transitions()
	if len(transitions) != 3 {
		t.Errorf("transitions = %d, want 3", len(transitions))
	}
	if transitions[0].From != EPTNonScheduled || transitions[0].To != EPTIdle {
		t.Errorf("first transition = %s->%s, want NON_SCHEDULED->IDLE", transitions[0].From, transitions[0].To)
	}
}

func TestEPTSameStateNoOp(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.SetState(EPTIdle)
	tracker.SetState(EPTIdle) // Same state, should not add transition

	if len(tracker.Transitions()) != 1 {
		t.Errorf("transitions = %d, want 1 (no-op for same state)", len(tracker.Transitions()))
	}
}

func TestEPTUnitCounting(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.RecordUnit(false)
	tracker.RecordUnit(false)
	tracker.RecordUnit(true) // defect

	total, defects := tracker.UnitCounts()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if defects != 1 {
		t.Errorf("defects = %d, want 1", defects)
	}
}

func TestEPTOEECalculation(t *testing.T) {
	tracker := NewEPTTracker()

	// Simulate: scheduled, then idle, then busy
	tracker.SetState(EPTIdle)
	time.Sleep(10 * time.Millisecond)
	tracker.SetState(EPTBusy)
	time.Sleep(20 * time.Millisecond)
	tracker.SetState(EPTIdle)

	// Record units
	tracker.RecordUnit(false) // good
	tracker.RecordUnit(false) // good
	tracker.RecordUnit(true)  // defect

	a, p, q, oee := tracker.OEE()

	// Availability should be ~1.0 (all time in available states)
	if a < 0.9 {
		t.Errorf("availability = %.2f, want >= 0.9", a)
	}

	// Performance should be > 0 (some busy time)
	if p <= 0 {
		t.Errorf("performance = %.2f, want > 0", p)
	}

	// Quality = 2/3 = 0.667
	if q < 0.6 || q > 0.7 {
		t.Errorf("quality = %.2f, want ~0.667", q)
	}

	// OEE should be > 0
	if oee <= 0 {
		t.Errorf("oee = %.2f, want > 0", oee)
	}
}

func TestEPTOEENoUnits(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.SetState(EPTIdle)
	time.Sleep(5 * time.Millisecond)

	_, _, q, _ := tracker.OEE()
	if q != 1.0 {
		t.Errorf("quality = %.2f, want 1.0 (no units = no defects)", q)
	}
}

func TestEPTOEENoScheduledTime(t *testing.T) {
	tracker := NewEPTTracker()
	// Only NonScheduled time
	a, p, q, oee := tracker.OEE()
	if a != 0 || p != 0 || q != 0 || oee != 0 {
		t.Errorf("OEE should be 0 with no scheduled time, got a=%.2f p=%.2f q=%.2f oee=%.2f", a, p, q, oee)
	}
}

func TestEPTStateDurations(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.SetState(EPTIdle)
	time.Sleep(10 * time.Millisecond)
	tracker.SetState(EPTBusy)
	time.Sleep(10 * time.Millisecond)

	durations := tracker.StateDurations()
	if d, ok := durations[EPTIdle]; !ok || d < 5*time.Millisecond {
		t.Errorf("idle duration = %v, want >= 5ms", d)
	}
}

func TestEPTReset(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.SetState(EPTBusy)
	tracker.RecordUnit(false)
	tracker.Reset()

	if tracker.State() != EPTNonScheduled {
		t.Errorf("state after reset = %s, want NON_SCHEDULED", tracker.State())
	}
	total, _ := tracker.UnitCounts()
	if total != 0 {
		t.Errorf("units after reset = %d, want 0", total)
	}
	if len(tracker.Transitions()) != 0 {
		t.Errorf("transitions after reset = %d, want 0", len(tracker.Transitions()))
	}
}

func TestEPTCallback(t *testing.T) {
	tracker := NewEPTTracker()
	var gotFrom, gotTo EPTState
	tracker.OnStateChange(func(from, to EPTState) {
		gotFrom = from
		gotTo = to
	})

	tracker.SetState(EPTBusy)
	if gotFrom != EPTNonScheduled || gotTo != EPTBusy {
		t.Errorf("callback: %s->%s, want NON_SCHEDULED->BUSY", gotFrom, gotTo)
	}
}

func TestEPTSummary(t *testing.T) {
	tracker := NewEPTTracker()
	tracker.SetState(EPTBusy)
	tracker.RecordUnit(false)
	summary := tracker.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestEPTStateStrings(t *testing.T) {
	tests := []struct {
		state EPTState
		want  string
	}{
		{EPTIdle, "IDLE"},
		{EPTBusy, "BUSY"},
		{EPTDownUnscheduled, "DOWN_UNSCHEDULED"},
		{EPTState(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("EPTState(%d).String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}
