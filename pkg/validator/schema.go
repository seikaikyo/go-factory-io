// Package validator provides SECS/GEM message validation, state transition
// checking, protocol timing compliance, and implementation coverage reporting.
package validator

import (
	"fmt"
	"strings"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
)

// Direction indicates the expected message flow.
type Direction int

const (
	HostToEquip   Direction = iota // Host sends to equipment
	EquipToHost                    // Equipment sends to host
	Bidirectional                  // Either side may initiate
)

func (d Direction) String() string {
	switch d {
	case HostToEquip:
		return "H->E"
	case EquipToHost:
		return "E->H"
	case Bidirectional:
		return "H<->E"
	default:
		return "?"
	}
}

// Level indicates the severity of a validation result.
type Level int

const (
	LevelPass Level = iota
	LevelWarn
	LevelFail
)

func (l Level) String() string {
	switch l {
	case LevelPass:
		return "PASS"
	case LevelWarn:
		return "WARN"
	case LevelFail:
		return "FAIL"
	default:
		return "?"
	}
}

// ValidationResult is the outcome of a single validation check.
type ValidationResult struct {
	Level   Level  `json:"level"`
	Path    string `json:"path"`    // e.g. "[0]" for first list child
	Message string `json:"message"` // human-readable
}

// ItemSchema describes the expected shape of a SECS-II item.
type ItemSchema struct {
	Format   secs2.Format   // Expected format (FormatList, FormatASCII, etc.)
	Name     string         // Field name for reporting (e.g. "COMMACK", "MDLN")
	MinLen   int            // Minimum element count (0 = optional/empty allowed)
	MaxLen   int            // Maximum element count (-1 = unlimited)
	Children []*ItemSchema  // For FormatList: expected child schemas
	AllowedBytes []byte     // For binary fields: set of valid values
	Optional bool           // Whether this field may be absent entirely
}

// MessageSchema defines the expected structure of a specific S/F message.
type MessageSchema struct {
	Stream    byte
	Function  byte
	Name      string    // e.g. "Establish Communication Ack"
	Direction Direction
	WBit      bool
	Body      *ItemSchema // nil = no body expected
	Standard  string      // "E30", "E87", "E40", etc.
}

type sfKey struct {
	stream, function byte
}

// SchemaRegistry holds all known message schemas.
type SchemaRegistry struct {
	schemas map[sfKey]*MessageSchema
}

// NewSchemaRegistry creates an empty registry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{schemas: make(map[sfKey]*MessageSchema)}
}

// Register adds a message schema.
func (r *SchemaRegistry) Register(s *MessageSchema) {
	r.schemas[sfKey{s.Stream, s.Function}] = s
}

// Get returns the schema for a stream/function pair, or nil.
func (r *SchemaRegistry) Get(stream, function byte) *MessageSchema {
	return r.schemas[sfKey{stream, function}]
}

// All returns all registered schemas.
func (r *SchemaRegistry) All() []*MessageSchema {
	result := make([]*MessageSchema, 0, len(r.schemas))
	for _, s := range r.schemas {
		result = append(result, s)
	}
	return result
}

// Count returns the number of registered schemas.
func (r *SchemaRegistry) Count() int {
	return len(r.schemas)
}

// ValidateMessage checks an actual SECS-II item against the schema for the
// given stream/function. Returns nil results if no schema is registered.
func (r *SchemaRegistry) ValidateMessage(stream, function byte, body *secs2.Item) []ValidationResult {
	schema := r.Get(stream, function)
	if schema == nil {
		return nil
	}
	if schema.Body == nil {
		if body != nil && body.Len() > 0 {
			return []ValidationResult{{
				Level:   LevelWarn,
				Path:    "",
				Message: fmt.Sprintf("S%dF%d: body present but not expected", stream, function),
			}}
		}
		return []ValidationResult{{Level: LevelPass, Message: fmt.Sprintf("S%dF%d: no body expected (OK)", stream, function)}}
	}
	if body == nil {
		return []ValidationResult{{
			Level:   LevelFail,
			Path:    "",
			Message: fmt.Sprintf("S%dF%d: body missing", stream, function),
		}}
	}
	return validateItem(schema.Body, body, "")
}

