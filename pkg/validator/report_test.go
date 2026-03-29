package validator

import "testing"

func TestGenerateReport(t *testing.T) {
	registry := DefaultRegistry()
	report := GenerateReport(registry)

	if report.TotalExpected == 0 {
		t.Fatal("expected non-zero total")
	}
	if report.TotalHandled == 0 {
		t.Fatal("expected non-zero handled")
	}
	if report.Percentage <= 0 {
		t.Fatal("expected positive coverage percentage")
	}
	if len(report.Standards) == 0 {
		t.Fatal("expected at least one standard")
	}
	if len(report.SFDetail) == 0 {
		t.Fatal("expected at least one S/F detail")
	}

	// E30 should have high coverage since most S1/S2/S5/S6 are handled
	for _, sc := range report.Standards {
		if sc.Standard == "E30" && sc.Percentage < 50 {
			t.Errorf("E30 coverage too low: %.1f%%", sc.Percentage)
		}
	}
}

func TestGenerateReport_HasAllStandards(t *testing.T) {
	registry := DefaultRegistry()
	report := GenerateReport(registry)

	standards := make(map[string]bool)
	for _, sc := range report.Standards {
		standards[sc.Standard] = true
	}

	expected := []string{"E30", "E40", "E87"}
	for _, e := range expected {
		if !standards[e] {
			t.Errorf("missing standard: %s", e)
		}
	}
}

func TestGenerateReport_SFDetailSorted(t *testing.T) {
	registry := DefaultRegistry()
	report := GenerateReport(registry)

	for i := 1; i < len(report.SFDetail); i++ {
		prev := report.SFDetail[i-1]
		curr := report.SFDetail[i]
		if prev.Stream > curr.Stream || (prev.Stream == curr.Stream && prev.Function > curr.Function) {
			t.Errorf("S/F detail not sorted: S%dF%d after S%dF%d", curr.Stream, curr.Function, prev.Stream, prev.Function)
		}
	}
}
