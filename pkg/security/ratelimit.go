package security

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for per-connection
// message rate control. Covers IEC 62443 FR7 (Resource Availability).
type RateLimiter struct {
	mu       sync.Mutex
	rate     int           // tokens per second
	burst    int           // max tokens (bucket size)
	tokens   float64       // current tokens
	lastTime time.Time
}

// NewRateLimiter creates a rate limiter.
// rate: messages allowed per second
// burst: max burst size (allows short bursts above the sustained rate)
func NewRateLimiter(rate, burst int) *RateLimiter {
	return &RateLimiter{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

// Allow checks if a message is allowed. Returns true if within rate limit.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now

	// Refill tokens
	rl.tokens += elapsed * float64(rl.rate)
	if rl.tokens > float64(rl.burst) {
		rl.tokens = float64(rl.burst)
	}

	// Consume one token
	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	return false
}

// Rate returns the configured rate (messages/sec).
func (rl *RateLimiter) Rate() int {
	return rl.rate
}
