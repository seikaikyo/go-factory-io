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
	if err := checkECRange(ec, value); err != nil {
		return err
	}
	ec.Value = value
	return nil
}

// numeric coerces the Go numeric kinds an EquipmentConstant can hold into a
// float64 for comparison. Anything else reports ok false; a non-numeric
// constant simply has no range to enforce.
func numeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

// checkECRange enforces MinValue and MaxValue when the constant declares them.
//
// These bounds were already part of the type and are reported to the host in
// S2F30, but nothing enforced them on write: SetEC took any value for any
// known ECID. An equipment constant is a process parameter, so an out of range
// write is a physical instruction, not a bad record.
//
// A constant that declares no bounds keeps its previous behaviour: anything
// goes. Enforcing a bound we do not have would break every existing
// definition, none of which sets them.
func checkECRange(ec *EquipmentConstant, value interface{}) error {
	v, ok := numeric(value)
	if !ok {
		// A non-numeric value cannot be compared against a numeric bound.
		// Reject it only when the constant declares one, since that says the
		// constant is numeric.
		if ec.MinValue != nil || ec.MaxValue != nil {
			return fmt.Errorf("gem: ECID %d takes a numeric value, got %T", ec.ECID, value)
		}
		return nil
	}
	if min, ok := numeric(ec.MinValue); ok && v < min {
		return fmt.Errorf("gem: ECID %d value %v below minimum %v", ec.ECID, value, ec.MinValue)
	}
	if max, ok := numeric(ec.MaxValue); ok && v > max {
		return fmt.Errorf("gem: ECID %d value %v above maximum %v", ec.ECID, value, ec.MaxValue)
	}
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
