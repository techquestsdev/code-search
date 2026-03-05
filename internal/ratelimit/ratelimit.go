package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/techquestsdev/code-search/internal/metrics"
)

// Config holds rate limiter configuration.
type Config struct {
	RequestsPerSecond float64       // Requests allowed per second
	BurstSize         int           // Maximum burst size
	CleanupInterval   time.Duration // How often to clean up old entries
	Enabled           bool          // Whether rate limiting is enabled
}

// DefaultConfig returns the default rate limiter configuration.
func DefaultConfig() *Config {
	return &Config{
		RequestsPerSecond: 100,
		BurstSize:         200,
		CleanupInterval:   time.Minute,
		Enabled:           true,
	}
}

// Limiter implements a per-IP token bucket rate limiter.
type Limiter struct {
	config  *Config
	buckets map[string]*tokenBucket
	mu      sync.RWMutex
	stopCh  chan struct{}
}

// tokenBucket implements the token bucket algorithm.
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewLimiter creates a new rate limiter.
func NewLimiter(config *Config) *Limiter {
	if config == nil {
		config = DefaultConfig()
	}

	l := &Limiter{
		config:  config,
		buckets: make(map[string]*tokenBucket),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go l.cleanup()

	return l
}

// Stop stops the rate limiter cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stopCh)
}

// Allow checks if a request from the given key (usually IP) is allowed.
func (l *Limiter) Allow(key string) bool {
	if !l.config.Enabled {
		return true
	}

	l.mu.Lock()

	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(l.config.BurstSize),
			lastUpdate: time.Now(),
		}
		l.buckets[key] = bucket
	}

	l.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.lastUpdate = now

	// Add tokens based on time elapsed
	bucket.tokens += elapsed * l.config.RequestsPerSecond
	if bucket.tokens > float64(l.config.BurstSize) {
		bucket.tokens = float64(l.config.BurstSize)
	}

	// Check if we have tokens available
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// RemainingTokens returns the number of remaining tokens for a key.
func (l *Limiter) RemainingTokens(key string) int {
	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return l.config.BurstSize
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refresh tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()

	tokens := bucket.tokens + elapsed*l.config.RequestsPerSecond
	if tokens > float64(l.config.BurstSize) {
		tokens = float64(l.config.BurstSize)
	}

	return int(tokens)
}

// ResetAfter returns the duration until the bucket is refilled.
func (l *Limiter) ResetAfter(key string) time.Duration {
	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		return 0
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if bucket.tokens >= 1 {
		return 0
	}

	// Calculate time until we have 1 token
	needed := 1 - bucket.tokens

	return time.Duration(needed/l.config.RequestsPerSecond*1000) * time.Millisecond
}

// cleanup periodically removes old entries.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.doCleanup()
		}
	}
}

// doCleanup removes entries that haven't been used recently.
func (l *Limiter) doCleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	threshold := time.Now().Add(-l.config.CleanupInterval * 2)

	for key, bucket := range l.buckets {
		bucket.mu.Lock()

		if bucket.lastUpdate.Before(threshold) {
			delete(l.buckets, key)
		}

		bucket.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get client IP
		key := getClientIP(r)

		// Check rate limit
		if !l.Allow(key) {
			// Record rate limit rejection in metrics
			metrics.RecordRateLimit(false)

			w.Header().Set("X-Ratelimit-Limit", strconv.Itoa(l.config.BurstSize))
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().
				Set("X-Ratelimit-Reset", strconv.FormatInt(time.Now().Add(l.ResetAfter(key)).Unix(), 10))
			w.Header().Set("Retry-After", strconv.Itoa(int(l.ResetAfter(key).Seconds())))

			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)

			return
		}

		// Record rate limit allowed in metrics
		metrics.RecordRateLimit(true)

		// Add rate limit headers
		remaining := l.RemainingTokens(key)
		w.Header().Set("X-Ratelimit-Limit", strconv.Itoa(l.config.BurstSize))
		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
		w.Header().
			Set("X-Ratelimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := range len(xff) {
			if xff[i] == ',' {
				return xff[:i]
			}
		}

		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}

	return ip
}

// MiddlewareFunc is a convenience function that creates a rate limiting middleware.
func MiddlewareFunc(requestsPerSecond float64, burstSize int) func(http.Handler) http.Handler {
	limiter := NewLimiter(&Config{
		RequestsPerSecond: requestsPerSecond,
		BurstSize:         burstSize,
		CleanupInterval:   time.Minute,
		Enabled:           true,
	})

	return limiter.Middleware
}
