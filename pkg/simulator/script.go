package simulator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"gopkg.in/yaml.v3"
)

// Script represents a test scenario loaded from YAML.
type Script struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Steps       []ScriptStep `yaml:"steps"`
}

// ScriptStep is one action in a scenario.
type ScriptStep struct {
	Action   string        `yaml:"action"`   // "send", "delay", "assert_reply"
	Stream   byte          `yaml:"stream"`
	Function byte          `yaml:"function"`
	WBit     bool          `yaml:"wbit"`
	Body     string        `yaml:"body"`     // SML notation
	Delay    time.Duration `yaml:"delay"`    // For delay action
	Expect   string        `yaml:"expect"`   // Expected format check
}

// StepResult records the outcome of a single script step.
type StepResult struct {
	Step   int    `json:"step"`
	Action string `json:"action"`
	Status string `json:"status"` // "pass", "fail", "skip"
	Detail string `json:"detail"`
}

// ScriptResult records the outcome of running a complete script.
type ScriptResult struct {
	Name     string        `json:"name"`
	Steps    []StepResult  `json:"steps"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Duration time.Duration `json:"duration"`
}

// LoadScript parses YAML bytes into a Script.
func LoadScript(data []byte) (*Script, error) {
	var s Script
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("script: parse: %w", err)
	}
	return &s, nil
}

// LoadScriptFile reads and parses a YAML script file.
func LoadScriptFile(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("script: read %s: %w", path, err)
	}
	return LoadScript(data)
}

// ScriptRunner executes scripts against a Host simulator.
type ScriptRunner struct {
	host   *Host
	logger *slog.Logger
}

// NewScriptRunner creates a script runner.
func NewScriptRunner(host *Host, logger *slog.Logger) *ScriptRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScriptRunner{host: host, logger: logger}
}

// Run executes a script and returns the results.
func (sr *ScriptRunner) Run(ctx context.Context, script *Script) *ScriptResult {
	start := time.Now()
	result := &ScriptResult{Name: script.Name}

	var lastReply *secs2.Item

	for i, step := range script.Steps {
		sr.logger.Info("Script step", "step", i+1, "action", step.Action)

		var sr2 StepResult
		sr2.Step = i + 1
		sr2.Action = step.Action

		switch step.Action {
		case "send":
			var body *secs2.Item
			if step.Body != "" {
				var err error
				body, err = ParseSML(step.Body)
				if err != nil {
					sr2.Status = "fail"
					sr2.Detail = fmt.Sprintf("SML parse error: %s", err)
					result.Steps = append(result.Steps, sr2)
					result.Failed++
					continue
				}
			}
			reply, err := sr.host.SendRaw(ctx, step.Stream, step.Function, step.WBit, body)
			if err != nil {
				sr2.Status = "fail"
				sr2.Detail = fmt.Sprintf("send error: %s", err)
				result.Failed++
			} else {
				sr2.Status = "pass"
				sr2.Detail = fmt.Sprintf("S%dF%d sent OK", step.Stream, step.Function)
				lastReply = reply
				result.Passed++
			}

		case "delay":
			d := step.Delay
			if d <= 0 {
				d = 100 * time.Millisecond
			}
			select {
			case <-time.After(d):
				sr2.Status = "pass"
				sr2.Detail = fmt.Sprintf("waited %s", d)
				result.Passed++
			case <-ctx.Done():
				sr2.Status = "fail"
				sr2.Detail = "context cancelled during delay"
				result.Failed++
			}

		case "assert_reply":
			if lastReply == nil {
				sr2.Status = "fail"
				sr2.Detail = "no reply to assert"
				result.Failed++
			} else if step.Expect != "" {
				// Check format matches
				actual := lastReply.Format().String()
				if actual == step.Expect || step.Expect == "*" {
					sr2.Status = "pass"
					sr2.Detail = fmt.Sprintf("reply format=%s matches", actual)
					result.Passed++
				} else {
					sr2.Status = "fail"
					sr2.Detail = fmt.Sprintf("expected format %s, got %s", step.Expect, actual)
					result.Failed++
				}
			} else {
				sr2.Status = "pass"
				sr2.Detail = "reply exists"
				result.Passed++
			}

		default:
			sr2.Status = "skip"
			sr2.Detail = fmt.Sprintf("unknown action: %s", step.Action)
		}

		result.Steps = append(result.Steps, sr2)
	}

	result.Duration = time.Since(start)
	return result
}

// --- SML Parser ---
// ParseSML parses a simplified SML (SECS Message Language) notation into a secs2.Item.
// Supported formats:
//   L:n { ... }          — List with n children
//   A "string"           — ASCII string
//   B 0xFF               — Binary byte(s)
//   U1/U2/U4/U8 123     — Unsigned integer
//   I1/I2/I4/I8 -42      — Signed integer
//   F4/F8 3.14           — Float
//   BOOLEAN true/false   — Boolean
func ParseSML(text string) (*secs2.Item, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("sml: empty input")
	}
	p := &smlParser{input: text, pos: 0}
	item, err := p.parseItem()
	if err != nil {
		return nil, fmt.Errorf("sml: %w", err)
	}
	return item, nil
}

type smlParser struct {
	input string
	pos   int
}

func (p *smlParser) skipSpace() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else if ch == '/' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '/' {
			// Skip line comment
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *smlParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *smlParser) readToken() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) && !unicode.IsSpace(rune(p.input[p.pos])) && p.input[p.pos] != '{' && p.input[p.pos] != '}' {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *smlParser) parseItem() (*secs2.Item, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of input")
	}

	token := p.readToken()

	switch {
	case strings.HasPrefix(token, "L"):
		return p.parseList()
	case token == "A":
		return p.parseASCII()
	case token == "B":
		return p.parseBinary()
	case token == "BOOLEAN":
		return p.parseBoolean()
	case token == "U1":
		return p.parseUint(1)
	case token == "U2":
		return p.parseUint(2)
	case token == "U4":
		return p.parseUint(4)
	case token == "U8":
		return p.parseUint(8)
	case token == "I1":
		return p.parseInt(1)
	case token == "I2":
		return p.parseInt(2)
	case token == "I4":
		return p.parseInt(4)
	case token == "I8":
		return p.parseInt(8)
	case token == "F4":
		return p.parseFloat(4)
	case token == "F8":
		return p.parseFloat(8)
	default:
		return nil, fmt.Errorf("unknown format: %q at pos %d", token, p.pos)
	}
}

func (p *smlParser) parseList() (*secs2.Item, error) {
	p.skipSpace()
	if p.peek() != '{' {
		// Empty list: L:0
		return secs2.NewList(), nil
	}
	p.pos++ // skip '{'

	var children []*secs2.Item
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unterminated list")
		}
		if p.peek() == '}' {
			p.pos++
			break
		}
		child, err := p.parseItem()
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return secs2.NewList(children...), nil
}

func (p *smlParser) parseASCII() (*secs2.Item, error) {
	p.skipSpace()
	if p.peek() != '"' {
		return nil, fmt.Errorf("expected '\"' for ASCII value at pos %d", p.pos)
	}
	p.pos++ // skip opening quote
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != '"' {
		if p.input[p.pos] == '\\' {
			p.pos++ // skip escape
		}
		p.pos++
	}
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unterminated string")
	}
	s := p.input[start:p.pos]
	p.pos++ // skip closing quote
	return secs2.NewASCII(s), nil
}

func (p *smlParser) parseBinary() (*secs2.Item, error) {
	p.skipSpace()
	token := p.readToken()
	if strings.HasPrefix(token, "0x") || strings.HasPrefix(token, "0X") {
		val, err := strconv.ParseUint(token[2:], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid binary hex: %s", token)
		}
		return secs2.NewBinary([]byte{byte(val)}), nil
	}
	val, err := strconv.ParseUint(token, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid binary value: %s", token)
	}
	return secs2.NewBinary([]byte{byte(val)}), nil
}

func (p *smlParser) parseBoolean() (*secs2.Item, error) {
	p.skipSpace()
	token := p.readToken()
	switch strings.ToLower(token) {
	case "true":
		return secs2.NewBoolean(true), nil
	case "false":
		return secs2.NewBoolean(false), nil
	default:
		return nil, fmt.Errorf("invalid boolean: %s", token)
	}
}

func (p *smlParser) parseUint(size int) (*secs2.Item, error) {
	p.skipSpace()
	token := p.readToken()
	val, err := strconv.ParseUint(token, 0, size*8)
	if err != nil {
		return nil, fmt.Errorf("invalid U%d: %s", size, token)
	}
	switch size {
	case 1:
		return secs2.NewU1(uint8(val)), nil
	case 2:
		return secs2.NewU2(uint16(val)), nil
	case 4:
		return secs2.NewU4(uint32(val)), nil
	case 8:
		return secs2.NewU8(val), nil
	}
	return nil, fmt.Errorf("unsupported uint size: %d", size)
}

func (p *smlParser) parseInt(size int) (*secs2.Item, error) {
	p.skipSpace()
	token := p.readToken()
	val, err := strconv.ParseInt(token, 0, size*8)
	if err != nil {
		return nil, fmt.Errorf("invalid I%d: %s", size, token)
	}
	switch size {
	case 1:
		return secs2.NewI1(int8(val)), nil
	case 2:
		return secs2.NewI2(int16(val)), nil
	case 4:
		return secs2.NewI4(int32(val)), nil
	case 8:
		return secs2.NewI8(val), nil
	}
	return nil, fmt.Errorf("unsupported int size: %d", size)
}

func (p *smlParser) parseFloat(size int) (*secs2.Item, error) {
	p.skipSpace()
	token := p.readToken()
	val, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid F%d: %s", size, token)
	}
	switch size {
	case 4:
		return secs2.NewF4(float32(val)), nil
	case 8:
		return secs2.NewF8(val), nil
	}
	return nil, fmt.Errorf("unsupported float size: %d", size)
}

// BuiltinScenarios returns the built-in test scenarios.
func BuiltinScenarios() []*Script {
	return []*Script{
		{
			Name:        "E30 Communication Setup",
			Description: "Establish communication and go online",
			Steps: []ScriptStep{
				{Action: "send", Stream: 1, Function: 13, WBit: true, Body: `L:2 { A "HOST" A "1.0.0" }`},
				{Action: "assert_reply", Expect: "L"},
				{Action: "send", Stream: 1, Function: 1, WBit: true},
				{Action: "assert_reply", Expect: "L"},
				{Action: "send", Stream: 1, Function: 17, WBit: true},
				{Action: "assert_reply", Expect: "B"},
			},
		},
		{
			Name:        "Remote Command",
			Description: "Send a remote command after establishing communication",
			Steps: []ScriptStep{
				{Action: "send", Stream: 1, Function: 13, WBit: true, Body: `L:2 { A "HOST" A "1.0.0" }`},
				{Action: "assert_reply", Expect: "L"},
				{Action: "send", Stream: 2, Function: 41, WBit: true, Body: `L:2 { A "START" L:0 }`},
				{Action: "assert_reply", Expect: "L"},
			},
		},
	}
}
