package gem

import (
	"fmt"
	"sync"
	"time"
)

// --- E87 Carrier State Model ---

// CarrierState represents the carrier lifecycle state per SEMI E87.
type CarrierState int

const (
	CarrierNotAccessed     CarrierState = iota // Initial: carrier present but not accessed
	CarrierWaitingForHost                      // Waiting for host instruction
	CarrierInAccess                            // Being accessed (wafers transferring)
	CarrierCarrierComplete                     // Access completed
	CarrierStopped                             // Processing halted mid-access
	CarrierReadyToUnload                       // Ready for physical removal
)

func (s CarrierState) String() string {
	switch s {
	case CarrierNotAccessed:
		return "NOT_ACCESSED"
	case CarrierWaitingForHost:
		return "WAITING_FOR_HOST"
	case CarrierInAccess:
		return "IN_ACCESS"
	case CarrierCarrierComplete:
		return "CARRIER_COMPLETE"
	case CarrierStopped:
		return "STOPPED"
	case CarrierReadyToUnload:
		return "READY_TO_UNLOAD"
	default:
		return "UNKNOWN"
	}
}

// --- Load Port State Model ---

// LoadPortState represents the load port state per SEMI E87.
type LoadPortState int

const (
	PortOutOfService     LoadPortState = iota // Port not available
	PortTransferBlocked                       // In service but blocked
	PortReadyToLoad                           // Ready to accept carrier
	PortTransferReady                         // Carrier present, ready for transfer
	PortReadyToUnload                         // Carrier done, ready for removal
)

func (s LoadPortState) String() string {
	switch s {
	case PortOutOfService:
		return "OUT_OF_SERVICE"
	case PortTransferBlocked:
		return "TRANSFER_BLOCKED"
	case PortReadyToLoad:
		return "READY_TO_LOAD"
	case PortTransferReady:
		return "TRANSFER_READY"
	case PortReadyToUnload:
		return "READY_TO_UNLOAD"
	default:
		return "UNKNOWN"
	}
}

// --- Carrier ---

// Carrier represents a 300mm wafer carrier (FOUP/FOSB) per SEMI E87.
type Carrier struct {
	CarrierID  string       // Unique carrier identifier
	State      CarrierState // Current lifecycle state
	SlotMap    [25]SlotState // 25-slot FOUP map
	PortID     uint32       // Load port where carrier is located
	LotID      string       // Associated lot identifier
	Usage      string       // "PRODUCT", "TEST", "DUMMY", etc.
	CreatedAt  time.Time
	AccessedAt time.Time    // When access started
}

// SlotState represents the occupancy state of a FOUP slot.
type SlotState byte

const (
	SlotEmpty          SlotState = 0 // No wafer
	SlotOccupied       SlotState = 1 // Wafer present
	SlotDoubleSlotted  SlotState = 2 // Error: two wafers
	SlotCrossSlotted   SlotState = 3 // Error: wafer between slots
	SlotUnknown        SlotState = 4 // Not yet mapped
)

func (s SlotState) String() string {
	switch s {
	case SlotEmpty:
		return "EMPTY"
	case SlotOccupied:
		return "OCCUPIED"
	case SlotDoubleSlotted:
		return "DOUBLE_SLOTTED"
	case SlotCrossSlotted:
		return "CROSS_SLOTTED"
	case SlotUnknown:
		return "UNKNOWN"
	default:
		return "INVALID"
	}
}

// WaferCount returns the number of occupied slots.
func (c *Carrier) WaferCount() int {
	count := 0
	for _, s := range c.SlotMap {
		if s == SlotOccupied {
			count++
		}
	}
	return count
}

// --- Load Port ---

// LoadPort represents a physical load port on 300mm equipment.
type LoadPort struct {
	PortID    uint32
	State     LoadPortState
	CarrierID string // ID of carrier on port (empty if none)
}

// --- Carrier Manager ---

// CarrierManager manages carriers and load ports per SEMI E87.
type CarrierManager struct {
	mu       sync.RWMutex
	carriers map[string]*Carrier  // CarrierID -> Carrier
	ports    map[uint32]*LoadPort // PortID -> LoadPort
	onChange func(carrierID string, oldState, newState CarrierState)
}

// NewCarrierManager creates an E87 carrier manager.
func NewCarrierManager() *CarrierManager {
	return &CarrierManager{
		carriers: make(map[string]*Carrier),
		ports:    make(map[uint32]*LoadPort),
	}
}

// OnCarrierStateChange sets a callback for carrier state transitions.
func (cm *CarrierManager) OnCarrierStateChange(fn func(carrierID string, oldState, newState CarrierState)) {
	cm.mu.Lock()
	cm.onChange = fn
	cm.mu.Unlock()
}

// DefinePort registers a load port.
func (cm *CarrierManager) DefinePort(portID uint32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.ports[portID] = &LoadPort{
		PortID: portID,
		State:  PortOutOfService,
	}
}

// SetPortInService transitions port to in-service (ReadyToLoad).
func (cm *CarrierManager) SetPortInService(portID uint32) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	port, ok := cm.ports[portID]
	if !ok {
		return fmt.Errorf("e87: unknown port %d", portID)
	}
	port.State = PortReadyToLoad
	return nil
}

// SetPortOutOfService transitions port to out-of-service.
func (cm *CarrierManager) SetPortOutOfService(portID uint32) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	port, ok := cm.ports[portID]
	if !ok {
		return fmt.Errorf("e87: unknown port %d", portID)
	}
	if port.CarrierID != "" {
		return fmt.Errorf("e87: port %d has carrier %s, cannot go out of service", portID, port.CarrierID)
	}
	port.State = PortOutOfService
	return nil
}

