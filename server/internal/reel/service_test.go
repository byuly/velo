package reel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/byuly/velo/server/internal/ffmpeg"
	"github.com/byuly/velo/server/internal/storage"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/stretchr/testify/require"
)

// --- mocks ---

// failStorage fails on Download.
type failStorage struct {
	storage.MemStorage
}

func (f *failStorage) Download(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("simulated S3 failure")
}

// failUploadStorage fails on Upload.
type failUploadStorage struct {
	*storage.MemStorage
}

func (f *failUploadStorage) Download(ctx context.Context, bucket, key, localPath string) error {
	return f.MemStorage.Download(ctx, bucket, key, localPath)
}

func (f *failUploadStorage) Upload(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("simulated S3 upload failure")
}

func (f *failUploadStorage) ReelURL(key string) string {
	return f.MemStorage.ReelURL(key)
}

// mockNormalizer implements clipNormalizer by copying input to output.
type mockNormalizer struct {
	err    error
	called int
}

func (m *mockNormalizer) NormalizeClip(_ context.Context, input, output string) error {
	m.called++
	if m.err != nil {
		return m.err
	}
	return copyFile(input, output)
}

// mockEngine implements reelComposer by writing a dummy file.
type mockEngine struct {
	err error
}

func (m *mockEngine) Compose(_ context.Context, req ffmpeg.ComposeRequest) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	out := filepath.Join(req.WorkDir, "reel.mp4")
	if err := os.WriteFile(out, []byte("fake-reel-data"), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// --- helpers ---

// seedClipInStorage creates a dummy clip file and stores it in MemStorage
// under the given S3 key, returning the key.
func seedClipInStorage(t *testing.T, mem *storage.MemStorage, bucket, s3Key string) {
	t.Helper()
	mem.Put(bucket, s3Key, []byte("fake-video-data"))
}

// newMockService creates a Service wired with mocks and a real DB store.
func newMockService(t *testing.T, store *Store, mem storage.Storage, norm clipNormalizer, eng reelComposer) *Service {
	t.Helper()
	return NewService(store, mem, eng, norm, "clips", "reels", testLogger())
}

// --- Tier 1: mock-based tests ---

func TestGenerate_ZeroSubmitters(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)

	// No clips → zero submitters.
	svc := newMockService(t, store, mem, nil, nil)
	err := svc.Generate(ctx, sess)
	require.NoError(t, err)

	var status string
	var reelURL *string
	err = pool.QueryRow(ctx, "SELECT status, reel_url FROM sessions WHERE id = $1", sess).
		Scan(&status, &reelURL)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.Nil(t, reelURL)
}

func TestGenerate_DownloadFailure(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	fs := &failStorage{}
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	createTestClip(t, pool, sess, user, slot)

	svc := newMockService(t, store, fs, &mockNormalizer{}, &mockEngine{})
	err := svc.Generate(ctx, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), "download clip")
}

func TestGenerate_NormalizeFailure(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	norm := &mockNormalizer{err: fmt.Errorf("ffmpeg crashed")}
	svc := newMockService(t, store, mem, norm, &mockEngine{})

	err := svc.Generate(ctx, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), "normalize clip")
	require.Equal(t, 1, norm.called)
}

