package gem

import (
	"log/slog"
	"sync"
)

// AlarmAction defines what happens when a safety-critical alarm is triggered.
// Per SEMI S2: "equipment activities potentially impacted by the presence of
// an alarm shall be inhibited."
type AlarmAction int

const (
	AlarmActionLogOnly     AlarmAction = iota // Log the alarm, take no automatic action
	AlarmActionForceIdle                      // Pause current processing
	AlarmActionForceOffline                   // Transition to OFFLINE state
)

func (a AlarmAction) String() string {
	switch a {
	case AlarmActionLogOnly:
		return "LOG_ONLY"
	case AlarmActionForceIdle:
		return "FORCE_IDLE"
	case AlarmActionForceOffline:
		return "FORCE_OFFLINE"
	default:
		return "UNKNOWN"
	}
}

// SafetyInterlock connects the alarm system to the equipment state machine.
// When a high-severity alarm fires, it can automatically transition the
// equipment to a safe state per SEMI S2 requirements.
type SafetyInterlock struct {
	mu     sync.RWMutex
	logger *slog.Logger
	state  *StateMachine

	// Severity threshold: alarms at or above this severity trigger the action.
	// Default: AlarmEquipSafety (bit 6)
	Threshold AlarmSeverity

	// Action to take when threshold is met.
	Action AlarmAction

	// OnSafetyAlarm: optional callback for custom handling.
	OnSafetyAlarm func(alarm *Alarm, action AlarmAction)
}

// NewSafetyInterlock creates a safety interlock with default settings.
// Default: alarms >= Equipment Safety trigger ForceOffline.
func NewSafetyInterlock(state *StateMachine, logger *slog.Logger) *SafetyInterlock {
	if logger == nil {
		logger = slog.Default()
	}
	return &SafetyInterlock{
		logger:    logger,
		state:     state,
		Threshold: AlarmEquipSafety, // Bit 6: equipment safety
		Action:    AlarmActionForceOffline,
	}
}

// Evaluate checks an alarm against the safety interlock rules.
// Called automatically when an alarm is set.
// Returns true if a safety action was triggered.
func (si *SafetyInterlock) Evaluate(alarm *Alarm) bool {
	si.mu.RLock()
	threshold := si.Threshold
	action := si.Action
	callback := si.OnSafetyAlarm
	si.mu.RUnlock()

	if alarm.State != AlarmSet {
		return false
	}

	// Check if alarm severity meets or exceeds threshold
	if alarm.Severity&threshold == 0 {
		// Also check higher severity bits
		higherBits := byte(threshold)
		alarmBits := byte(alarm.Severity)
		// Any bit at or above the threshold position triggers
		if alarmBits < higherBits {
			return false
		}
	}

	si.logger.Warn("Safety interlock triggered",
		"alid", alarm.ALID,
		"name", alarm.Name,
		"severity", alarm.Severity,
		"action", action,
	)

	// Execute the configured action
	switch action {
	case AlarmActionForceOffline:
		if si.state.IsOnline() {
			if err := si.state.GoOffline(); err != nil {
				si.logger.Error("Safety interlock: failed to go offline", "error", err)
			} else {
				si.logger.Warn("Safety interlock: equipment forced OFFLINE")
			}
		}
	case AlarmActionForceIdle:
		si.logger.Warn("Safety interlock: equipment should IDLE (application must implement)")
	case AlarmActionLogOnly:
		// Already logged above
	}

	// Fire callback
	if callback != nil {
		callback(alarm, action)
	}

	return true
}

// SetThreshold changes the alarm severity threshold.
func (si *SafetyInterlock) SetThreshold(threshold AlarmSeverity) {
	si.mu.Lock()
	si.Threshold = threshold
	si.mu.Unlock()
}

// SetAction changes the automatic action.
func (si *SafetyInterlock) SetAction(action AlarmAction) {
	si.mu.Lock()
	si.Action = action
	si.mu.Unlock()
}