func validateItem(schema *ItemSchema, item *secs2.Item, path string) []ValidationResult {
	var results []ValidationResult

	// Format check
	if item.Format() != schema.Format {
		results = append(results, ValidationResult{
			Level:   LevelFail,
			Path:    path,
			Message: fmt.Sprintf("%s: expected %s, got %s", nameOrPath(schema.Name, path), schema.Format, item.Format()),
		})
		return results
	}

	// Length check
	length := item.Len()
	if schema.Format == FormatList {
		length = len(item.Items())
	}

	if schema.MinLen > 0 && length < schema.MinLen {
		results = append(results, ValidationResult{
			Level:   LevelFail,
			Path:    path,
			Message: fmt.Sprintf("%s: too few elements (%d < %d)", nameOrPath(schema.Name, path), length, schema.MinLen),
		})
		return results
	}
	if schema.MaxLen >= 0 && length > schema.MaxLen {
		results = append(results, ValidationResult{
			Level:   LevelFail,
			Path:    path,
			Message: fmt.Sprintf("%s: too many elements (%d > %d)", nameOrPath(schema.Name, path), length, schema.MaxLen),
		})
		return results
	}

	// Binary value range check
	if schema.Format == secs2.FormatBinary && len(schema.AllowedBytes) > 0 {
		data, err := item.ToBinary()
		if err == nil && len(data) > 0 {
			val := data[0]
			found := false
			for _, b := range schema.AllowedBytes {
				if val == b {
					found = true
					break
				}
			}
			if !found {
				results = append(results, ValidationResult{
					Level:   LevelFail,
					Path:    path,
					Message: fmt.Sprintf("%s: value 0x%02X not in allowed set %v", nameOrPath(schema.Name, path), val, schema.AllowedBytes),
				})
				return results
			}
		}
	}

	// Recurse into list children
	if schema.Format == secs2.FormatList && len(schema.Children) > 0 {
		children := item.Items()
		for i, childSchema := range schema.Children {
			if i >= len(children) {
				if !childSchema.Optional {
					results = append(results, ValidationResult{
						Level:   LevelWarn,
						Path:    childPath(path, i),
						Message: fmt.Sprintf("%s: missing (optional=%v)", nameOrPath(childSchema.Name, childPath(path, i)), childSchema.Optional),
					})
				}
				continue
			}
			results = append(results, validateItem(childSchema, children[i], childPath(path, i))...)
		}
	}

	// If nothing failed, add a pass result
	if len(results) == 0 {
		results = append(results, ValidationResult{
			Level:   LevelPass,
			Path:    path,
			Message: fmt.Sprintf("%s: valid", nameOrPath(schema.Name, path)),
		})
	}

	return results
}

func nameOrPath(name, path string) string {
	if name != "" {
		return name
	}
	if path != "" {
		return path
	}
	return "root"
}

func childPath(parent string, index int) string {
	return fmt.Sprintf("%s[%d]", parent, index)
}

// MaxLevel returns the highest severity level from a set of results.
func MaxLevel(results []ValidationResult) Level {
	max := LevelPass
	for _, r := range results {
		if r.Level > max {
			max = r.Level
		}
	}
	return max
}

// FormatResults returns a compact multi-line summary of validation results.
func FormatResults(results []ValidationResult) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("[%s] %s\n", r.Level, r.Message))
	}
	return b.String()
}