func TestGenerate_ComposeFailure(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	eng := &mockEngine{err: fmt.Errorf("composition failed")}
	svc := newMockService(t, store, mem, &mockNormalizer{}, eng)

	err := svc.Generate(ctx, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compose reel")
}

func TestGenerate_UploadFailure(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	fs := &failUploadStorage{MemStorage: mem}
	svc := newMockService(t, store, fs, &mockNormalizer{}, &mockEngine{})

	err := svc.Generate(ctx, sess)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload reel")
}

func TestGenerate_ContextCancellation(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	// Normalizer cancels context on first call.
	ctx, cancel := context.WithCancel(context.Background())
	norm := &mockNormalizer{err: fmt.Errorf("context canceled")}
	cancel() // pre-cancel

	svc := newMockService(t, store, mem, norm, &mockEngine{})
	err := svc.Generate(ctx, sess)
	require.Error(t, err)
}

func TestGenerate_Success_Mock(t *testing.T) {
	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	user := createTestUser(t, pool)
	sess := createTestSessionGenerating(t, pool, user)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user)
	clip := createTestClipReturning(t, pool, sess, user, slot)

	seedClipInStorage(t, mem, "clips", clip.S3Key)

	norm := &mockNormalizer{}
	eng := &mockEngine{}
	svc := newMockService(t, store, mem, norm, eng)

	err := svc.Generate(ctx, sess)
	require.NoError(t, err)

	// Verify DB state.
	var status string
	var reelURL *string
	err = pool.QueryRow(ctx, "SELECT status, reel_url FROM sessions WHERE id = $1", sess).
		Scan(&status, &reelURL)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.NotNil(t, reelURL)
	require.Contains(t, *reelURL, "cdn.test")
	require.Contains(t, *reelURL, sess.String())

	// Verify reel was uploaded.
	reelKey := fmt.Sprintf("reels/%s/reel.mp4", sess)
	data, ok := mem.Get("reels", reelKey)
	require.True(t, ok, "reel should be in storage")
	require.NotEmpty(t, data)

	// Verify normalizer was called.
	require.Equal(t, 1, norm.called)
}

// --- Tier 2: integration test with real FFmpeg ---

func TestGenerate_Integration(t *testing.T) {
	requireFFmpeg(t)

	pool := setupTestDBOrSkip(t)
	store := NewStore(pool)
	mem := storage.NewMemStorage("cdn.test")
	ctx := context.Background()

	// Generate synthetic clips.
	fixtureDir := t.TempDir()
	clip1Path := generateSyntheticClip(t, fixtureDir, "clip1.mp4", 3)
	clip2Path := generateSyntheticClip(t, fixtureDir, "clip2.mp4", 3)

	clip1Data, err := os.ReadFile(clip1Path)
	require.NoError(t, err)
	clip2Data, err := os.ReadFile(clip2Path)
	require.NoError(t, err)

	// Create DB records: 2 users, 1 session, 1 slot, 2 participants, 2 clips.
	user1 := testutil.CreateUser(t, pool, nil)
	user2 := testutil.CreateUser(t, pool, nil)

	sess := createTestSessionGenerating(t, pool, user1.ID)
	slot := createTestSlot(t, pool, sess, 0)
	createTestParticipant(t, pool, sess, user1.ID)
	createTestParticipant(t, pool, sess, user2.ID)

	clip1 := createTestClipReturning(t, pool, sess, user1.ID, slot)
	clip2 := createTestClipReturning(t, pool, sess, user2.ID, slot)

	// Seed clips in mock S3.
	mem.Put("clips", clip1.S3Key, clip1Data)
	mem.Put("clips", clip2.S3Key, clip2Data)

	// Wire real FFmpeg.
	composer, err := ffmpeg.New()
	require.NoError(t, err)
	engine := ffmpeg.NewEngine(composer)

	svc := NewService(store, mem, engine, composer, "clips", "reels", testLogger())
	err = svc.Generate(ctx, sess)
	require.NoError(t, err)

	// Verify DB state.
	var status string
	var reelURL *string
	err = pool.QueryRow(ctx, "SELECT status, reel_url FROM sessions WHERE id = $1", sess).
		Scan(&status, &reelURL)
	require.NoError(t, err)
	require.Equal(t, "complete", status)
	require.NotNil(t, reelURL)
	require.Contains(t, *reelURL, sess.String())

	// Verify reel exists in storage.
	reelKey := fmt.Sprintf("reels/%s/reel.mp4", sess)
	reelData, ok := mem.Get("reels", reelKey)
	require.True(t, ok, "reel should be in storage")
	require.True(t, len(reelData) > 1000, "reel should be a non-trivial file, got %d bytes", len(reelData))

	// Write reel to disk and probe it with ffprobe.
	reelFile := filepath.Join(t.TempDir(), "reel.mp4")
	require.NoError(t, os.WriteFile(reelFile, reelData, 0o644))

	pr, err := composer.Probe(ctx, reelFile)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height) // 2 panels stacked: 720×640 * 2
	require.True(t, pr.Duration > 0, "reel duration should be > 0")
	require.True(t, pr.HasAudio, "reel should have audio")
}
