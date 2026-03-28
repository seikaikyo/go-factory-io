package security

import (
	"sync"
	"time"
)

// AnomalyDetector is the interface for external anomaly detection systems.
// Implementations analyze SECS message patterns to detect abnormal behavior.
// The interface is deliberately minimal to support various ML backends.
type AnomalyDetector interface {
	// Analyze evaluates a message pattern and returns an anomaly score (0.0 - 1.0).
	// Score > threshold triggers an alert. Returns 0 if the pattern is normal.
	Analyze(pattern MessagePattern) float64

	// Train feeds a known-good pattern for baseline learning.
	Train(pattern MessagePattern)
}

// MessagePattern captures statistics about a SECS message for anomaly analysis.
type MessagePattern struct {
	Stream      byte          // SECS stream number
	Function    byte          // SECS function number
	Source      string        // Remote address or session ID
	DataSize    int           // Payload size in bytes
	Timestamp   time.Time
	InterArrival time.Duration // Time since previous message from same source
	IsReply     bool
}

// PatternCollector gathers message pattern statistics for anomaly detection.
// Feeds patterns to a registered AnomalyDetector for real-time analysis.
type PatternCollector struct {
	mu       sync.RWMutex
	detector AnomalyDetector
	threshold float64

	// Per-source timing for inter-arrival calculation
	lastSeen map[string]time.Time

	// Aggregate statistics
	sfCounts   map[sfKey]int64
	totalMsgs  int64
	alertCount int64

	onAlert func(pattern MessagePattern, score float64)
}

type sfKey struct {
	stream, function byte
}

// NewPatternCollector creates a message pattern collector.
// threshold: anomaly score above which an alert is triggered (e.g., 0.8).
func NewPatternCollector(detector AnomalyDetector, threshold float64) *PatternCollector {
	return &PatternCollector{
		detector:  detector,
		threshold: threshold,
		lastSeen:  make(map[string]time.Time),
		sfCounts:  make(map[sfKey]int64),
	}
}

// OnAlert sets a callback for when an anomaly is detected.
func (pc *PatternCollector) OnAlert(fn func(pattern MessagePattern, score float64)) {
	pc.mu.Lock()
	pc.onAlert = fn
	pc.mu.Unlock()
}

// Record processes an incoming message and checks for anomalies.
func (pc *PatternCollector) Record(stream, function byte, source string, dataSize int, isReply bool) {
	pc.mu.Lock()

	now := time.Now()
	var interArrival time.Duration
	if last, ok := pc.lastSeen[source]; ok {
		interArrival = now.Sub(last)
	}
	pc.lastSeen[source] = now

	key := sfKey{stream, function}
	pc.sfCounts[key]++
	pc.totalMsgs++

	pattern := MessagePattern{
		Stream:       stream,
		Function:     function,
		Source:       source,
		DataSize:     dataSize,
		Timestamp:    now,
		InterArrival: interArrival,
		IsReply:      isReply,
	}

	detector := pc.detector
	threshold := pc.threshold
	onAlert := pc.onAlert
	pc.mu.Unlock()

	if detector == nil {
		return
	}

	score := detector.Analyze(pattern)
	if score > threshold && onAlert != nil {
		pc.mu.Lock()
		pc.alertCount++
		pc.mu.Unlock()
		onAlert(pattern, score)
	}
}

// Train feeds a known-good message pattern to the detector.
func (pc *PatternCollector) Train(stream, function byte, source string, dataSize int) {
	if pc.detector == nil {
		return
	}

	pattern := MessagePattern{
		Stream:    stream,
		Function:  function,
		Source:    source,
		DataSize:  dataSize,
		Timestamp: time.Now(),
	}
	pc.detector.Train(pattern)
}

// Stats returns aggregate statistics.
func (pc *PatternCollector) Stats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	sfStats := make(map[string]int64)
	for k, v := range pc.sfCounts {
		sfStats[sfKeyString(k)] = v
	}

	return map[string]interface{}{
		"total_messages":   pc.totalMsgs,
		"alert_count":      pc.alertCount,
		"unique_sources":   len(pc.lastSeen),
		"sf_distribution":  sfStats,
	}
}

// AlertCount returns the number of anomaly alerts triggered.
func (pc *PatternCollector) AlertCount() int64 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.alertCount
}

func sfKeyString(k sfKey) string {
	return string(rune('S')) + string(rune('0'+k.stream)) + string(rune('F')) + string(rune('0'+k.function))
}

// NoopDetector is a detector that never reports anomalies. Useful as a default.
type NoopDetector struct{}

func (NoopDetector) Analyze(MessagePattern) float64 { return 0 }
func (NoopDetector) Train(MessagePattern)           {}

// ThresholdDetector is a simple detector that flags messages with unusually large payloads.
// Useful for testing and as a baseline detector.
type ThresholdDetector struct {
	MaxDataSize int // Messages larger than this get score 1.0
}

func (d ThresholdDetector) Analyze(p MessagePattern) float64 {
	if d.MaxDataSize > 0 && p.DataSize > d.MaxDataSize {
		return 1.0
	}
	return 0
}

func (d ThresholdDetector) Train(MessagePattern) {}
