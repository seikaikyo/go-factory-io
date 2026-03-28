package gem

import "testing"

func TestCarrierLifecycle(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.SetPortInService(1)

	// Bind carrier to port
	if err := cm.BindCarrier("FOUP-001", 1, "LOT-A", "PRODUCT"); err != nil {
		t.Fatalf("BindCarrier: %v", err)
	}

	c, ok := cm.GetCarrier("FOUP-001")
	if !ok {
		t.Fatal("carrier not found")
	}
	if c.State != CarrierNotAccessed {
		t.Errorf("state = %s, want NOT_ACCESSED", c.State)
	}

	// Full lifecycle: NotAccessed -> WaitingForHost -> InAccess -> CarrierComplete -> ReadyToUnload
	if err := cm.ProceedWithCarrier("FOUP-001"); err != nil {
		t.Fatalf("ProceedWithCarrier: %v", err)
	}
	if err := cm.StartAccess("FOUP-001"); err != nil {
		t.Fatalf("StartAccess: %v", err)
	}
	c, _ = cm.GetCarrier("FOUP-001")
	if c.State != CarrierInAccess {
		t.Errorf("state = %s, want IN_ACCESS", c.State)
	}
	if c.AccessedAt.IsZero() {
		t.Error("AccessedAt should be set")
	}

	if err := cm.CompleteAccess("FOUP-001"); err != nil {
		t.Fatalf("CompleteAccess: %v", err)
	}
	if err := cm.ReadyToUnload("FOUP-001"); err != nil {
		t.Fatalf("ReadyToUnload: %v", err)
	}

	// Port should be ReadyToUnload
	port, _ := cm.GetPort(1)
	if port.State != PortReadyToUnload {
		t.Errorf("port state = %s, want READY_TO_UNLOAD", port.State)
	}

	// Unbind carrier
	if err := cm.UnbindCarrier("FOUP-001"); err != nil {
		t.Fatalf("UnbindCarrier: %v", err)
	}
	if _, ok := cm.GetCarrier("FOUP-001"); ok {
		t.Error("carrier should be removed after unbind")
	}
}

func TestCarrierStopResume(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.SetPortInService(1)
	cm.BindCarrier("FOUP-002", 1, "LOT-B", "PRODUCT")
	cm.ProceedWithCarrier("FOUP-002")
	cm.StartAccess("FOUP-002")

	if err := cm.StopCarrier("FOUP-002"); err != nil {
		t.Fatalf("StopCarrier: %v", err)
	}
	c, _ := cm.GetCarrier("FOUP-002")
	if c.State != CarrierStopped {
		t.Errorf("state = %s, want STOPPED", c.State)
	}

	if err := cm.ResumeCarrier("FOUP-002"); err != nil {
		t.Fatalf("ResumeCarrier: %v", err)
	}
	c, _ = cm.GetCarrier("FOUP-002")
	if c.State != CarrierInAccess {
		t.Errorf("state = %s, want IN_ACCESS", c.State)
	}
}

func TestCarrierInvalidTransition(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.SetPortInService(1)
	cm.BindCarrier("FOUP-003", 1, "LOT-C", "PRODUCT")

	// Can't start access from NotAccessed (must go through WaitingForHost)
	if err := cm.StartAccess("FOUP-003"); err == nil {
		t.Error("expected error for invalid transition")
	}
}

func TestLoadPortStates(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)

	port, _ := cm.GetPort(1)
	if port.State != PortOutOfService {
		t.Errorf("initial state = %s, want OUT_OF_SERVICE", port.State)
	}

	cm.SetPortInService(1)
	port, _ = cm.GetPort(1)
	if port.State != PortReadyToLoad {
		t.Errorf("state = %s, want READY_TO_LOAD", port.State)
	}

	// Can't go out of service while carrier is present
	cm.BindCarrier("FOUP-004", 1, "LOT-D", "TEST")
	if err := cm.SetPortOutOfService(1); err == nil {
		t.Error("expected error: port has carrier")
	}
}

func TestSlotMap(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.SetPortInService(1)
	cm.BindCarrier("FOUP-005", 1, "LOT-E", "PRODUCT")

	var slots [25]SlotState
	slots[0] = SlotOccupied
	slots[1] = SlotOccupied
	slots[2] = SlotEmpty
	slots[24] = SlotOccupied

	cm.SetSlotMap("FOUP-005", slots)

	c, _ := cm.GetCarrier("FOUP-005")
	if c.WaferCount() != 3 {
		t.Errorf("wafer count = %d, want 3", c.WaferCount())
	}
}

func TestCarrierStateCallback(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.SetPortInService(1)

	var gotID string
	var gotOld, gotNew CarrierState
	cm.OnCarrierStateChange(func(id string, old, new_ CarrierState) {
		gotID = id
		gotOld = old
		gotNew = new_
	})

	cm.BindCarrier("FOUP-006", 1, "LOT-F", "PRODUCT")
	cm.ProceedWithCarrier("FOUP-006")

	if gotID != "FOUP-006" {
		t.Errorf("callback ID = %s, want FOUP-006", gotID)
	}
	if gotOld != CarrierNotAccessed || gotNew != CarrierWaitingForHost {
		t.Errorf("transition = %s->%s, want NOT_ACCESSED->WAITING_FOR_HOST", gotOld, gotNew)
	}
}

func TestCarrierListMethods(t *testing.T) {
	cm := NewCarrierManager()
	cm.DefinePort(1)
	cm.DefinePort(2)
	cm.SetPortInService(1)
	cm.SetPortInService(2)

	cm.BindCarrier("A", 1, "LOT-1", "PRODUCT")
	cm.BindCarrier("B", 2, "LOT-2", "TEST")

	carriers := cm.ListCarriers()
	if len(carriers) != 2 {
		t.Errorf("carrier count = %d, want 2", len(carriers))
	}

	ports := cm.ListPorts()
	if len(ports) != 2 {
		t.Errorf("port count = %d, want 2", len(ports))
	}
}