// BindCarrier associates a carrier with a load port.
func (cm *CarrierManager) BindCarrier(carrierID string, portID uint32, lotID, usage string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	port, ok := cm.ports[portID]
	if !ok {
		return fmt.Errorf("e87: unknown port %d", portID)
	}
	if port.State != PortReadyToLoad {
		return fmt.Errorf("e87: port %d not ready to load (state=%s)", portID, port.State)
	}
	if port.CarrierID != "" {
		return fmt.Errorf("e87: port %d already has carrier %s", portID, port.CarrierID)
	}

	// Create carrier in NotAccessed state
	var slotMap [25]SlotState
	for i := range slotMap {
		slotMap[i] = SlotUnknown
	}

	c := &Carrier{
		CarrierID: carrierID,
		State:     CarrierNotAccessed,
		SlotMap:   slotMap,
		PortID:    portID,
		LotID:     lotID,
		Usage:     usage,
		CreatedAt: time.Now(),
	}

	cm.carriers[carrierID] = c
	port.CarrierID = carrierID
	port.State = PortTransferReady
	return nil
}

// UnbindCarrier removes a carrier from its load port.
func (cm *CarrierManager) UnbindCarrier(carrierID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	c, ok := cm.carriers[carrierID]
	if !ok {
		return fmt.Errorf("e87: unknown carrier %s", carrierID)
	}
	if c.State != CarrierReadyToUnload && c.State != CarrierNotAccessed {
		return fmt.Errorf("e87: carrier %s not ready to unbind (state=%s)", carrierID, c.State)
	}

	if port, ok := cm.ports[c.PortID]; ok {
		port.CarrierID = ""
		port.State = PortReadyToLoad
	}

	delete(cm.carriers, carrierID)
	return nil
}

// transitionCarrier changes carrier state with validation.
func (cm *CarrierManager) transitionCarrier(carrierID string, expected []CarrierState, newState CarrierState) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	c, ok := cm.carriers[carrierID]
	if !ok {
		return fmt.Errorf("e87: unknown carrier %s", carrierID)
	}

	valid := false
	for _, s := range expected {
		if c.State == s {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("e87: carrier %s cannot transition from %s to %s", carrierID, c.State, newState)
	}

	oldState := c.State
	c.State = newState

	if newState == CarrierInAccess {
		c.AccessedAt = time.Now()
	}

	if cm.onChange != nil {
		cm.onChange(carrierID, oldState, newState)
	}
	return nil
}

// StartAccess transitions carrier from WaitingForHost to InAccess.
func (cm *CarrierManager) StartAccess(carrierID string) error {
	return cm.transitionCarrier(carrierID, []CarrierState{CarrierWaitingForHost}, CarrierInAccess)
}

// ProceedWithCarrier transitions from NotAccessed to WaitingForHost.
func (cm *CarrierManager) ProceedWithCarrier(carrierID string) error {
	return cm.transitionCarrier(carrierID, []CarrierState{CarrierNotAccessed}, CarrierWaitingForHost)
}

// CompleteAccess transitions from InAccess to CarrierComplete.
func (cm *CarrierManager) CompleteAccess(carrierID string) error {
	return cm.transitionCarrier(carrierID, []CarrierState{CarrierInAccess}, CarrierCarrierComplete)
}

// StopCarrier transitions from InAccess to Stopped.
func (cm *CarrierManager) StopCarrier(carrierID string) error {
	return cm.transitionCarrier(carrierID, []CarrierState{CarrierInAccess}, CarrierStopped)
}

// ResumeCarrier transitions from Stopped back to InAccess.
func (cm *CarrierManager) ResumeCarrier(carrierID string) error {
	return cm.transitionCarrier(carrierID, []CarrierState{CarrierStopped}, CarrierInAccess)
}

// ReadyToUnload transitions carrier to ReadyToUnload.
func (cm *CarrierManager) ReadyToUnload(carrierID string) error {
	err := cm.transitionCarrier(carrierID, []CarrierState{CarrierCarrierComplete, CarrierStopped, CarrierNotAccessed}, CarrierReadyToUnload)
	if err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if c, ok := cm.carriers[carrierID]; ok {
		if port, ok := cm.ports[c.PortID]; ok {
			port.State = PortReadyToUnload
		}
	}
	return nil
}

// SetSlotMap updates the carrier slot map after mapping operation.
func (cm *CarrierManager) SetSlotMap(carrierID string, slots [25]SlotState) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	c, ok := cm.carriers[carrierID]
	if !ok {
		return fmt.Errorf("e87: unknown carrier %s", carrierID)
	}
	c.SlotMap = slots
	return nil
}

// GetCarrier returns a carrier by ID.
func (cm *CarrierManager) GetCarrier(carrierID string) (*Carrier, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.carriers[carrierID]
	return c, ok
}

// GetPort returns a load port by ID.
func (cm *CarrierManager) GetPort(portID uint32) (*LoadPort, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	p, ok := cm.ports[portID]
	return p, ok
}

// ListCarriers returns all carriers.
func (cm *CarrierManager) ListCarriers() []*Carrier {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*Carrier, 0, len(cm.carriers))
	for _, c := range cm.carriers {
		result = append(result, c)
	}
	return result
}

// ListPorts returns all load ports.
func (cm *CarrierManager) ListPorts() []*LoadPort {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*LoadPort, 0, len(cm.ports))
	for _, p := range cm.ports {
		result = append(result, p)
	}
	return result
}
