package gem

import (
	"fmt"
	"sync"
	"time"
)

// --- E116 Equipment Performance Tracking ---

// EPTState represents the equipment performance state per SEMI E116.
type EPTState int

const (
	EPTIdle                    EPTState = iota // No work, available
	EPTBusy                                     // Actively processing
	EPTBlocked                                  // Cannot output (downstream full)
	EPTBusyAndBlocked                           // Processing but output blocked
	EPTStandbyScheduled                         // Scheduled maintenance/idle
	EPTStandbyUnscheduled                       // Unscheduled idle (e.g., no material)
	EPTEngScheduled                             // Scheduled engineering time
	EPTEngUnscheduled                           // Unscheduled engineering
	EPTDownScheduled                            // Scheduled maintenance downtime
	EPTDownUnscheduled                          // Unplanned downtime (breakdown)
	EPTNonScheduled                             // Not scheduled for production
)

func (s EPTState) String() string {
	switch s {
	case EPTIdle:
		return "IDLE"
	case EPTBusy:
		return "BUSY"
	case EPTBlocked:
		return "BLOCKED"
	case EPTBusyAndBlocked:
		return "BUSY_AND_BLOCKED"
	case EPTStandbyScheduled:
		return "STANDBY_SCHEDULED"
	case EPTStandbyUnscheduled:
		return "STANDBY_UNSCHEDULED"
	case EPTEngScheduled:
		return "ENGINEERING_SCHEDULED"
	case EPTEngUnscheduled:
		return "ENGINEERING_UNSCHEDULED"
	case EPTDownScheduled:
		return "DOWN_SCHEDULED"
	case EPTDownUnscheduled:
		return "DOWN_UNSCHEDULED"
	case EPTNonScheduled:
		return "NON_SCHEDULED"
	default:
		return "UNKNOWN"
	}
}

// isProductive returns true if the state counts as productive time.
func (s EPTState) isProductive() bool {
	return s == EPTBusy || s == EPTBusyAndBlocked
}

// isAvailable returns true if the equipment is available for production.
func (s EPTState) isAvailable() bool {
	return s == EPTIdle || s == EPTBusy || s == EPTBlocked || s == EPTBusyAndBlocked
}

// EPTTransition records a state change with timestamp.
type EPTTransition struct {
	Time  time.Time
	From  EPTState
	To    EPTState
}

// EPTTracker tracks equipment performance states and computes OEE metrics per SEMI E116.
type EPTTracker struct {
	mu          sync.RWMutex
	state       EPTState
	stateStart  time.Time          // When current state began
	durations   map[EPTState]time.Duration // Accumulated time per state
	transitions []EPTTransition    // State change log
	unitCount   int64              // Total units processed
	defectCount int64              // Total defective units
	onChange    func(from, to EPTState)
}

// NewEPTTracker creates an E116 equipment performance tracker.
func NewEPTTracker() *EPTTracker {
	return &EPTTracker{
		state:      EPTNonScheduled,
		stateStart: time.Now(),
		durations:  make(map[EPTState]time.Duration),
	}
}

// OnStateChange sets a callback for EPT state transitions.
func (t *EPTTracker) OnStateChange(fn func(from, to EPTState)) {
	t.mu.Lock()
	t.onChange = fn
	t.mu.Unlock()
}

// SetState transitions to a new EPT state.
func (t *EPTTracker) SetState(newState EPTState) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if newState == t.state {
		return nil // No change
	}

	now := time.Now()
	elapsed := now.Sub(t.stateStart)
	t.durations[t.state] += elapsed

	oldState := t.state
	t.transitions = append(t.transitions, EPTTransition{
		Time: now,
		From: oldState,
		To:   newState,
	})

	t.state = newState
	t.stateStart = now

	if t.onChange != nil {
		t.onChange(oldState, newState)
	}
	return nil
}

// State returns the current EPT state.
func (t *EPTTracker) State() EPTState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// RecordUnit records a processed unit (for quality rate calculation).
func (t *EPTTracker) RecordUnit(defective bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.unitCount++
	if defective {
		t.defectCount++
	}
}

// OEE computes Overall Equipment Effectiveness per SEMI E116.
// Returns availability, performance, quality, and overall OEE (all 0.0-1.0).
func (t *EPTTracker) OEE() (availability, performance, quality, oee float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Flush current state duration
	now := time.Now()
	durations := make(map[EPTState]time.Duration)
	for k, v := range t.durations {
		durations[k] = v
	}
	durations[t.state] += now.Sub(t.stateStart)

	// Calculate totals
	var totalTime, availableTime, productiveTime time.Duration
	for state, d := range durations {
		totalTime += d
		if state.isAvailable() {
			availableTime += d
		}
		if state.isProductive() {
			productiveTime += d
		}
	}

	// Exclude NonScheduled from total
	scheduledTime := totalTime - durations[EPTNonScheduled]

	if scheduledTime <= 0 {
		return 0, 0, 0, 0
	}

	// Availability = available time / scheduled time
	availability = float64(availableTime) / float64(scheduledTime)

	// Performance = productive time / available time
	if availableTime > 0 {
		performance = float64(productiveTime) / float64(availableTime)
	}

	// Quality = good units / total units
	if t.unitCount > 0 {
		quality = float64(t.unitCount-t.defectCount) / float64(t.unitCount)
	} else {
		quality = 1.0 // No units = no defects
	}

	oee = availability * performance * quality
	return
}

// StateDurations returns accumulated time per state.
func (t *EPTTracker) StateDurations() map[EPTState]time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[EPTState]time.Duration)
	for k, v := range t.durations {
		result[k] = v
	}
	// Include current state's ongoing duration
	result[t.state] += time.Since(t.stateStart)
	return result
}

// Transitions returns the state change log.
func (t *EPTTracker) Transitions() []EPTTransition {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]EPTTransition, len(t.transitions))
	copy(result, t.transitions)
	return result
}

// UnitCounts returns total and defective unit counts.
func (t *EPTTracker) UnitCounts() (total, defective int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.unitCount, t.defectCount
}

// Reset clears all accumulated data and resets to NonScheduled.
func (t *EPTTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = EPTNonScheduled
	t.stateStart = time.Now()
	t.durations = make(map[EPTState]time.Duration)
	t.transitions = nil
	t.unitCount = 0
	t.defectCount = 0
}

// Summary returns a human-readable performance summary.
func (t *EPTTracker) Summary() string {
	a, p, q, oee := t.OEE()
	total, defective := t.UnitCounts()
	return fmt.Sprintf("OEE=%.1f%% (A=%.1f%% P=%.1f%% Q=%.1f%%) units=%d defects=%d",
		oee*100, a*100, p*100, q*100, total, defective)
}
