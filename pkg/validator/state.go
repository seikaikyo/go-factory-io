package validator

import (
	"fmt"
	"sync"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
)

// StateViolation records an invalid state transition.
type StateViolation struct {
	Timestamp time.Time `json:"timestamp"`
	Machine   string    `json:"machine"`   // "CommState", "ControlState", "CarrierState", "ProcessJobState", "ControlJobState"
	EntityID  string    `json:"entityId"`  // "" for comm/control, carrierID or jobID otherwise
	FromState string    `json:"fromState"`
	ToState   string    `json:"toState"`
	Message   string    `json:"message"`
}

// StateTransitionRecord logs an observed transition.
type StateTransitionRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Machine   string    `json:"machine"`
	EntityID  string    `json:"entityId"`
	FromState string    `json:"fromState"`
	ToState   string    `json:"toState"`
	Valid     bool      `json:"valid"`
}

// StateValidator observes state transitions and reports violations.
type StateValidator struct {
	mu          sync.Mutex
	transitions []StateTransitionRecord
	violations  []StateViolation
}

// NewStateValidator creates a new validator.
func NewStateValidator() *StateValidator {
	return &StateValidator{}
}

// Transitions returns all recorded transitions.
func (sv *StateValidator) Transitions() []StateTransitionRecord {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	out := make([]StateTransitionRecord, len(sv.transitions))
	copy(out, sv.transitions)
	return out
}

// Violations returns all recorded violations.
func (sv *StateValidator) Violations() []StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	out := make([]StateViolation, len(sv.violations))
	copy(out, sv.violations)
	return out
}

// Reset clears all recorded data.
func (sv *StateValidator) Reset() {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.transitions = nil
	sv.violations = nil
}

// --- Communication State (E30 Figure 2) ---

// Valid transitions: DISABLED->WAIT_CRA, WAIT_CRA->COMMUNICATING,
// WAIT_CRA->WAIT_DELAY, WAIT_DELAY->WAIT_CRA, any->DISABLED
var validCommTransitions = map[[2]gem.CommState]bool{
	{gem.CommDisabled, gem.CommWaitCRA}:          true,
	{gem.CommWaitCRA, gem.CommCommunicating}:     true,
	{gem.CommWaitCRA, gem.CommWaitDelay}:         true,
	{gem.CommWaitDelay, gem.CommWaitCRA}:         true,
	// DisableComm can go from any state to DISABLED
	{gem.CommWaitCRA, gem.CommDisabled}:          true,
	{gem.CommWaitDelay, gem.CommDisabled}:        true,
	{gem.CommCommunicating, gem.CommDisabled}:    true,
}

// ValidateCommTransition checks if a communication state transition is valid.
func (sv *StateValidator) ValidateCommTransition(from, to gem.CommState) *StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	valid := validCommTransitions[[2]gem.CommState{from, to}]
	sv.transitions = append(sv.transitions, StateTransitionRecord{
		Timestamp: time.Now(),
		Machine:   "CommState",
		FromState: from.String(),
		ToState:   to.String(),
		Valid:     valid,
	})

	if !valid {
		v := &StateViolation{
			Timestamp: time.Now(),
			Machine:   "CommState",
			FromState: from.String(),
			ToState:   to.String(),
			Message:   fmt.Sprintf("E30: invalid comm transition %s -> %s", from, to),
		}
		sv.violations = append(sv.violations, *v)
		return v
	}
	return nil
}

// --- Control State (E30 Figure 3) ---

// Valid transitions per E30 control state diagram
var validControlTransitions = map[[2]gem.ControlState]bool{
	{gem.ControlOfflineEquipment, gem.ControlOfflineHost}: true,
	{gem.ControlOfflineHost, gem.ControlOnlineLocal}:      true,
	{gem.ControlOfflineHost, gem.ControlOnlineRemote}:     true,
	{gem.ControlOnlineLocal, gem.ControlOnlineRemote}:     true,
	{gem.ControlOnlineRemote, gem.ControlOnlineLocal}:     true,
	// GoOffline from any online state
	{gem.ControlOnlineLocal, gem.ControlOfflineEquipment}:  true,
	{gem.ControlOnlineRemote, gem.ControlOfflineEquipment}: true,
	// Host can also go offline
	{gem.ControlOfflineHost, gem.ControlOfflineEquipment}: true,
}

// ValidateControlTransition checks if a control state transition is valid.
func (sv *StateValidator) ValidateControlTransition(from, to gem.ControlState) *StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	valid := validControlTransitions[[2]gem.ControlState{from, to}]
	sv.transitions = append(sv.transitions, StateTransitionRecord{
		Timestamp: time.Now(),
		Machine:   "ControlState",
		FromState: from.String(),
		ToState:   to.String(),
		Valid:     valid,
	})

	if !valid {
		v := &StateViolation{
			Timestamp: time.Now(),
			Machine:   "ControlState",
			FromState: from.String(),
			ToState:   to.String(),
			Message:   fmt.Sprintf("E30: invalid control transition %s -> %s", from, to),
		}
		sv.violations = append(sv.violations, *v)
		return v
	}
	return nil
}

// --- Carrier State (E87) ---

