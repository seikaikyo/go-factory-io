package gem

import (
	"log/slog"
	"testing"

	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	logger := slog.Default()
	cfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	session := hsms.NewSession(cfg, logger)
	return NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)
}

// TestDefaultPolicyIsMonitor pins the default-deny posture: a freshly created
// handler must not accept the messages that change equipment state.
func TestDefaultPolicyIsMonitor(t *testing.T) {
	h := newTestHandler(t)

	if h.Policy() == nil {
		t.Fatal("a new handler must carry a policy; nil means allow-all")
	}

	tests := []struct {
		name             string
		stream, function byte
		wantAllowed      bool
	}{
		// Reads and handshake stay open.
		{"S1F1 are you there", 1, 1, true},
		{"S1F3 status request", 1, 3, true},
		{"S1F11 SV namelist", 1, 11, true},
		{"S1F13 establish comm", 1, 13, true},
		{"S2F13 EC request", 2, 13, true},
		{"S5F5 list alarms", 5, 5, true},

		// State-changing messages are denied.
		{"S1F15 request offline", 1, 15, false},
		{"S1F17 request online", 1, 17, false},
		{"S2F15 set EC", 2, 15, false},
		{"S2F33 define report", 2, 33, false},
		{"S2F35 link event report", 2, 35, false},
		{"S2F37 enable event", 2, 37, false},
		{"S2F41 remote command", 2, 41, false},
		{"S5F3 enable/disable alarm", 5, 3, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := h.Policy().IsAllowed(tc.stream, tc.function)
			if got != tc.wantAllowed {
				t.Errorf("S%dF%d allowed = %v, want %v", tc.stream, tc.function, got, tc.wantAllowed)
			}
		})
	}
}

// TestFullAccessPolicyOptIn checks that the documented escape hatch works and
// that it is the only way to reach the write messages.
func TestFullAccessPolicyOptIn(t *testing.T) {
	h := newTestHandler(t)

	writes := []struct {
		stream, function byte
	}{
		{1, 15}, {1, 17}, {2, 15}, {2, 33}, {2, 35}, {2, 37}, {2, 41}, {5, 3},
	}

	for _, w := range writes {
		if h.Policy().IsAllowed(w.stream, w.function) {
			t.Fatalf("S%dF%d allowed before opting in", w.stream, w.function)
		}
	}

	h.SetPolicy(security.FullAccessPolicy())

	for _, w := range writes {
		if !h.Policy().IsAllowed(w.stream, w.function) {
			t.Errorf("S%dF%d denied after SetPolicy(FullAccessPolicy)", w.stream, w.function)
		}
	}
}

// TestReadOnlyPolicyOptIn covers the middle setting, which allows any read but
// blocks the write list.
func TestReadOnlyPolicyOptIn(t *testing.T) {
	h := newTestHandler(t)
	h.SetPolicy(security.ReadOnlyPolicy())

	if !h.Policy().IsAllowed(6, 11) {
		t.Error("read-only policy should allow S6F11 event reports")
	}
	if h.Policy().IsAllowed(2, 41) {
		t.Error("read-only policy must still deny S2F41 remote command")
	}
}
