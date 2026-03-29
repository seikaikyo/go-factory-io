package simulator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
)

func TestParseSML_List(t *testing.T) {
	item, err := ParseSML(`L:2 { A "HOST" A "1.0.0" }`)
	if err != nil {
		t.Fatal(err)
	}
	if item.Format() != secs2.FormatList {
		t.Fatalf("expected List, got %s", item.Format())
	}
	if len(item.Items()) != 2 {
		t.Fatalf("expected 2 children, got %d", len(item.Items()))
	}
	s, _ := item.ItemAt(0).ToASCII()
	if s != "HOST" {
		t.Errorf("expected HOST, got %s", s)
	}
}

func TestParseSML_EmptyList(t *testing.T) {
	item, err := ParseSML(`L:0`)
	if err != nil {
		t.Fatal(err)
	}
	if item.Format() != secs2.FormatList {
		t.Fatalf("expected List, got %s", item.Format())
	}
	if len(item.Items()) != 0 {
		t.Errorf("expected empty list, got %d items", len(item.Items()))
	}
}

func TestParseSML_Binary(t *testing.T) {
	item, err := ParseSML(`B 0x00`)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := item.ToBinary()
	if len(data) != 1 || data[0] != 0x00 {
		t.Errorf("expected [0x00], got %v", data)
	}
}

func TestParseSML_U4(t *testing.T) {
	item, err := ParseSML(`U4 42`)
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := item.ToUint64s()
	if len(vals) != 1 || vals[0] != 42 {
		t.Errorf("expected [42], got %v", vals)
	}
}

func TestParseSML_I4(t *testing.T) {
	item, err := ParseSML(`I4 -7`)
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := item.ToInt64s()
	if len(vals) != 1 || vals[0] != -7 {
		t.Errorf("expected [-7], got %v", vals)
	}
}

func TestParseSML_F8(t *testing.T) {
	item, err := ParseSML(`F8 3.14`)
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := item.ToFloat64s()
	if len(vals) != 1 || vals[0] != 3.14 {
		t.Errorf("expected [3.14], got %v", vals)
	}
}

func TestParseSML_Boolean(t *testing.T) {
	item, err := ParseSML(`BOOLEAN true`)
	if err != nil {
		t.Fatal(err)
	}
	vals, _ := item.ToBooleans()
	if len(vals) != 1 || !vals[0] {
		t.Errorf("expected [true], got %v", vals)
	}
}

func TestParseSML_NestedList(t *testing.T) {
	item, err := ParseSML(`L:2 { B 0x00 L:2 { A "SIM" A "1.0" } }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Items()) != 2 {
		t.Fatalf("expected 2 children, got %d", len(item.Items()))
	}
	nested := item.ItemAt(1)
	if nested.Format() != secs2.FormatList {
		t.Fatalf("expected nested list, got %s", nested.Format())
	}
}

func TestParseSML_WithComments(t *testing.T) {
	item, err := ParseSML(`L:2 {
		A "HOST" // model name
		A "1.0"  // software rev
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Items()) != 2 {
		t.Errorf("expected 2 children, got %d", len(item.Items()))
	}
}

func TestParseSML_Error(t *testing.T) {
	_, err := ParseSML("")
	if err == nil {
		t.Error("expected error for empty input")
	}
	_, err = ParseSML("UNKNOWN 123")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestLoadScript(t *testing.T) {
	yaml := `
name: test
description: a test script
steps:
  - action: send
    stream: 1
    function: 13
    wbit: true
    body: 'L:2 { A "HOST" A "1.0" }'
  - action: assert_reply
    expect: "L"
  - action: delay
    delay: 10ms
`
	script, err := LoadScript([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	if script.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", script.Name)
	}
	if len(script.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(script.Steps))
	}
}

func TestScriptRunner_BuiltinE30(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()
	time.Sleep(50 * time.Millisecond)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	defer host.Close()

	runner := NewScriptRunner(host, logger)
	scenarios := BuiltinScenarios()
	result := runner.Run(ctx, scenarios[0]) // E30 Communication Setup

	if result.Failed > 0 {
		for _, s := range result.Steps {
			if s.Status == "fail" {
				t.Errorf("step %d (%s) failed: %s", s.Step, s.Action, s.Detail)
			}
		}
	}
	if result.Passed == 0 {
		t.Error("expected at least one passed step")
	}
}

func TestBuiltinScenarios(t *testing.T) {
	scenarios := BuiltinScenarios()
	if len(scenarios) < 2 {
		t.Errorf("expected at least 2 built-in scenarios, got %d", len(scenarios))
	}
	for _, s := range scenarios {
		if s.Name == "" {
			t.Error("scenario has empty name")
		}
		if len(s.Steps) == 0 {
			t.Errorf("scenario %s has no steps", s.Name)
		}
	}
}
