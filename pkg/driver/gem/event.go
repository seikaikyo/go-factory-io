package gem

import (
	"fmt"
	"sync"
)

// EventManager manages Collection Events (CE), Reports, and their linkages.
type EventManager struct {
	mu      sync.RWMutex
	events  map[uint32]*CollectionEvent
	reports map[uint32]*ReportDef
}

// CollectionEvent represents a GEM collection event (CEID).
type CollectionEvent struct {
	CEID      uint32
	Name      string
	Enabled   bool
	ReportIDs []uint32 // Linked report IDs
}

// ReportDef defines a report (RPTID) and its associated variable IDs.
type ReportDef struct {
	RPTID uint32
	VIDs  []uint32 // Variable IDs included in this report
}

// NewEventManager creates an empty event manager.
func NewEventManager() *EventManager {
	return &EventManager{
		events:  make(map[uint32]*CollectionEvent),
		reports: make(map[uint32]*ReportDef),
	}
}

// --- Report Management (S2F33) ---

// DefineReport creates or replaces a report definition.
func (em *EventManager) DefineReport(rptid uint32, vids []uint32) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.reports[rptid] = &ReportDef{
		RPTID: rptid,
		VIDs:  vids,
	}
}

// DeleteReport removes a report definition.
func (em *EventManager) DeleteReport(rptid uint32) {
	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.reports, rptid)
}

// DeleteAllReports removes all report definitions and event links.
func (em *EventManager) DeleteAllReports() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.reports = make(map[uint32]*ReportDef)
	for _, ce := range em.events {
		ce.ReportIDs = nil
	}
}

// GetReport returns a report definition by ID.
func (em *EventManager) GetReport(rptid uint32) (*ReportDef, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	rpt, ok := em.reports[rptid]
	return rpt, ok
}

// --- Event Management ---

// DefineEvent registers a collection event.
func (em *EventManager) DefineEvent(ceid uint32, name string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.events[ceid] = &CollectionEvent{
		CEID:    ceid,
		Name:    name,
		Enabled: true,
	}
}

// GetEvent returns a collection event by ID.
func (em *EventManager) GetEvent(ceid uint32) (*CollectionEvent, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	ce, ok := em.events[ceid]
	return ce, ok
}

// --- Link Event to Report (S2F35) ---

// LinkEventReport links reports to a collection event.
func (em *EventManager) LinkEventReport(ceid uint32, rptids []uint32) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	ce, ok := em.events[ceid]
	if !ok {
		return fmt.Errorf("gem: unknown CEID %d", ceid)
	}
	// Verify all reports exist
	for _, rptid := range rptids {
		if _, ok := em.reports[rptid]; !ok {
			return fmt.Errorf("gem: unknown RPTID %d", rptid)
		}
	}
	ce.ReportIDs = rptids
	return nil
}

// --- Enable/Disable Events (S2F37) ---

// EnableEvent enables or disables a collection event.
func (em *EventManager) EnableEvent(ceid uint32, enabled bool) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	ce, ok := em.events[ceid]
	if !ok {
		return fmt.Errorf("gem: unknown CEID %d", ceid)
	}
	ce.Enabled = enabled
	return nil
}

// EnableAllEvents enables or disables all collection events.
func (em *EventManager) EnableAllEvents(enabled bool) {
	em.mu.Lock()
	defer em.mu.Unlock()
	for _, ce := range em.events {
		ce.Enabled = enabled
	}
}

// IsEventEnabled returns whether a collection event is enabled.
func (em *EventManager) IsEventEnabled(ceid uint32) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	ce, ok := em.events[ceid]
	if !ok {
		return false
	}
	return ce.Enabled
}

// --- Event Triggering ---

// GetEventReportVIDs returns the variable IDs needed for an event's reports.
// Used when building S6F11 event report messages.
func (em *EventManager) GetEventReportVIDs(ceid uint32) ([]ReportVIDs, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ce, ok := em.events[ceid]
	if !ok {
		return nil, fmt.Errorf("gem: unknown CEID %d", ceid)
	}
	if !ce.Enabled {
		return nil, nil // Event disabled, no reports
	}

	var result []ReportVIDs
	for _, rptid := range ce.ReportIDs {
		rpt, ok := em.reports[rptid]
		if !ok {
			continue
		}
		result = append(result, ReportVIDs{
			RPTID: rpt.RPTID,
			VIDs:  rpt.VIDs,
		})
	}
	return result, nil
}

// ReportVIDs pairs a report ID with its variable IDs.
type ReportVIDs struct {
	RPTID uint32
	VIDs  []uint32
}