var validCarrierTransitions = map[[2]gem.CarrierState]bool{
	{gem.CarrierNotAccessed, gem.CarrierWaitingForHost}:     true,
	{gem.CarrierWaitingForHost, gem.CarrierInAccess}:        true,
	{gem.CarrierInAccess, gem.CarrierCarrierComplete}:       true,
	{gem.CarrierInAccess, gem.CarrierStopped}:               true,
	{gem.CarrierCarrierComplete, gem.CarrierReadyToUnload}:  true,
	{gem.CarrierStopped, gem.CarrierReadyToUnload}:          true,
	{gem.CarrierStopped, gem.CarrierInAccess}:               true, // resume
	{gem.CarrierWaitingForHost, gem.CarrierReadyToUnload}:   true, // cancel
	{gem.CarrierNotAccessed, gem.CarrierReadyToUnload}:      true, // reject
}

// ValidateCarrierTransition checks if a carrier state transition is valid.
func (sv *StateValidator) ValidateCarrierTransition(carrierID string, from, to gem.CarrierState) *StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	valid := validCarrierTransitions[[2]gem.CarrierState{from, to}]
	sv.transitions = append(sv.transitions, StateTransitionRecord{
		Timestamp: time.Now(),
		Machine:   "CarrierState",
		EntityID:  carrierID,
		FromState: from.String(),
		ToState:   to.String(),
		Valid:     valid,
	})

	if !valid {
		v := &StateViolation{
			Timestamp: time.Now(),
			Machine:   "CarrierState",
			EntityID:  carrierID,
			FromState: from.String(),
			ToState:   to.String(),
			Message:   fmt.Sprintf("E87: carrier %s invalid transition %s -> %s", carrierID, from, to),
		}
		sv.violations = append(sv.violations, *v)
		return v
	}
	return nil
}

// --- Process Job State (E40) ---

var validPJTransitions = map[[2]gem.ProcessJobState]bool{
	{gem.PJQueued, gem.PJSettingUp}:           true,
	{gem.PJSettingUp, gem.PJWaitingForStart}:  true,
	{gem.PJWaitingForStart, gem.PJProcessing}: true,
	{gem.PJProcessing, gem.PJProcessComplete}: true,
	{gem.PJProcessing, gem.PJStopping}:        true,
	{gem.PJStopping, gem.PJStopped}:           true,
	{gem.PJStopped, gem.PJSettingUp}:          true, // restart
	// Abort from any active state
	{gem.PJQueued, gem.PJAborting}:           true,
	{gem.PJSettingUp, gem.PJAborting}:        true,
	{gem.PJWaitingForStart, gem.PJAborting}:  true,
	{gem.PJProcessing, gem.PJAborting}:       true,
	{gem.PJStopping, gem.PJAborting}:         true,
	{gem.PJStopped, gem.PJAborting}:          true,
	{gem.PJAborting, gem.PJAborted}:          true,
}

// ValidatePJTransition checks if a process job state transition is valid.
func (sv *StateValidator) ValidatePJTransition(jobID string, from, to gem.ProcessJobState) *StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	valid := validPJTransitions[[2]gem.ProcessJobState{from, to}]
	sv.transitions = append(sv.transitions, StateTransitionRecord{
		Timestamp: time.Now(),
		Machine:   "ProcessJobState",
		EntityID:  jobID,
		FromState: from.String(),
		ToState:   to.String(),
		Valid:     valid,
	})

	if !valid {
		v := &StateViolation{
			Timestamp: time.Now(),
			Machine:   "ProcessJobState",
			EntityID:  jobID,
			FromState: from.String(),
			ToState:   to.String(),
			Message:   fmt.Sprintf("E40: job %s invalid transition %s -> %s", jobID, from, to),
		}
		sv.violations = append(sv.violations, *v)
		return v
	}
	return nil
}

// --- Control Job State (E94) ---

var validCJTransitions = map[[2]gem.ControlJobState]bool{
	{gem.CJQueued, gem.CJSelected}:            true,
	{gem.CJSelected, gem.CJWaitingForStart}:   true,
	{gem.CJWaitingForStart, gem.CJExecuting}:  true,
	{gem.CJExecuting, gem.CJCompleted}:        true,
	{gem.CJExecuting, gem.CJPausing}:          true,
	{gem.CJPausing, gem.CJPaused}:             true,
	{gem.CJPaused, gem.CJExecuting}:           true, // resume
	{gem.CJExecuting, gem.CJStopping}:         true,
	{gem.CJPausing, gem.CJStopping}:           true,
	{gem.CJPaused, gem.CJStopping}:            true,
	{gem.CJStopping, gem.CJStopped}:           true,
	{gem.CJQueued, gem.CJStopping}:            true,
	{gem.CJSelected, gem.CJStopping}:          true,
	{gem.CJWaitingForStart, gem.CJStopping}:   true,
}

// ValidateCJTransition checks if a control job state transition is valid.
func (sv *StateValidator) ValidateCJTransition(jobID string, from, to gem.ControlJobState) *StateViolation {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	valid := validCJTransitions[[2]gem.ControlJobState{from, to}]
	sv.transitions = append(sv.transitions, StateTransitionRecord{
		Timestamp: time.Now(),
		Machine:   "ControlJobState",
		EntityID:  jobID,
		FromState: from.String(),
		ToState:   to.String(),
		Valid:     valid,
	})

	if !valid {
		v := &StateViolation{
			Timestamp: time.Now(),
			Machine:   "ControlJobState",
			EntityID:  jobID,
			FromState: from.String(),
			ToState:   to.String(),
			Message:   fmt.Sprintf("E94: job %s invalid transition %s -> %s", jobID, from, to),
		}
		sv.violations = append(sv.violations, *v)
		return v
	}
	return nil
}
