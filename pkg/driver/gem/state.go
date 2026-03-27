// Package gem implements the GEM (Generic Equipment Model) per SEMI E30.
// GEM defines the standard behavior for semiconductor equipment communication.
package gem

import (
	"fmt"
	"sync"
)

// --- Communication State Model (SEMI E30 Figure 2) ---

// CommState represents the GEM communication state.
type CommState int

const (
	CommDisabled         CommState = iota // Communication disabled
	CommWaitCRA                           // Wait for Communication Request Acknowledge
	CommWaitDelay                         // Wait for retry delay
	CommCommunicating                     // Actively communicating
)

func (s CommState) String() string {
	switch s {
	case CommDisabled:
		return "DISABLED"
	case CommWaitCRA:
		return "WAIT_CRA"
	case CommWaitDelay:
		return "WAIT_DELAY"
	case CommCommunicating:
		return "COMMUNICATING"
	default:
		return "UNKNOWN"
	}
}

// --- Control State Model (SEMI E30 Figure 3) ---

// ControlState represents the GEM equipment control state.
type ControlState int

const (
	ControlOfflineEquipment ControlState = iota // Equipment offline (init state)
	ControlOfflineHost                          // Attempting to go online
	ControlOnlineLocal                          // Online, local control
	ControlOnlineRemote                         // Online, remote control
)

func (s ControlState) String() string {
	switch s {
	case ControlOfflineEquipment:
		return "OFFLINE/EQUIPMENT"
	case ControlOfflineHost:
		return "OFFLINE/HOST"
	case ControlOnlineLocal:
		return "ONLINE/LOCAL"
	case ControlOnlineRemote:
		return "ONLINE/REMOTE"
	default:
		return "UNKNOWN"
	}
}

// --- State Machine ---

// StateMachine manages GEM communication and control states.
type StateMachine struct {
	mu           sync.RWMutex
	commState    CommState
	controlState ControlState
	onChange     func(comm CommState, control ControlState)
}

// NewStateMachine creates a GEM state machine in the initial state.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		commState:    CommDisabled,
		controlState: ControlOfflineEquipment,
	}
}

// OnChange sets a callback for state changes.
func (sm *StateMachine) OnChange(fn func(comm CommState, control ControlState)) {
	sm.mu.Lock()
	sm.onChange = fn
	sm.mu.Unlock()
}

// CommState returns the current communication state.
func (sm *StateMachine) CommState() CommState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.commState
}

// ControlState returns the current control state.
func (sm *StateMachine) ControlState() ControlState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.controlState
}

func (sm *StateMachine) notifyChange() {
	if sm.onChange != nil {
		sm.onChange(sm.commState, sm.controlState)
	}
}

// --- Communication State Transitions ---

// EnableComm transitions from DISABLED to WAIT_CRA.
func (sm *StateMachine) EnableComm() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommDisabled {
		return fmt.Errorf("gem: cannot enable comm in state %s", sm.commState)
	}
	sm.commState = CommWaitCRA
	sm.notifyChange()
	return nil
}

// DisableComm transitions to DISABLED from any state.
func (sm *StateMachine) DisableComm() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.commState = CommDisabled
	sm.controlState = ControlOfflineEquipment
	sm.notifyChange()
}

// CommEstablished transitions from WAIT_CRA to COMMUNICATING.
func (sm *StateMachine) CommEstablished() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommWaitCRA {
		return fmt.Errorf("gem: cannot establish comm in state %s", sm.commState)
	}
	sm.commState = CommCommunicating
	sm.notifyChange()
	return nil
}

// CommFailed transitions from WAIT_CRA to WAIT_DELAY.
func (sm *StateMachine) CommFailed() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommWaitCRA {
		return fmt.Errorf("gem: comm failure not valid in state %s", sm.commState)
	}
	sm.commState = CommWaitDelay
	sm.notifyChange()
	return nil
}

// RetryComm transitions from WAIT_DELAY to WAIT_CRA.
func (sm *StateMachine) RetryComm() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommWaitDelay {
		return fmt.Errorf("gem: cannot retry in state %s", sm.commState)
	}
	sm.commState = CommWaitCRA
	sm.notifyChange()
	return nil
}

// --- Control State Transitions ---

// GoOnlineLocal transitions to ONLINE/LOCAL.
func (sm *StateMachine) GoOnlineLocal() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommCommunicating {
		return fmt.Errorf("gem: must be communicating to go online (state=%s)", sm.commState)
	}
	sm.controlState = ControlOnlineLocal
	sm.notifyChange()
	return nil
}

// GoOnlineRemote transitions to ONLINE/REMOTE.
func (sm *StateMachine) GoOnlineRemote() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.commState != CommCommunicating {
		return fmt.Errorf("gem: must be communicating to go online (state=%s)", sm.commState)
	}
	sm.controlState = ControlOnlineRemote
	sm.notifyChange()
	return nil
}

// GoOffline transitions to OFFLINE/EQUIPMENT.
func (sm *StateMachine) GoOffline() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.controlState != ControlOnlineLocal && sm.controlState != ControlOnlineRemote {
		return fmt.Errorf("gem: cannot go offline from state %s", sm.controlState)
	}
	sm.controlState = ControlOfflineEquipment
	sm.notifyChange()
	return nil
}

// IsOnline returns true if the equipment is in any online state.
func (sm *StateMachine) IsOnline() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.controlState == ControlOnlineLocal || sm.controlState == ControlOnlineRemote
}

// IsCommunicating returns true if communication is established.
func (sm *StateMachine) IsCommunicating() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.commState == CommCommunicating
}
