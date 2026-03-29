package validator

import (
	"testing"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
)

func TestDefaultRegistryCount(t *testing.T) {
	r := DefaultRegistry()
	if r.Count() == 0 {
		t.Fatal("DefaultRegistry returned 0 schemas")
	}
	// We register S1F1/F2/F3/F4/F11/F12/F13/F14/F15/F16/F17/F18
	//           + S2F13/F14/F15/F16/F29/F30/F33/F34/F35/F36/F37/F38/F41/F42
	//           + S3F17/F18 + S5F1/F2/F3/F4/F5/F6/F7/F8 + S6F11/F12 + S16F11/F12/F15/F16
	if r.Count() < 30 {
		t.Errorf("expected >= 30 schemas, got %d", r.Count())
	}
}

func TestValidateS1F2_Valid(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewASCII("EQUIP-001"),
		secs2.NewASCII("1.0.0"),
	)
	results := r.ValidateMessage(1, 2, body)
	for _, res := range results {
		if res.Level == LevelFail {
			t.Errorf("unexpected FAIL: %s", res.Message)
		}
	}
}

func TestValidateS1F2_WrongFormat(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewASCII("not a list")
	results := r.ValidateMessage(1, 2, body)
	if MaxLevel(results) != LevelFail {
		t.Error("expected FAIL for wrong format")
	}
}

func TestValidateS1F14_Valid(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x00}),
		secs2.NewList(
			secs2.NewASCII("SIM-01"),
			secs2.NewASCII("1.0"),
		),
	)
	results := r.ValidateMessage(1, 14, body)
	for _, res := range results {
		if res.Level == LevelFail {
			t.Errorf("unexpected FAIL: %s", res.Message)
		}
	}
}

func TestValidateS1F14_BadCOMMACC(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x05}), // invalid COMMACK
		secs2.NewList(),
	)
	results := r.ValidateMessage(1, 14, body)
	if MaxLevel(results) != LevelFail {
		t.Error("expected FAIL for bad COMMACK value")
	}
}

func TestValidateS2F42_Valid(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x00}),
		secs2.NewList(),
	)
	results := r.ValidateMessage(2, 42, body)
	for _, res := range results {
		if res.Level == LevelFail {
			t.Errorf("unexpected FAIL: %s", res.Message)
		}
	}
}

func TestValidateS2F42_BadHCACK(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x09}), // HCACK out of range
		secs2.NewList(),
	)
	results := r.ValidateMessage(2, 42, body)
	if MaxLevel(results) != LevelFail {
		t.Error("expected FAIL for HCACK=9")
	}
}

func TestValidateS5F1_Valid(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x80}),
		secs2.NewU4(3),
		secs2.NewASCII("Temperature High"),
	)
	results := r.ValidateMessage(5, 1, body)
	for _, res := range results {
		if res.Level == LevelFail {
			t.Errorf("unexpected FAIL: %s", res.Message)
		}
	}
}

func TestValidateS6F11_Valid(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewU4(1),
		secs2.NewU4(100),
		secs2.NewList(), // empty report list
	)
	results := r.ValidateMessage(6, 11, body)
	for _, res := range results {
		if res.Level == LevelFail {
			t.Errorf("unexpected FAIL: %s", res.Message)
		}
	}
}

func TestValidateS1F1_NoBody(t *testing.T) {
	r := DefaultRegistry()
	results := r.ValidateMessage(1, 1, nil)
	// S1F1 has no body expected, nil body should pass
	if MaxLevel(results) == LevelFail {
		t.Error("S1F1 with nil body should not fail")
	}
}

func TestValidateUnknownSF(t *testing.T) {
	r := DefaultRegistry()
	results := r.ValidateMessage(99, 99, nil)
	if results != nil {
		t.Error("unknown S/F should return nil results")
	}
}

func TestValidateS1F14_MissingBody(t *testing.T) {
	r := DefaultRegistry()
	results := r.ValidateMessage(1, 14, nil)
	if MaxLevel(results) != LevelFail {
		t.Error("S1F14 with nil body should fail")
	}
}

func TestValidateS5F1_TooFewChildren(t *testing.T) {
	r := DefaultRegistry()
	body := secs2.NewList(
		secs2.NewBinary([]byte{0x80}),
		secs2.NewU4(3),
		// missing ALTX
	)
	results := r.ValidateMessage(5, 1, body)
	if MaxLevel(results) != LevelFail {
		t.Error("S5F1 with 2 children (expected 3) should fail")
	}
}

func TestFormatResults(t *testing.T) {
	results := []ValidationResult{
		{Level: LevelPass, Message: "OK"},
		{Level: LevelFail, Message: "bad"},
	}
	s := FormatResults(results)
	if s == "" {
		t.Error("FormatResults returned empty string")
	}
}