// DefaultRegistry returns a SchemaRegistry populated with standard SEMI
// message schemas (E30, E87, E40). These schemas describe the expected
// structure of each S/F message body.
func DefaultRegistry() *SchemaRegistry {
	r := NewSchemaRegistry()

	// --- S1: Equipment Status ---

	// S1F1: Are You There (no body)
	r.Register(&MessageSchema{Stream: 1, Function: 1, Name: "Are You There", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S1F2: On Line Data — L:2 { A MDLN, A SOFTREV }
	r.Register(&MessageSchema{Stream: 1, Function: 2, Name: "On Line Data", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S1F2", MinLen: 2, MaxLen: 2, Children: []*ItemSchema{
			{Format: secs2.FormatASCII, Name: "MDLN", MinLen: 0, MaxLen: -1},
			{Format: secs2.FormatASCII, Name: "SOFTREV", MinLen: 0, MaxLen: -1},
		}},
	})

	// S1F3: Selected Equipment Status Request — L:n { U4 SVID... } or empty
	r.Register(&MessageSchema{Stream: 1, Function: 3, Name: "Selected Equipment Status Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S1F4: Selected Equipment Status Data — L:n { V... }
	r.Register(&MessageSchema{Stream: 1, Function: 4, Name: "Selected Equipment Status Data", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S1F4", MinLen: 0, MaxLen: -1},
	})

	// S1F11: SV Namelist Request
	r.Register(&MessageSchema{Stream: 1, Function: 11, Name: "Status Variable Namelist Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S1F12: SV Namelist Reply — L:n { L:3 { U4 SVID, A SVNAME, A UNITS } }
	r.Register(&MessageSchema{Stream: 1, Function: 12, Name: "Status Variable Namelist Reply", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S1F12", MinLen: 0, MaxLen: -1},
	})

	// S1F13: Establish Communication Request — L:2 { A MDLN, A SOFTREV } or empty
	r.Register(&MessageSchema{Stream: 1, Function: 13, Name: "Establish Communication Request", Direction: Bidirectional, WBit: true, Standard: "E30"})

	// S1F14: Establish Communication Ack — L:2 { B:1 COMMACK, L:2 { A MDLN, A SOFTREV } }
	r.Register(&MessageSchema{Stream: 1, Function: 14, Name: "Establish Communication Ack", Direction: Bidirectional, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S1F14", MinLen: 2, MaxLen: 2, Children: []*ItemSchema{
			{Format: secs2.FormatBinary, Name: "COMMACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
			{Format: secs2.FormatList, Name: "MDLN/SOFTREV", MinLen: 0, MaxLen: 2, Children: []*ItemSchema{
				{Format: secs2.FormatASCII, Name: "MDLN", MinLen: 0, MaxLen: -1, Optional: true},
				{Format: secs2.FormatASCII, Name: "SOFTREV", MinLen: 0, MaxLen: -1, Optional: true},
			}},
		}},
	})

	// S1F15: Request OFF-LINE
	r.Register(&MessageSchema{Stream: 1, Function: 15, Name: "Request OFF-LINE", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S1F16: OFF-LINE Ack — B:1 OFLACK
	r.Register(&MessageSchema{Stream: 1, Function: 16, Name: "OFF-LINE Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "OFLACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
	})

	// S1F17: Request ON-LINE
	r.Register(&MessageSchema{Stream: 1, Function: 17, Name: "Request ON-LINE", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S1F18: ON-LINE Ack — B:1 ONLACK
	r.Register(&MessageSchema{Stream: 1, Function: 18, Name: "ON-LINE Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "ONLACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01, 0x02}},
	})

	// --- S2: Equipment Control ---

	// S2F13: Equipment Constant Request — L:n { U4 ECID } or empty
	r.Register(&MessageSchema{Stream: 2, Function: 13, Name: "Equipment Constant Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S2F14: Equipment Constant Data — L:n { V... }
	r.Register(&MessageSchema{Stream: 2, Function: 14, Name: "Equipment Constant Data", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F14", MinLen: 0, MaxLen: -1},
	})

	// S2F15: New Equipment Constant Send — L:n { L:2 { U4 ECID, V ECV } }
	r.Register(&MessageSchema{Stream: 2, Function: 15, Name: "New Equipment Constant Send", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F15", MinLen: 0, MaxLen: -1},
	})

	// S2F16: New Equipment Constant Ack — B:1 EAC
	r.Register(&MessageSchema{Stream: 2, Function: 16, Name: "New Equipment Constant Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "EAC", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01, 0x02, 0x03}},
	})

	// S2F29: EC Namelist Request
	r.Register(&MessageSchema{Stream: 2, Function: 29, Name: "Equipment Constant Namelist Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S2F30: EC Namelist Reply — L:n { L:6 { U4 ECID, A ECNAME, V ECMIN, V ECMAX, V ECDEF, A UNITS } }
	r.Register(&MessageSchema{Stream: 2, Function: 30, Name: "Equipment Constant Namelist Reply", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F30", MinLen: 0, MaxLen: -1},
	})

	// S2F33: Define Report — L:2 { U4 DATAID, L:n { L:2 { U4 RPTID, L:m { U4 VID } } } }
	r.Register(&MessageSchema{Stream: 2, Function: 33, Name: "Define Report", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F33", MinLen: 2, MaxLen: 2},
	})

	// S2F34: Define Report Ack — B:1 DRACK
	r.Register(&MessageSchema{Stream: 2, Function: 34, Name: "Define Report Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "DRACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}},
	})

	// S2F35: Link Event Report — L:2 { U4 DATAID, L:n { L:2 { U4 CEID, L:m { U4 RPTID } } } }
	r.Register(&MessageSchema{Stream: 2, Function: 35, Name: "Link Event Report", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F35", MinLen: 2, MaxLen: 2},
	})

	// S2F36: Link Event Report Ack — B:1 LRACK
	r.Register(&MessageSchema{Stream: 2, Function: 36, Name: "Link Event Report Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "LRACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}},
	})

	// S2F37: Enable/Disable Event Report — L:2 { BOOLEAN CEED, L:n { U4 CEID } }
	r.Register(&MessageSchema{Stream: 2, Function: 37, Name: "Enable/Disable Event Report", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F37", MinLen: 2, MaxLen: 2},
	})

	// S2F38: Enable/Disable Event Report Ack — B:1 ERACK
	r.Register(&MessageSchema{Stream: 2, Function: 38, Name: "Enable/Disable Event Report Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "ERACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
	})

	// S2F41: Host Command Send — L:2 { A RCMD, L:n { L:2 { A CPNAME, V CPVAL } } }
	r.Register(&MessageSchema{Stream: 2, Function: 41, Name: "Host Command Send", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F41", MinLen: 2, MaxLen: 2, Children: []*ItemSchema{
			{Format: secs2.FormatASCII, Name: "RCMD", MinLen: 1, MaxLen: -1},
			{Format: secs2.FormatList, Name: "PARAMS", MinLen: 0, MaxLen: -1},
		}},
	})

	// S2F42: Host Command Ack — L:2 { B:1 HCACK, L:n { L:2 { A CPNAME, B:1 CPACK } } }
	r.Register(&MessageSchema{Stream: 2, Function: 42, Name: "Host Command Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S2F42", MinLen: 2, MaxLen: 2, Children: []*ItemSchema{
			{Format: secs2.FormatBinary, Name: "HCACK", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}},
			{Format: secs2.FormatList, Name: "CPACK_LIST", MinLen: 0, MaxLen: -1},
		}},
	})

	// --- S3: Material Status (E87) ---

	// S3F17: Carrier Action Request — L:5 { A COMMAND, A CARRIERID, U1 PORTID, U4 DATAID, L:n }
	r.Register(&MessageSchema{Stream: 3, Function: 17, Name: "Carrier Action Request", Direction: HostToEquip, WBit: true, Standard: "E87",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S3F17", MinLen: 3, MaxLen: -1},
	})

	// S3F18: Carrier Action Ack — L:2 { B:1 CAACK, L:n { L:2 { A ERRCODE, A ERRTEXT } } }
	r.Register(&MessageSchema{Stream: 3, Function: 18, Name: "Carrier Action Ack", Direction: EquipToHost, Standard: "E87",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S3F18", MinLen: 2, MaxLen: 2},
	})

	// --- S5: Alarm Management ---

	// S5F1: Alarm Report — L:3 { B:1 ALCD, U4 ALID, A ALTX }
	r.Register(&MessageSchema{Stream: 5, Function: 1, Name: "Alarm Report Send", Direction: EquipToHost, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S5F1", MinLen: 3, MaxLen: 3, Children: []*ItemSchema{
			{Format: secs2.FormatBinary, Name: "ALCD", MinLen: 1, MaxLen: 1},
			{Format: secs2.FormatU4, Name: "ALID", MinLen: 1, MaxLen: 1},
			{Format: secs2.FormatASCII, Name: "ALTX", MinLen: 0, MaxLen: -1},
		}},
	})

	// S5F2: Alarm Report Ack — B:1 ACKC5
	r.Register(&MessageSchema{Stream: 5, Function: 2, Name: "Alarm Report Ack", Direction: HostToEquip, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "ACKC5", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
	})

	// S5F3: Enable/Disable Alarm Send — L:2 { B:1 ALED, U4 ALID }
	r.Register(&MessageSchema{Stream: 5, Function: 3, Name: "Enable/Disable Alarm Send", Direction: HostToEquip, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S5F3", MinLen: 2, MaxLen: 2, Children: []*ItemSchema{
			{Format: secs2.FormatBinary, Name: "ALED", MinLen: 1, MaxLen: 1},
			{Format: secs2.FormatU4, Name: "ALID", MinLen: 1, MaxLen: 1},
		}},
	})

	// S5F4: Enable/Disable Alarm Ack — B:1 ACKC5
	r.Register(&MessageSchema{Stream: 5, Function: 4, Name: "Enable/Disable Alarm Ack", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "ACKC5", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
	})

	// S5F5: List Alarms Request (no body)
	r.Register(&MessageSchema{Stream: 5, Function: 5, Name: "List Alarms Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S5F6: List Alarms Data — L:n { L:3 { B:1 ALCD, U4 ALID, A ALTX } }
	r.Register(&MessageSchema{Stream: 5, Function: 6, Name: "List Alarms Data", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S5F6", MinLen: 0, MaxLen: -1},
	})

	// S5F7: List Enabled Alarms Request (no body)
	r.Register(&MessageSchema{Stream: 5, Function: 7, Name: "List Enabled Alarms Request", Direction: HostToEquip, WBit: true, Standard: "E30"})

	// S5F8: List Enabled Alarms Data — L:n { L:3 { B:1 ALCD, U4 ALID, A ALTX } }
	r.Register(&MessageSchema{Stream: 5, Function: 8, Name: "List Enabled Alarms Data", Direction: EquipToHost, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S5F8", MinLen: 0, MaxLen: -1},
	})

	// --- S6: Data Collection ---

	// S6F11: Event Report Send — L:3 { U4 DATAID, U4 CEID, L:n { L:2 { U4 RPTID, L:m { V } } } }
	r.Register(&MessageSchema{Stream: 6, Function: 11, Name: "Event Report Send", Direction: EquipToHost, WBit: true, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S6F11", MinLen: 3, MaxLen: 3, Children: []*ItemSchema{
			{Format: secs2.FormatU4, Name: "DATAID", MinLen: 1, MaxLen: 1},
			{Format: secs2.FormatU4, Name: "CEID", MinLen: 1, MaxLen: 1},
			{Format: secs2.FormatList, Name: "RPT_LIST", MinLen: 0, MaxLen: -1},
		}},
	})

	// S6F12: Event Report Ack — B:1 ACKC6
	r.Register(&MessageSchema{Stream: 6, Function: 12, Name: "Event Report Ack", Direction: HostToEquip, Standard: "E30",
		Body: &ItemSchema{Format: secs2.FormatBinary, Name: "ACKC6", MinLen: 1, MaxLen: 1, AllowedBytes: []byte{0x00, 0x01}},
	})

	// --- S16: Process Job (E40) ---

	// S16F11: Process Job Create — L:5 { A DATAID, A PRJOBID, U1 MF, L:n { A CARRIERID }, L:n { L:2 { A PNAME, V PVAL } } }
	r.Register(&MessageSchema{Stream: 16, Function: 11, Name: "Process Job Create Request", Direction: HostToEquip, WBit: true, Standard: "E40",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S16F11", MinLen: 3, MaxLen: -1},
	})

	// S16F12: Process Job Create Ack — L:2 { B:1 ACKA, L:n { L:2 { A ERRCODE, A ERRTEXT } } }
	r.Register(&MessageSchema{Stream: 16, Function: 12, Name: "Process Job Create Ack", Direction: EquipToHost, Standard: "E40",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S16F12", MinLen: 2, MaxLen: 2},
	})

	// S16F15: Process Job Command — L:2 { A PRJOBID, B:1 PRCMDNAME }
	r.Register(&MessageSchema{Stream: 16, Function: 15, Name: "Process Job Command Request", Direction: HostToEquip, WBit: true, Standard: "E40",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S16F15", MinLen: 2, MaxLen: 2},
	})

	// S16F16: Process Job Command Ack — L:2 { B:1 ACKA, L:n { L:2 { A ERRCODE, A ERRTEXT } } }
	r.Register(&MessageSchema{Stream: 16, Function: 16, Name: "Process Job Command Ack", Direction: EquipToHost, Standard: "E40",
		Body: &ItemSchema{Format: secs2.FormatList, Name: "S16F16", MinLen: 2, MaxLen: 2},
	})

	return r
}

// FormatList re-exported for external use without importing secs2 directly.
const FormatList = secs2.FormatList
