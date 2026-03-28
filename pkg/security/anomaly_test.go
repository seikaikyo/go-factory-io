package security

import (
	"testing"
)

func TestPatternCollectorNoDetector(t *testing.T) {
	pc := NewPatternCollector(nil, 0.8)
	// Should not panic with nil detector
	pc.Record(1, 1, "10.0.0.1", 100, false)
	pc.Train(1, 1, "10.0.0.1", 100)

	stats := pc.Stats()
	if stats["total_messages"].(int64) != 1 {
		t.Errorf("total_messages = %v, want 1", stats["total_messages"])
	}
}

func TestPatternCollectorNoopDetector(t *testing.T) {
	pc := NewPatternCollector(NoopDetector{}, 0.8)

	var alertCount int
	pc.OnAlert(func(p MessagePattern, score float64) {
		alertCount++
	})

	pc.Record(1, 1, "10.0.0.1", 100, false)
	pc.Record(1, 3, "10.0.0.1", 200, false)

	if alertCount != 0 {
		t.Errorf("alertCount = %d, want 0 (NoopDetector)", alertCount)
	}
}

func TestPatternCollectorThresholdDetector(t *testing.T) {
	detector := ThresholdDetector{MaxDataSize: 1000}
	pc := NewPatternCollector(detector, 0.5)

	var alerts []MessagePattern
	pc.OnAlert(func(p MessagePattern, score float64) {
		alerts = append(alerts, p)
	})

	// Normal message
	pc.Record(1, 1, "10.0.0.1", 100, false)
	if len(alerts) != 0 {
		t.Error("should not alert on normal message")
	}

	// Large message (anomaly)
	pc.Record(6, 11, "10.0.0.1", 5000, false)
	if len(alerts) != 1 {
		t.Errorf("alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Stream != 6 || alerts[0].Function != 11 {
		t.Errorf("alert S/F = S%dF%d, want S6F11", alerts[0].Stream, alerts[0].Function)
	}

	if pc.AlertCount() != 1 {
		t.Errorf("AlertCount = %d, want 1", pc.AlertCount())
	}
}

func TestPatternCollectorStats(t *testing.T) {
	pc := NewPatternCollector(NoopDetector{}, 0.8)

	pc.Record(1, 1, "10.0.0.1", 100, false)
	pc.Record(1, 1, "10.0.0.1", 100, false)
	pc.Record(1, 3, "10.0.0.2", 200, false)

	stats := pc.Stats()
	if stats["total_messages"].(int64) != 3 {
		t.Errorf("total = %v, want 3", stats["total_messages"])
	}
	if stats["unique_sources"].(int) != 2 {
		t.Errorf("sources = %v, want 2", stats["unique_sources"])
	}
}

func TestPatternCollectorInterArrival(t *testing.T) {
	detector := &recordingDetector{}
	pc := NewPatternCollector(detector, 0.8)

	pc.Record(1, 1, "src", 10, false)
	pc.Record(1, 1, "src", 10, false)

	if len(detector.patterns) != 2 {
		t.Fatalf("patterns = %d, want 2", len(detector.patterns))
	}
	// First message should have 0 inter-arrival
	if detector.patterns[0].InterArrival != 0 {
		t.Errorf("first inter-arrival = %v, want 0", detector.patterns[0].InterArrival)
	}
	// Second message should have > 0 inter-arrival
	if detector.patterns[1].InterArrival <= 0 {
		t.Errorf("second inter-arrival = %v, want > 0", detector.patterns[1].InterArrival)
	}
}

func TestThresholdDetectorAnalyze(t *testing.T) {
	d := ThresholdDetector{MaxDataSize: 500}

	if score := d.Analyze(MessagePattern{DataSize: 100}); score != 0 {
		t.Errorf("score = %.1f, want 0 for normal message", score)
	}
	if score := d.Analyze(MessagePattern{DataSize: 1000}); score != 1.0 {
		t.Errorf("score = %.1f, want 1.0 for large message", score)
	}
}

func TestAnomalyDetectorInterface(t *testing.T) {
	// Verify both detectors implement the interface
	var _ AnomalyDetector = NoopDetector{}
	var _ AnomalyDetector = ThresholdDetector{}
}

// recordingDetector records all analyzed patterns for testing.
type recordingDetector struct {
	patterns []MessagePattern
}

func (d *recordingDetector) Analyze(p MessagePattern) float64 {
	d.patterns = append(d.patterns, p)
	return 0
}

func (d *recordingDetector) Train(p MessagePattern) {}
