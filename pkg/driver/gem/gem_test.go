package gem

import (
	"testing"
)

// --- State Machine Tests ---

func TestCommStateTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Initial state
	if sm.CommState() != CommDisabled {
		t.Fatalf("initial: got %s, want DISABLED", sm.CommState())
	}

	// DISABLED -> WAIT_CRA
	if err := sm.EnableComm(); err != nil {
		t.Fatalf("EnableComm: %v", err)
	}
	if sm.CommState() != CommWaitCRA {
		t.Fatalf("after enable: got %s, want WAIT_CRA", sm.CommState())
	}

	// WAIT_CRA -> COMMUNICATING
	if err := sm.CommEstablished(); err != nil {
		t.Fatalf("CommEstablished: %v", err)
	}
	if sm.CommState() != CommCommunicating {
		t.Fatalf("after establish: got %s, want COMMUNICATING", sm.CommState())
	}
	if !sm.IsCommunicating() {
		t.Fatal("expected IsCommunicating=true")
	}

	// COMMUNICATING -> DISABLED
	sm.DisableComm()
	if sm.CommState() != CommDisabled {
		t.Fatalf("after disable: got %s, want DISABLED", sm.CommState())
	}
}

func TestCommStateRetry(t *testing.T) {
	sm := NewStateMachine()
	sm.EnableComm()

	// WAIT_CRA -> WAIT_DELAY (comm failed)
	if err := sm.CommFailed(); err != nil {
		t.Fatalf("CommFailed: %v", err)
	}
	if sm.CommState() != CommWaitDelay {
		t.Fatalf("after fail: got %s, want WAIT_DELAY", sm.CommState())
	}

	// WAIT_DELAY -> WAIT_CRA (retry)
	if err := sm.RetryComm(); err != nil {
		t.Fatalf("RetryComm: %v", err)
	}
	if sm.CommState() != CommWaitCRA {
		t.Fatalf("after retry: got %s, want WAIT_CRA", sm.CommState())
	}
}

func TestCommStateInvalidTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Cannot establish from DISABLED
	if err := sm.CommEstablished(); err == nil {
		t.Fatal("expected error for CommEstablished from DISABLED")
	}

	// Cannot enable from non-DISABLED
	sm.EnableComm()
	if err := sm.EnableComm(); err == nil {
		t.Fatal("expected error for double EnableComm")
	}
}

func TestControlStateTransitions(t *testing.T) {
	sm := NewStateMachine()
	sm.EnableComm()
	sm.CommEstablished()

	// Initial control state
	if sm.ControlState() != ControlOfflineEquipment {
		t.Fatalf("initial control: got %s, want OFFLINE/EQUIPMENT", sm.ControlState())
	}

	// Go ONLINE/REMOTE
	if err := sm.GoOnlineRemote(); err != nil {
		t.Fatalf("GoOnlineRemote: %v", err)
	}
	if sm.ControlState() != ControlOnlineRemote {
		t.Fatalf("after online: got %s, want ONLINE/REMOTE", sm.ControlState())
	}
	if !sm.IsOnline() {
		t.Fatal("expected IsOnline=true")
	}

	// Go OFFLINE
	if err := sm.GoOffline(); err != nil {
		t.Fatalf("GoOffline: %v", err)
	}
	if sm.ControlState() != ControlOfflineEquipment {
		t.Fatalf("after offline: got %s, want OFFLINE/EQUIPMENT", sm.ControlState())
	}

	// Go ONLINE/LOCAL
	if err := sm.GoOnlineLocal(); err != nil {
		t.Fatalf("GoOnlineLocal: %v", err)
	}
	if sm.ControlState() != ControlOnlineLocal {
		t.Fatalf("after online local: got %s, want ONLINE/LOCAL", sm.ControlState())
	}
}

func TestControlStateInvalidTransitions(t *testing.T) {
	sm := NewStateMachine()

	// Cannot go online without communicating
	if err := sm.GoOnlineRemote(); err == nil {
		t.Fatal("expected error for GoOnlineRemote without comm")
	}

	// Cannot go offline when already offline
	if err := sm.GoOffline(); err == nil {
		t.Fatal("expected error for GoOffline when already offline")
	}
}

func TestDisableCommResetsControlState(t *testing.T) {
	sm := NewStateMachine()
	sm.EnableComm()
	sm.CommEstablished()
	sm.GoOnlineRemote()

	sm.DisableComm()
	if sm.ControlState() != ControlOfflineEquipment {
		t.Fatalf("expected OFFLINE/EQUIPMENT after DisableComm, got %s", sm.ControlState())
	}
}

func TestStateChangeCallback(t *testing.T) {
	sm := NewStateMachine()
	var lastComm CommState
	var lastControl ControlState
	callCount := 0

	sm.OnChange(func(comm CommState, control ControlState) {
		lastComm = comm
		lastControl = control
		callCount++
	})

	sm.EnableComm()
	if callCount != 1 || lastComm != CommWaitCRA {
		t.Errorf("callback: count=%d, comm=%s", callCount, lastComm)
	}

	sm.CommEstablished()
	sm.GoOnlineRemote()
	if callCount != 3 || lastControl != ControlOnlineRemote {
		t.Errorf("callback: count=%d, control=%s", callCount, lastControl)
	}
}

