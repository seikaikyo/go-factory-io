package gem

import (
	"fmt"
	"sync"
)

// AlarmSeverity represents the alarm severity level.
type AlarmSeverity byte

const (
	AlarmPersonalSafety AlarmSeverity = 1 << 7 // Bit 7: personal safety
	AlarmEquipSafety    AlarmSeverity = 1 << 6 // Bit 6: equipment safety
	AlarmParameter      AlarmSeverity = 1 << 5 // Bit 5: parameter control warning
	AlarmProcess        AlarmSeverity = 1 << 4 // Bit 4: process error (wafer/die)
	AlarmEquipStatus    AlarmSeverity = 1 << 3 // Bit 3: equipment status warning
	AlarmAttention      AlarmSeverity = 1 << 2 // Bit 2: attention flags
	AlarmReserved1      AlarmSeverity = 1 << 1 // Bit 1: reserved
	AlarmReserved0      AlarmSeverity = 1       // Bit 0: reserved
)

// AlarmState tracks whether an alarm is currently set or cleared.
type AlarmState int

const (
	AlarmCleared AlarmState = iota
	AlarmSet
)

func (s AlarmState) String() string {
	if s == AlarmSet {
		return "SET"
	}
	return "CLEARED"
}

// Alarm represents a GEM alarm definition.
type Alarm struct {
	ALID     uint32
	Name     string
	Text     string         // Alarm text description
	Severity AlarmSeverity
	State    AlarmState
	Enabled  bool           // Whether alarm reporting is enabled
	CEID     uint32         // Collection event triggered on alarm set (0 = none)
	ClearCEID uint32        // Collection event triggered on alarm clear (0 = none)
}

// AlarmManager manages equipment alarms per SEMI E30.
type AlarmManager struct {
	mu     sync.RWMutex
	alarms map[uint32]*Alarm
}

// NewAlarmManager creates an empty alarm manager.
func NewAlarmManager() *AlarmManager {
	return &AlarmManager{
		alarms: make(map[uint32]*Alarm),
	}
}

// DefineAlarm registers a new alarm.
func (am *AlarmManager) DefineAlarm(alarm *Alarm) {
	am.mu.Lock()
	defer am.mu.Unlock()
	if alarm.Enabled == false {
		alarm.Enabled = true // Default enabled
	}
	am.alarms[alarm.ALID] = alarm
}

// GetAlarm returns an alarm by ID.
func (am *AlarmManager) GetAlarm(alid uint32) (*Alarm, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	a, ok := am.alarms[alid]
	return a, ok
}

// SetAlarm sets an alarm to active state. Returns the alarm for S5F1 reporting.
func (am *AlarmManager) SetAlarm(alid uint32) (*Alarm, error) {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.alarms[alid]
	if !ok {
		return nil, fmt.Errorf("gem: unknown ALID %d", alid)
	}
	a.State = AlarmSet
	return a, nil
}

// ClearAlarm clears an alarm. Returns the alarm for S5F1 reporting.
func (am *AlarmManager) ClearAlarm(alid uint32) (*Alarm, error) {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.alarms[alid]
	if !ok {
		return nil, fmt.Errorf("gem: unknown ALID %d", alid)
	}
	a.State = AlarmCleared
	return a, nil
}

// EnableAlarm enables or disables alarm reporting (S5F3 handler).
func (am *AlarmManager) EnableAlarm(alid uint32, enabled bool) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.alarms[alid]
	if !ok {
		return fmt.Errorf("gem: unknown ALID %d", alid)
	}
	a.Enabled = enabled
	return nil
}

// EnableAllAlarms enables or disables all alarms.
func (am *AlarmManager) EnableAllAlarms(enabled bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	for _, a := range am.alarms {
		a.Enabled = enabled
	}
}

// ListAlarms returns all registered alarms.
func (am *AlarmManager) ListAlarms() []*Alarm {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]*Alarm, 0, len(am.alarms))
	for _, a := range am.alarms {
		result = append(result, a)
	}
	return result
}

// ListActiveAlarms returns alarms currently in SET state.
func (am *AlarmManager) ListActiveAlarms() []*Alarm {
	am.mu.RLock()
	defer am.mu.RUnlock()
	var result []*Alarm
	for _, a := range am.alarms {
		if a.State == AlarmSet {
			result = append(result, a)
		}
	}
	return result
}
