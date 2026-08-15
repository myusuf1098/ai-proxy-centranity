package limiter

import (
	"context"
	"sync"
	"time"
)

// RateLimiter interface for rate limit verification
type RateLimiter interface {
	Allow(ctx context.Context, keyID string, limitRPM int) (allowed bool, remaining int, retryAfter time.Duration)
	AllowRPS(ctx context.Context, keyID string, limitRPS int) (allowed bool, remaining int, retryAfter time.Duration)
}

type windowEntry struct {
	timestamps []time.Time
}

// MemoryLimiter implements sliding window rate limiting in memory
type MemoryLimiter struct {
	mu      sync.Mutex
	windows map[string]*windowEntry
}

// NewMemoryLimiter creates a new in-memory rate limiter
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		windows: make(map[string]*windowEntry),
	}
}

// Allow evaluates if a request for the given keyID is allowed within the 1-minute sliding window
func (l *MemoryLimiter) Allow(ctx context.Context, keyID string, limitRPM int) (bool, int, time.Duration) {
	if limitRPM <= 0 {
		return true, 999999, 0 // No limit configured
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	entry, exists := l.windows[keyID]
	if !exists {
		entry = &windowEntry{timestamps: make([]time.Time, 0, limitRPM)}
		l.windows[keyID] = entry
	}

	// Purge timestamps older than 1 minute
	valid := make([]time.Time, 0, len(entry.timestamps))
	for _, t := range entry.timestamps {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid

	currentCount := len(entry.timestamps)
	if currentCount >= limitRPM {
		oldest := entry.timestamps[0]
		retryAfter := oldest.Add(1 * time.Minute).Sub(now)
		if retryAfter < 0 {
			retryAfter = time.Second
		}
		return false, 0, retryAfter
	}

	// Record current request
	entry.timestamps = append(entry.timestamps, now)
	remaining := limitRPM - len(entry.timestamps)
	return true, remaining, 0
}

func (l *MemoryLimiter) AllowRPS(ctx context.Context, keyID string, limitRPS int) (bool, int, time.Duration) {
	if limitRPS <= 0 {
		return true, 999999, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Second)
	windowKey := keyID + ":rps"

	entry, exists := l.windows[windowKey]
	if !exists {
		entry = &windowEntry{timestamps: make([]time.Time, 0, limitRPS)}
		l.windows[windowKey] = entry
	}

	valid := make([]time.Time, 0, len(entry.timestamps))
	for _, t := range entry.timestamps {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid

	if len(entry.timestamps) >= limitRPS {
		retryAfter := entry.timestamps[0].Add(time.Second).Sub(now)
		if retryAfter < 0 {
			retryAfter = time.Second
		}
		return false, 0, retryAfter
	}

	entry.timestamps = append(entry.timestamps, now)
	remaining := limitRPS - len(entry.timestamps)
	return true, remaining, 0
}
