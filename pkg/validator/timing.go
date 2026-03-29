package validator

import (
	"fmt"
	"sync"
	"time"
)

// TimingStatus indicates the result of a timing check.
type TimingStatus int

const (
	TimingOK       TimingStatus = iota // Within limits
	TimingWarning                      // > 80% of limit
	TimingViolation                    // Exceeded limit
)

func (s TimingStatus) String() string {
	switch s {
	case TimingOK:
		return "OK"
	case TimingWarning:
		return "WARNING"
	case TimingViolation:
		return "VIOLATION"
	default:
		return "?"
	}
}

// TimingConfig holds timeout limits (from HSMS T3-T8).
type TimingConfig struct {
	T3 time.Duration // Reply timeout (default 45s)
	T5 time.Duration // Connect separation (default 10s)
	T6 time.Duration // Control transaction (default 5s)
	T7 time.Duration // Not-selected (default 10s)
	T8 time.Duration // Network intercharacter (default 5s)
}

// DefaultTimingConfig returns SEMI E37 recommended defaults.
func DefaultTimingConfig() TimingConfig {
	return TimingConfig{
		T3: 45 * time.Second,
		T5: 10 * time.Second,
		T6: 5 * time.Second,
		T7: 10 * time.Second,
		T8: 5 * time.Second,
	}
}

// PendingTransaction tracks an outgoing request awaiting reply.
type PendingTransaction struct {
	SystemID  uint32
	Stream    byte
	Function  byte
	SentAt    time.Time
	IsControl bool // true for Select/Deselect/Linktest
}

// TransactionTiming records a completed request/reply pair.
type TransactionTiming struct {
	SystemID     uint32        `json:"systemId"`
	Stream       byte          `json:"stream"`
	Function     byte          `json:"function"`
	SentAt       time.Time     `json:"sentAt"`
	ReplyAt      time.Time     `json:"replyAt"`
	Duration     time.Duration `json:"duration"`
	TimeoutName  string        `json:"timeoutName"`
	TimeoutLimit time.Duration `json:"timeoutLimit"`
	Status       TimingStatus  `json:"status"`
}

// TimingTracker records message timestamps for timing validation.
type TimingTracker struct {
	mu        sync.Mutex
	pending   map[uint32]*PendingTransaction
	completed []TransactionTiming
	config    TimingConfig
}

// NewTimingTracker creates a tracker with the given timing config.
func NewTimingTracker(cfg TimingConfig) *TimingTracker {
	return &TimingTracker{
		pending: make(map[uint32]*PendingTransaction),
		config:  cfg,
	}
}

// OnSend records an outgoing message with W-bit set (expecting reply).
func (tt *TimingTracker) OnSend(systemID uint32, stream, function byte, wBit, isControl bool) {
	if !wBit {
		return // no reply expected
	}
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.pending[systemID] = &PendingTransaction{
		SystemID:  systemID,
		Stream:    stream,
		Function:  function,
		SentAt:    time.Now(),
		IsControl: isControl,
	}
}

// OnReceive records an incoming reply message and returns the timing result.
func (tt *TimingTracker) OnReceive(systemID uint32) *TransactionTiming {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	pend, ok := tt.pending[systemID]
	if !ok {
		return nil // no matching pending request
	}
	delete(tt.pending, systemID)

	now := time.Now()
	duration := now.Sub(pend.SentAt)

	// Determine which timeout applies
	timeoutName := "T3"
	limit := tt.config.T3
	if pend.IsControl {
		timeoutName = "T6"
		limit = tt.config.T6
	}

	status := TimingOK
	if duration > limit {
		status = TimingViolation
	} else if float64(duration) > float64(limit)*0.8 {
		status = TimingWarning
	}

	result := TransactionTiming{
		SystemID:     systemID,
		Stream:       pend.Stream,
		Function:     pend.Function,
		SentAt:       pend.SentAt,
		ReplyAt:      now,
		Duration:     duration,
		TimeoutName:  timeoutName,
		TimeoutLimit: limit,
		Status:       status,
	}
	tt.completed = append(tt.completed, result)
	return &result
}

// CheckPending returns violations for any pending requests that have timed out.
func (tt *TimingTracker) CheckPending() []TransactionTiming {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	now := time.Now()
	var timedOut []TransactionTiming
	for _, pend := range tt.pending {
		limit := tt.config.T3
		timeoutName := "T3"
		if pend.IsControl {
			limit = tt.config.T6
			timeoutName = "T6"
		}
		elapsed := now.Sub(pend.SentAt)
		if elapsed > limit {
			timedOut = append(timedOut, TransactionTiming{
				SystemID:     pend.SystemID,
				Stream:       pend.Stream,
				Function:     pend.Function,
				SentAt:       pend.SentAt,
				Duration:     elapsed,
				TimeoutName:  timeoutName,
				TimeoutLimit: limit,
				Status:       TimingViolation,
			})
		}
	}
	return timedOut
}

// Completed returns all completed transaction timings.
func (tt *TimingTracker) Completed() []TransactionTiming {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	out := make([]TransactionTiming, len(tt.completed))
	copy(out, tt.completed)
	return out
}

// PendingCount returns the number of pending transactions.
func (tt *TimingTracker) PendingCount() int {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	return len(tt.pending)
}

// CheckSystemByteUniqueness verifies no duplicate system bytes in completed transactions.
func (tt *TimingTracker) CheckSystemByteUniqueness() []ValidationResult {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	seen := make(map[uint32]int) // systemID -> count
	for _, t := range tt.completed {
		seen[t.SystemID]++
	}

	var results []ValidationResult
	hasDuplicate := false
	for id, count := range seen {
		if count > 1 {
			hasDuplicate = true
			results = append(results, ValidationResult{
				Level:   LevelFail,
				Message: fmt.Sprintf("duplicate system byte: 0x%08X used %d times", id, count),
			})
		}
	}
	if !hasDuplicate {
		results = append(results, ValidationResult{
			Level:   LevelPass,
			Message: "system byte uniqueness: no duplicates",
		})
	}
	return results
}

// Reset clears all tracking data.
func (tt *TimingTracker) Reset() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.pending = make(map[uint32]*PendingTransaction)
	tt.completed = nil
}
