package gem

import (
	"fmt"
	"sync"
)

// VariableStore manages Equipment Constants (EC) and Status Variables (SV).
type VariableStore struct {
	mu  sync.RWMutex
	ecs map[uint32]*EquipmentConstant
	svs map[uint32]*StatusVariable
}

// EquipmentConstant is a configurable equipment parameter (SEMI E30 ECID).
type EquipmentConstant struct {
	ECID     uint32
	Name     string
	Value    interface{}
	MinValue interface{}
	MaxValue interface{}
	Units    string
}

// StatusVariable is a read-only equipment status value (SEMI E30 SVID).
type StatusVariable struct {
	SVID   uint32
	Name   string
	Value  interface{}
	Units  string
	update func() interface{} // Optional: dynamic value provider
}

// NewVariableStore creates an empty variable store.
func NewVariableStore() *VariableStore {
	return &VariableStore{
		ecs: make(map[uint32]*EquipmentConstant),
		svs: make(map[uint32]*StatusVariable),
	}
}

// --- Equipment Constants ---

// DefineEC registers a new equipment constant.
func (vs *VariableStore) DefineEC(ec *EquipmentConstant) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.ecs[ec.ECID] = ec
}

// GetEC returns an equipment constant by ID.
func (vs *VariableStore) GetEC(ecid uint32) (*EquipmentConstant, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	ec, ok := vs.ecs[ecid]
	return ec, ok
}

// SetEC updates an equipment constant value.
func (vs *VariableStore) SetEC(ecid uint32, value interface{}) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	ec, ok := vs.ecs[ecid]
	if !ok {
		return fmt.Errorf("gem: unknown ECID %d", ecid)
	}
	ec.Value = value
	return nil
}

// ListECIDs returns all registered equipment constant IDs.
func (vs *VariableStore) ListECIDs() []uint32 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	ids := make([]uint32, 0, len(vs.ecs))
	for id := range vs.ecs {
		ids = append(ids, id)
	}
	return ids
}

// --- Status Variables ---

// DefineSV registers a new status variable.
func (vs *VariableStore) DefineSV(sv *StatusVariable) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.svs[sv.SVID] = sv
}

// DefineSVDynamic registers a status variable with a dynamic value provider.
func (vs *VariableStore) DefineSVDynamic(svid uint32, name, units string, fn func() interface{}) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.svs[svid] = &StatusVariable{
		SVID:   svid,
		Name:   name,
		Units:  units,
		update: fn,
	}
}

// GetSV returns a status variable value by ID. If the SV has a dynamic
// provider, it calls the provider to get the current value.
func (vs *VariableStore) GetSV(svid uint32) (interface{}, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	sv, ok := vs.svs[svid]
	if !ok {
		return nil, false
	}
	if sv.update != nil {
		return sv.update(), true
	}
	return sv.Value, true
}

// SetSV updates a status variable value.
func (vs *VariableStore) SetSV(svid uint32, value interface{}) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	sv, ok := vs.svs[svid]
	if !ok {
		return fmt.Errorf("gem: unknown SVID %d", svid)
	}
	sv.Value = value
	return nil
}

// ListSVIDs returns all registered status variable IDs.
func (vs *VariableStore) ListSVIDs() []uint32 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	ids := make([]uint32, 0, len(vs.svs))
	for id := range vs.svs {
		ids = append(ids, id)
	}
	return ids
}

// GetSVInfo returns the status variable metadata.
func (vs *VariableStore) GetSVInfo(svid uint32) (*StatusVariable, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	sv, ok := vs.svs[svid]
	return sv, ok
}

// GetECInfo returns the equipment constant metadata.
func (vs *VariableStore) GetECInfo(ecid uint32) (*EquipmentConstant, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	ec, ok := vs.ecs[ecid]
	return ec, ok
}
