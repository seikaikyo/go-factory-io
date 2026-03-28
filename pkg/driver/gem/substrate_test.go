package gem

import "testing"

func TestSubstrateMovement(t *testing.T) {
	st := NewSubstrateTracker()
	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")

	sub, ok := st.GetSubstrate("W001")
	if !ok {
		t.Fatal("substrate not found")
	}
	if sub.State != SubstrateAtSource {
		t.Errorf("state = %s, want AT_SOURCE", sub.State)
	}
	if sub.Location.Type != LocationPort || sub.Location.Slot != 1 {
		t.Errorf("location = %s, want PORT:PORT1/slot1", sub.Location)
	}

	// Move to robot arm
	st.MoveSubstrate("W001", SubstrateLocation{Type: LocationRobot, ID: "ARM1", Slot: 1})
	sub, _ = st.GetSubstrate("W001")
	if sub.State != SubstrateInTransit {
		t.Errorf("state = %s, want IN_TRANSIT", sub.State)
	}

	// Move to chamber
	st.MoveSubstrate("W001", SubstrateLocation{Type: LocationChamber, ID: "CH1", Slot: 1})
	sub, _ = st.GetSubstrate("W001")
	if sub.State != SubstrateAtProcess {
		t.Errorf("state = %s, want AT_PROCESS", sub.State)
	}

	// Mark processed
	st.MarkProcessed("W001")
	sub, _ = st.GetSubstrate("W001")
	if sub.State != SubstrateProcessed {
		t.Errorf("state = %s, want PROCESSED", sub.State)
	}

	// Move back to port
	st.MoveSubstrate("W001", SubstrateLocation{Type: LocationPort, ID: "PORT2", Slot: 1})
	sub, _ = st.GetSubstrate("W001")
	if sub.State != SubstrateAtDest {
		t.Errorf("state = %s, want AT_DESTINATION", sub.State)
	}

	// Check history
	if len(sub.History) != 3 {
		t.Errorf("history length = %d, want 3", len(sub.History))
	}
}

func TestSubstrateGetAt(t *testing.T) {
	st := NewSubstrateTracker()
	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")
	st.RegisterSubstrate("W002", "LOT-A", "FOUP-001", 2, "PORT1")

	loc := SubstrateLocation{Type: LocationPort, ID: "PORT1", Slot: 2}
	sub, ok := st.GetSubstrateAt(loc)
	if !ok {
		t.Fatal("substrate not found at location")
	}
	if sub.SubstrateID != "W002" {
		t.Errorf("substrate = %s, want W002", sub.SubstrateID)
	}
}

func TestSubstrateListByCarrier(t *testing.T) {
	st := NewSubstrateTracker()
	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")
	st.RegisterSubstrate("W002", "LOT-A", "FOUP-001", 2, "PORT1")
	st.RegisterSubstrate("W003", "LOT-B", "FOUP-002", 1, "PORT2")

	subs := st.ListByCarrier("FOUP-001")
	if len(subs) != 2 {
		t.Errorf("count = %d, want 2", len(subs))
	}
}

func TestSubstrateReject(t *testing.T) {
	st := NewSubstrateTracker()
	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")
	st.MarkRejected("W001")

	sub, _ := st.GetSubstrate("W001")
	if sub.State != SubstrateRejected {
		t.Errorf("state = %s, want REJECTED", sub.State)
	}
}

func TestSubstrateMoveCallback(t *testing.T) {
	st := NewSubstrateTracker()
	var gotID string
	st.OnMove(func(id string, from, to SubstrateLocation) {
		gotID = id
	})

	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")
	st.MoveSubstrate("W001", SubstrateLocation{Type: LocationChamber, ID: "CH1", Slot: 1})

	if gotID != "W001" {
		t.Errorf("callback ID = %s, want W001", gotID)
	}
}

func TestSubstrateRemove(t *testing.T) {
	st := NewSubstrateTracker()
	st.RegisterSubstrate("W001", "LOT-A", "FOUP-001", 1, "PORT1")
	st.RemoveSubstrate("W001")

	if _, ok := st.GetSubstrate("W001"); ok {
		t.Error("substrate should be removed")
	}
}

func TestSubstrateUnknown(t *testing.T) {
	st := NewSubstrateTracker()
	if err := st.MoveSubstrate("NONE", SubstrateLocation{}); err == nil {
		t.Error("expected error for unknown substrate")
	}
	if err := st.MarkProcessed("NONE"); err == nil {
		t.Error("expected error for unknown substrate")
	}
}
