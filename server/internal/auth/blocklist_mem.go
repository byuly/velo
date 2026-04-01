package auth

import (
	"context"
	"sync"
	"time"
)

// Compile-time interface check.
var _ TokenBlocklist = (*MemoryBlocklist)(nil)

// MemoryBlocklist implements TokenBlocklist using an in-memory map with TTL.
// Suitable for single-instance MVP deployments where Redis is not available.
// Blocked tokens are lost on restart, but since access tokens have a short TTL
// (60 min), this is acceptable.
type MemoryBlocklist struct {
	mu      sync.Mutex
	entries map[string]time.Time // jti → expiry time
	stop    chan struct{}
}

// NewMemoryBlocklist creates an in-memory blocklist that sweeps expired entries
// at the given interval. Call Stop() to release the background goroutine.
func NewMemoryBlocklist(sweepInterval time.Duration) *MemoryBlocklist {
	bl := &MemoryBlocklist{
		entries: make(map[string]time.Time),
		stop:    make(chan struct{}),
	}
	go bl.sweepLoop(sweepInterval)
	return bl
}

func (m *MemoryBlocklist) Block(_ context.Context, jti string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[jti] = time.Now().Add(ttl)
	return nil
}

func (m *MemoryBlocklist) IsBlocked(_ context.Context, jti string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	expiry, ok := m.entries[jti]
	if !ok {
		return false, nil
	}
	if time.Now().After(expiry) {
		delete(m.entries, jti)
		return false, nil
	}
	return true, nil
}

// Stop signals the background sweep goroutine to exit.
func (m *MemoryBlocklist) Stop() {
	close(m.stop)
}

func (m *MemoryBlocklist) sweepLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.sweep()
		}
	}
}

func (m *MemoryBlocklist) sweep() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for jti, expiry := range m.entries {
		if now.After(expiry) {
			delete(m.entries, jti)
		}
	}
}