// --- Variable Store Tests ---

func TestVariableStoreEC(t *testing.T) {
	vs := NewVariableStore()

	vs.DefineEC(&EquipmentConstant{
		ECID:  1,
		Name:  "Temperature",
		Value: float64(25.0),
		Units: "C",
	})

	ec, ok := vs.GetEC(1)
	if !ok {
		t.Fatal("EC not found")
	}
	if ec.Name != "Temperature" {
		t.Errorf("Name: got %q, want %q", ec.Name, "Temperature")
	}

	if err := vs.SetEC(1, float64(30.0)); err != nil {
		t.Fatalf("SetEC: %v", err)
	}
	ec, _ = vs.GetEC(1)
	if ec.Value.(float64) != 30.0 {
		t.Errorf("Value: got %v, want 30.0", ec.Value)
	}

	if err := vs.SetEC(999, "x"); err == nil {
		t.Fatal("expected error for unknown ECID")
	}
}

func TestVariableStoreSV(t *testing.T) {
	vs := NewVariableStore()

	vs.DefineSV(&StatusVariable{
		SVID:  1001,
		Name:  "WaferCount",
		Value: uint32(42),
		Units: "pcs",
	})

	val, ok := vs.GetSV(1001)
	if !ok {
		t.Fatal("SV not found")
	}
	if val.(uint32) != 42 {
		t.Errorf("Value: got %v, want 42", val)
	}

	// Dynamic SV
	counter := 0
	vs.DefineSVDynamic(2001, "Counter", "", func() interface{} {
		counter++
		return counter
	})

	v1, _ := vs.GetSV(2001)
	v2, _ := vs.GetSV(2001)
	if v1.(int) != 1 || v2.(int) != 2 {
		t.Errorf("Dynamic SV: got %v, %v; want 1, 2", v1, v2)
	}
}

// --- Event Manager Tests ---

func TestEventManagerDefineAndLink(t *testing.T) {
	em := NewEventManager()

	// Define events
	em.DefineEvent(100, "ProcessComplete")
	em.DefineEvent(200, "LotComplete")

	// Define reports
	em.DefineReport(1, []uint32{1001, 1002, 1003})
	em.DefineReport(2, []uint32{2001, 2002})

	// Link events to reports
	if err := em.LinkEventReport(100, []uint32{1, 2}); err != nil {
		t.Fatalf("LinkEventReport: %v", err)
	}

	// Get event report VIDs
	reports, err := em.GetEventReportVIDs(100)
	if err != nil {
		t.Fatalf("GetEventReportVIDs: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
	if reports[0].RPTID != 1 || len(reports[0].VIDs) != 3 {
		t.Errorf("report 0: RPTID=%d, VIDs=%v", reports[0].RPTID, reports[0].VIDs)
	}
}

func TestEventManagerEnableDisable(t *testing.T) {
	em := NewEventManager()
	em.DefineEvent(100, "TestEvent")
	em.DefineReport(1, []uint32{1001})
	em.LinkEventReport(100, []uint32{1})

	// Disable event
	em.EnableEvent(100, false)
	if em.IsEventEnabled(100) {
		t.Fatal("expected event to be disabled")
	}

	// Disabled event returns no reports
	reports, _ := em.GetEventReportVIDs(100)
	if reports != nil {
		t.Fatal("expected nil reports for disabled event")
	}

	// Re-enable
	em.EnableEvent(100, true)
	reports, _ = em.GetEventReportVIDs(100)
	if len(reports) != 1 {
		t.Fatalf("expected 1 report after re-enable, got %d", len(reports))
	}
}

func TestEventManagerDeleteAllReports(t *testing.T) {
	em := NewEventManager()
	em.DefineEvent(100, "TestEvent")
	em.DefineReport(1, []uint32{1001})
	em.LinkEventReport(100, []uint32{1})

	em.DeleteAllReports()

	// Event should have no linked reports
	reports, _ := em.GetEventReportVIDs(100)
	if len(reports) != 0 {
		t.Fatalf("expected 0 reports after delete all, got %d", len(reports))
	}
}

func TestEventManagerLinkUnknownCEID(t *testing.T) {
	em := NewEventManager()
	em.DefineReport(1, []uint32{1001})
	if err := em.LinkEventReport(999, []uint32{1}); err == nil {
		t.Fatal("expected error for unknown CEID")
	}
}

func TestEventManagerLinkUnknownRPTID(t *testing.T) {
	em := NewEventManager()
	em.DefineEvent(100, "TestEvent")
	if err := em.LinkEventReport(100, []uint32{999}); err == nil {
		t.Fatal("expected error for unknown RPTID")
	}
}

// --- Value conversion ---

func TestValueToItem(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
	}{
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"int32", int32(100)},
		{"int64", int64(200)},
		{"uint32", uint32(300)},
		{"uint64", uint64(400)},
		{"float32", float32(1.5)},
		{"float64", float64(2.5)},
		{"bool", true},
		{"bytes", []byte{0x01, 0x02}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := valueToItem(tt.val)
			if item == nil {
				t.Fatal("expected non-nil item")
			}
		})
	}
}
