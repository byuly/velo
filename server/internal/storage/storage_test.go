package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestS3Client_ReelURL(t *testing.T) {
	// ReelURL is a pure function — no AWS calls needed.
	c := &S3Client{cdnDomain: "cdn.example.com"}
	got := c.ReelURL("reels/abc-123/reel.mp4")
	require.Equal(t, "https://cdn.example.com/reels/abc-123/reel.mp4", got)
}

func TestMemStorage_DownloadUpload(t *testing.T) {
	ctx := context.Background()
	m := NewMemStorage("cdn.test")

	// Seed a file.
	m.Put("clips", "test.mp4", []byte("fake video data"))

	// Download it.
	dir := t.TempDir()
	localPath := filepath.Join(dir, "test.mp4")
	require.NoError(t, m.Download(ctx, "clips", "test.mp4", localPath))

	data, err := os.ReadFile(localPath)
	require.NoError(t, err)
	require.Equal(t, []byte("fake video data"), data)

	// Upload a new file.
	outPath := filepath.Join(dir, "reel.mp4")
	require.NoError(t, os.WriteFile(outPath, []byte("reel data"), 0o644))
	require.NoError(t, m.Upload(ctx, "reels", "reel.mp4", outPath))

	got, ok := m.Get("reels", "reel.mp4")
	require.True(t, ok)
	require.Equal(t, []byte("reel data"), got)
}

func TestMemStorage_DownloadNotFound(t *testing.T) {
	m := NewMemStorage("cdn.test")
	err := m.Download(context.Background(), "clips", "missing.mp4", "/tmp/nope.mp4")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMemStorage_ReelURL(t *testing.T) {
	m := NewMemStorage("cdn.test")
	require.Equal(t, "https://cdn.test/reels/abc/reel.mp4", m.ReelURL("reels/abc/reel.mp4"))
}
