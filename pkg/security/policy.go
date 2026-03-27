package security

import "fmt"

// SFPair identifies a SECS-II Stream/Function message type.
type SFPair struct {
	Stream   byte
	Function byte
}

func (sf SFPair) String() string {
	return fmt.Sprintf("S%dF%d", sf.Stream, sf.Function)
}

// SessionPolicy defines access control for a SECS/GEM session.
// Covers IEC 62443 FR2 (Use Control).
type SessionPolicy struct {
	// AllowedMessages: S/F pairs this session can send.
	// nil = allow all (no restriction).
	AllowedMessages []SFPair

	// DeniedMessages: S/F pairs explicitly blocked.
	// Checked after AllowedMessages.
	DeniedMessages []SFPair

	// ReadOnly: if true, blocks all write/control operations.
	// Denies: S2F15 (set EC), S2F33 (define report), S2F35 (link event),
	// S2F37 (enable/disable event), S2F41 (RCMD), S1F15 (offline), S1F17 (online).
	ReadOnly bool
}

// writeMessages are S/F pairs that modify equipment state.
var writeMessages = []SFPair{
	{2, 15}, // New Equipment Constant Send
	{2, 33}, // Define Report
	{2, 35}, // Link Event Report
	{2, 37}, // Enable/Disable Event Report
	{2, 41}, // Host Command Send (RCMD)
	{1, 15}, // Request OFF-LINE
	{1, 17}, // Request ON-LINE
	{5, 3},  // Enable/Disable Alarm
}

// IsAllowed checks if a message S/F pair is permitted by this policy.
func (p *SessionPolicy) IsAllowed(stream, function byte) bool {
	if p == nil {
		return true // No policy = allow all
	}

	sf := SFPair{stream, function}

	// ReadOnly blocks write operations
	if p.ReadOnly {
		for _, w := range writeMessages {
			if w == sf {
				return false
			}
		}
	}

	// Explicit deny list
	for _, d := range p.DeniedMessages {
		if d == sf {
			return false
		}
	}

	// If allowlist is set, only those are permitted
	if len(p.AllowedMessages) > 0 {
		for _, a := range p.AllowedMessages {
			if a == sf {
				return true
			}
		}
		return false
	}

	return true
}

// ReadOnlyPolicy returns a policy that only allows read operations.
func ReadOnlyPolicy() *SessionPolicy {
	return &SessionPolicy{ReadOnly: true}
}

// MonitorPolicy returns a policy for monitoring-only access:
// can read SV/EC, receive events, but cannot modify anything.
func MonitorPolicy() *SessionPolicy {
	return &SessionPolicy{
		AllowedMessages: []SFPair{
			{1, 1},  // Are You There
			{1, 3},  // Selected Equipment Status
			{1, 11}, // SV Namelist
			{1, 13}, // Establish Communication
			{2, 13}, // Equipment Constant Request
			{2, 29}, // EC Namelist
			{5, 5},  // List Alarms
			{5, 7},  // List Enabled Alarms
		},
	}
}

// FullAccessPolicy returns a policy that allows everything.
func FullAccessPolicy() *SessionPolicy {
	return nil // nil = no restriction
}
