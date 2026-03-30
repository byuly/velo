package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Compile-time interface check.
var _ Storage = (*MemStorage)(nil)

// MemStorage is an in-memory Storage implementation for tests.
// Files are stored as byte slices keyed by "bucket/key".
type MemStorage struct {
	mu        sync.Mutex
	files     map[string][]byte
	cdnDomain string
}

// NewMemStorage creates a MemStorage.
func NewMemStorage(cdnDomain string) *MemStorage {
	return &MemStorage{
		files:     make(map[string][]byte),
		cdnDomain: cdnDomain,
	}
}

// Put adds a file to the in-memory store (for test setup).
func (m *MemStorage) Put(bucket, key string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[bucket+"/"+key] = data
}

// Get retrieves a file from the in-memory store (for test assertions).
func (m *MemStorage) Get(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[bucket+"/"+key]
	return data, ok
}

// Download writes the stored bytes to localPath.
func (m *MemStorage) Download(_ context.Context, bucket, key, localPath string) error {
	m.mu.Lock()
	data, ok := m.files[bucket+"/"+key]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("mem storage: %s/%s not found", bucket, key)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", localPath, err)
	}
	return os.WriteFile(localPath, data, 0o644)
}

// Upload reads localPath into the in-memory store.
func (m *MemStorage) Upload(_ context.Context, bucket, key, localPath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[bucket+"/"+key] = data
	return nil
}

// ReelURL returns the CDN URL for a reel key.
func (m *MemStorage) ReelURL(key string) string {
	return fmt.Sprintf("https://%s/%s", m.cdnDomain, key)
}
