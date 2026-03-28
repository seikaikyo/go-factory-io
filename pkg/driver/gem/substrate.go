package gem

import (
	"fmt"
	"sync"
	"time"
)

// --- E90 Substrate Tracking ---

// SubstrateLocation identifies where a substrate (wafer) currently resides.
type SubstrateLocation struct {
	Type     LocationType // Port, Chamber, Buffer, etc.
	ID       string       // Port ID, Chamber ID, etc.
	Slot     int          // Slot number (1-25 for FOUP, 1-N for chamber)
}

func (l SubstrateLocation) String() string {
	return fmt.Sprintf("%s:%s/slot%d", l.Type, l.ID, l.Slot)
}

// LocationType classifies a substrate location.
type LocationType string

const (
	LocationPort    LocationType = "PORT"    // Load port FOUP slot
	LocationChamber LocationType = "CHAMBER" // Process chamber
	LocationBuffer  LocationType = "BUFFER"  // Internal buffer
	LocationAligner LocationType = "ALIGNER" // Wafer aligner
	LocationRobot   LocationType = "ROBOT"   // Transfer robot arm
)

// Substrate represents a tracked wafer per SEMI E90.
type Substrate struct {
	SubstrateID string             // Unique wafer ID (e.g., "LOT001-01")
	LotID       string             // Parent lot
	CarrierID   string             // Source carrier
	SourceSlot  int                // Original slot in carrier
	Location    SubstrateLocation  // Current location
	State       SubstrateState
	History     []MovementRecord   // Movement log
}

// SubstrateState tracks the processing state of a wafer.
type SubstrateState int

const (
	SubstrateAtSource    SubstrateState = iota // In source carrier
	SubstrateInTransit                          // Being moved
	SubstrateAtProcess                          // In process chamber
	SubstrateProcessed                          // Processing complete
	SubstrateAtDest                             // Back in carrier (destination)
	SubstrateRejected                           // Failed inspection
)

func (s SubstrateState) String() string {
	switch s {
	case SubstrateAtSource:
		return "AT_SOURCE"
	case SubstrateInTransit:
		return "IN_TRANSIT"
	case SubstrateAtProcess:
		return "AT_PROCESS"
	case SubstrateProcessed:
		return "PROCESSED"
	case SubstrateAtDest:
		return "AT_DESTINATION"
	case SubstrateRejected:
		return "REJECTED"
	default:
		return "UNKNOWN"
	}
}

// MovementRecord logs a substrate transfer event.
type MovementRecord struct {
	Time time.Time
	From SubstrateLocation
	To   SubstrateLocation
}

// SubstrateTracker tracks wafer positions and movements per SEMI E90.
type SubstrateTracker struct {
	mu         sync.RWMutex
	substrates map[string]*Substrate // SubstrateID -> Substrate
	onMove     func(subID string, from, to SubstrateLocation)
}

// NewSubstrateTracker creates an E90 substrate tracker.
func NewSubstrateTracker() *SubstrateTracker {
	return &SubstrateTracker{
		substrates: make(map[string]*Substrate),
	}
}

// OnMove sets a callback for substrate movement events.
func (st *SubstrateTracker) OnMove(fn func(subID string, from, to SubstrateLocation)) {
	st.mu.Lock()
	st.onMove = fn
	st.mu.Unlock()
}

// RegisterSubstrate adds a substrate from a carrier slot.
func (st *SubstrateTracker) RegisterSubstrate(subID, lotID, carrierID string, slot int, portID string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.substrates[subID] = &Substrate{
		SubstrateID: subID,
		LotID:       lotID,
		CarrierID:   carrierID,
		SourceSlot:  slot,
		Location: SubstrateLocation{
			Type: LocationPort,
			ID:   portID,
			Slot: slot,
		},
		State: SubstrateAtSource,
	}
}

// MoveSubstrate records a substrate transfer from one location to another.
func (st *SubstrateTracker) MoveSubstrate(subID string, to SubstrateLocation) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	sub, ok := st.substrates[subID]
	if !ok {
		return fmt.Errorf("e90: unknown substrate %s", subID)
	}

	from := sub.Location
	sub.Location = to
	sub.History = append(sub.History, MovementRecord{
		Time: time.Now(),
		From: from,
		To:   to,
	})

	// Update state based on destination type
	switch to.Type {
	case LocationRobot:
		sub.State = SubstrateInTransit
	case LocationChamber, LocationAligner:
		sub.State = SubstrateAtProcess
	case LocationPort:
		if sub.State == SubstrateProcessed || sub.State == SubstrateAtProcess {
			sub.State = SubstrateAtDest
		}
	}

	if st.onMove != nil {
		st.onMove(subID, from, to)
	}
	return nil
}

// MarkProcessed marks a substrate as having completed processing.
func (st *SubstrateTracker) MarkProcessed(subID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	sub, ok := st.substrates[subID]
	if !ok {
		return fmt.Errorf("e90: unknown substrate %s", subID)
	}
	sub.State = SubstrateProcessed
	return nil
}

// MarkRejected marks a substrate as rejected (failed inspection).
func (st *SubstrateTracker) MarkRejected(subID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	sub, ok := st.substrates[subID]
	if !ok {
		return fmt.Errorf("e90: unknown substrate %s", subID)
	}
	sub.State = SubstrateRejected
	return nil
}

// GetSubstrate returns a substrate by ID.
func (st *SubstrateTracker) GetSubstrate(subID string) (*Substrate, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	s, ok := st.substrates[subID]
	return s, ok
}

// GetSubstrateAt returns the substrate at a given location, if any.
func (st *SubstrateTracker) GetSubstrateAt(loc SubstrateLocation) (*Substrate, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	for _, s := range st.substrates {
		if s.Location == loc {
			return s, true
		}
	}
	return nil, false
}

// ListSubstrates returns all tracked substrates.
func (st *SubstrateTracker) ListSubstrates() []*Substrate {
	st.mu.RLock()
	defer st.mu.RUnlock()
	result := make([]*Substrate, 0, len(st.substrates))
	for _, s := range st.substrates {
		result = append(result, s)
	}
	return result
}

// ListByCarrier returns all substrates from a specific carrier.
func (st *SubstrateTracker) ListByCarrier(carrierID string) []*Substrate {
	st.mu.RLock()
	defer st.mu.RUnlock()
	var result []*Substrate
	for _, s := range st.substrates {
		if s.CarrierID == carrierID {
			result = append(result, s)
		}
	}
	return result
}

// RemoveSubstrate removes tracking for a substrate.
func (st *SubstrateTracker) RemoveSubstrate(subID string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.substrates, subID)
}
