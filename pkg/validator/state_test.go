package validator

import (
	"testing"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
)

func TestCommState_ValidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.CommState{
		{gem.CommDisabled, gem.CommWaitCRA},
		{gem.CommWaitCRA, gem.CommCommunicating},
		{gem.CommWaitCRA, gem.CommWaitDelay},
		{gem.CommWaitDelay, gem.CommWaitCRA},
		{gem.CommCommunicating, gem.CommDisabled},
	}
	for _, tt := range tests {
		if v := sv.ValidateCommTransition(tt[0], tt[1]); v != nil {
			t.Errorf("expected valid: %s -> %s, got violation: %s", tt[0], tt[1], v.Message)
		}
	}
}

func TestCommState_InvalidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.CommState{
		{gem.CommDisabled, gem.CommCommunicating},  // skip WAIT_CRA
		{gem.CommCommunicating, gem.CommWaitCRA},    // backwards
		{gem.CommWaitDelay, gem.CommCommunicating},  // skip WAIT_CRA
	}
	for _, tt := range tests {
		if v := sv.ValidateCommTransition(tt[0], tt[1]); v == nil {
			t.Errorf("expected violation for %s -> %s", tt[0], tt[1])
		}
	}
}

func TestControlState_ValidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.ControlState{
		{gem.ControlOfflineEquipment, gem.ControlOfflineHost},
		{gem.ControlOfflineHost, gem.ControlOnlineRemote},
		{gem.ControlOnlineRemote, gem.ControlOnlineLocal},
		{gem.ControlOnlineLocal, gem.ControlOfflineEquipment},
	}
	for _, tt := range tests {
		if v := sv.ValidateControlTransition(tt[0], tt[1]); v != nil {
			t.Errorf("expected valid: %s -> %s, got violation: %s", tt[0], tt[1], v.Message)
		}
	}
}

func TestControlState_InvalidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.ControlState{
		{gem.ControlOfflineEquipment, gem.ControlOnlineRemote}, // skip OFFLINE/HOST
		{gem.ControlOfflineEquipment, gem.ControlOnlineLocal},  // skip OFFLINE/HOST
	}
	for _, tt := range tests {
		if v := sv.ValidateControlTransition(tt[0], tt[1]); v == nil {
			t.Errorf("expected violation for %s -> %s", tt[0], tt[1])
		}
	}
}

func TestCarrierState_ValidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.CarrierState{
		{gem.CarrierNotAccessed, gem.CarrierWaitingForHost},
		{gem.CarrierWaitingForHost, gem.CarrierInAccess},
		{gem.CarrierInAccess, gem.CarrierCarrierComplete},
		{gem.CarrierCarrierComplete, gem.CarrierReadyToUnload},
	}
	for _, tt := range tests {
		if v := sv.ValidateCarrierTransition("FOUP-01", tt[0], tt[1]); v != nil {
			t.Errorf("expected valid: %s -> %s, got: %s", tt[0], tt[1], v.Message)
		}
	}
}

func TestCarrierState_InvalidTransition(t *testing.T) {
	sv := NewStateValidator()
	v := sv.ValidateCarrierTransition("FOUP-01", gem.CarrierNotAccessed, gem.CarrierInAccess)
	if v == nil {
		t.Error("expected violation: NOT_ACCESSED -> IN_ACCESS should skip WAITING_FOR_HOST")
	}
}

func TestPJState_ValidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.ProcessJobState{
		{gem.PJQueued, gem.PJSettingUp},
		{gem.PJSettingUp, gem.PJWaitingForStart},
		{gem.PJWaitingForStart, gem.PJProcessing},
		{gem.PJProcessing, gem.PJProcessComplete},
		{gem.PJProcessing, gem.PJAborting},
		{gem.PJAborting, gem.PJAborted},
	}
	for _, tt := range tests {
		if v := sv.ValidatePJTransition("PJ-01", tt[0], tt[1]); v != nil {
			t.Errorf("expected valid: %s -> %s, got: %s", tt[0], tt[1], v.Message)
		}
	}
}

func TestPJState_InvalidTransition(t *testing.T) {
	sv := NewStateValidator()
	v := sv.ValidatePJTransition("PJ-01", gem.PJQueued, gem.PJProcessing)
	if v == nil {
		t.Error("expected violation: QUEUED -> PROCESSING should skip SETTING_UP")
	}
}

func TestCJState_ValidTransitions(t *testing.T) {
	sv := NewStateValidator()
	tests := [][2]gem.ControlJobState{
		{gem.CJQueued, gem.CJSelected},
		{gem.CJSelected, gem.CJWaitingForStart},
		{gem.CJWaitingForStart, gem.CJExecuting},
		{gem.CJExecuting, gem.CJCompleted},
		{gem.CJExecuting, gem.CJPausing},
		{gem.CJPausing, gem.CJPaused},
		{gem.CJPaused, gem.CJExecuting},
	}
	for _, tt := range tests {
		if v := sv.ValidateCJTransition("CJ-01", tt[0], tt[1]); v != nil {
			t.Errorf("expected valid: %s -> %s, got: %s", tt[0], tt[1], v.Message)
		}
	}
}

func TestCJState_InvalidTransition(t *testing.T) {
	sv := NewStateValidator()
	v := sv.ValidateCJTransition("CJ-01", gem.CJQueued, gem.CJExecuting)
	if v == nil {
		t.Error("expected violation: QUEUED -> EXECUTING should skip SELECTED")
	}
}

func TestStateValidator_TransitionsAndViolationsRecorded(t *testing.T) {
	sv := NewStateValidator()
	sv.ValidateCommTransition(gem.CommDisabled, gem.CommWaitCRA)            // valid
	sv.ValidateCommTransition(gem.CommDisabled, gem.CommCommunicating)      // invalid

	if len(sv.Transitions()) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(sv.Transitions()))
	}
	if len(sv.Violations()) != 1 {
		t.Errorf("expected 1 violation, got %d", len(sv.Violations()))
	}

	sv.Reset()
	if len(sv.Transitions()) != 0 {
		t.Error("reset should clear transitions")
	}
}
