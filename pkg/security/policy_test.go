package security

import "testing"

func TestPolicyAllowAll(t *testing.T) {
	var p *SessionPolicy // nil = allow all
	if !p.IsAllowed(1, 1) {
		t.Error("nil policy should allow all")
	}
	if !p.IsAllowed(2, 41) {
		t.Error("nil policy should allow S2F41")
	}
}

func TestPolicyReadOnly(t *testing.T) {
	p := ReadOnlyPolicy()

	// Read operations should pass
	reads := []SFPair{{1, 1}, {1, 3}, {1, 11}, {1, 13}, {2, 13}, {2, 29}, {5, 5}, {5, 7}, {6, 11}}
	for _, sf := range reads {
		if !p.IsAllowed(sf.Stream, sf.Function) {
			t.Errorf("ReadOnly should allow %s", sf)
		}
	}

	// Write operations should be blocked
	writes := []SFPair{{2, 15}, {2, 33}, {2, 35}, {2, 37}, {2, 41}, {1, 15}, {1, 17}, {5, 3}}
	for _, sf := range writes {
		if p.IsAllowed(sf.Stream, sf.Function) {
			t.Errorf("ReadOnly should deny %s", sf)
		}
	}
}

func TestPolicyMonitor(t *testing.T) {
	p := MonitorPolicy()

	// Allowed
	if !p.IsAllowed(1, 1) {
		t.Error("monitor should allow S1F1")
	}
	if !p.IsAllowed(1, 3) {
		t.Error("monitor should allow S1F3")
	}
	if !p.IsAllowed(2, 13) {
		t.Error("monitor should allow S2F13")
	}

	// Denied (not in allowlist)
	if p.IsAllowed(2, 41) {
		t.Error("monitor should deny S2F41")
	}
	if p.IsAllowed(2, 15) {
		t.Error("monitor should deny S2F15")
	}
	if p.IsAllowed(6, 11) {
		t.Error("monitor should deny S6F11 (not in allowlist)")
	}
}

func TestPolicyDenyList(t *testing.T) {
	p := &SessionPolicy{
		DeniedMessages: []SFPair{{2, 41}}, // Only deny RCMD
	}

	if p.IsAllowed(2, 41) {
		t.Error("should deny S2F41")
	}
	if !p.IsAllowed(2, 15) {
		t.Error("should allow S2F15")
	}
	if !p.IsAllowed(1, 1) {
		t.Error("should allow S1F1")
	}
}

func TestPolicyAllowList(t *testing.T) {
	p := &SessionPolicy{
		AllowedMessages: []SFPair{{1, 1}, {1, 13}},
	}

	if !p.IsAllowed(1, 1) {
		t.Error("should allow S1F1")
	}
	if !p.IsAllowed(1, 13) {
		t.Error("should allow S1F13")
	}
	if p.IsAllowed(2, 41) {
		t.Error("should deny S2F41 (not in allowlist)")
	}
}

func TestFullAccessPolicy(t *testing.T) {
	p := FullAccessPolicy()
	if p != nil {
		t.Error("FullAccessPolicy should return nil")
	}
}
