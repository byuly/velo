package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixturesDir is set by TestMain after generating synthetic clips.
var fixturesDir string

// clip paths set by TestMain.
var (
	clipRed10s   string // 1080×1920, 10s, VFR, AAC
	clipBlue8s   string // 1080×1920, 8s, rotate=90 metadata, AAC
	clipGreen5s  string // 720×1280, 5s, no rotation, AAC
)

// Pre-normalized clip paths set by TestMain (Phase 1 output: CFR 30fps, original resolution, audio preserved).
var (
	normRed10s  string // normalized red clip (~10s, 1080×1920)
	normGreen5s string // normalized green clip (~5s, 720×1280)
)

// requireFFmpeg skips the test if ffmpeg is not available in PATH.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH — skipping")
	}
}

// requireDrawtext skips the test if the drawtext filter is not available.
func requireDrawtext(t *testing.T) {
	t.Helper()
	requireFFmpeg(t)
	c, err := New()
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	if !c.HasDrawtext() {
		t.Skip("ffmpeg drawtext filter not available — skipping")
	}
}

// TestMain generates synthetic fixture clips once and runs all tests.
func TestMain(m *testing.M) {
	// If ffmpeg is absent, tests will be skipped individually via requireFFmpeg.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		os.Exit(m.Run())
	}

	dir, err := os.MkdirTemp("", "ffmpeg-fixtures-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create fixture dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	fixturesDir = dir
	clipRed10s = filepath.Join(dir, "clip_red_10s.mp4")
	clipBlue8s = filepath.Join(dir, "clip_blue_8s.mp4")
	clipGreen5s = filepath.Join(dir, "clip_green_5s.mp4")

	if err := generateFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate fixtures: %v\n", err)
		os.Exit(1)
	}

	if err := generateNormalizedFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate normalized fixtures: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func generateFixtures() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c := NewWithBin("ffmpeg", "ffprobe")

	// clip_red_10s — VFR source simulated with -vsync vfr
	if err := runFFmpeg(ctx, c.ffmpegBin,
		"-y",
		"-f", "lavfi", "-i", "color=c=red:s=1080x1920:r=30",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "10",
		"-c:v", "libx264", "-preset", "fast", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-vsync", "vfr",
		clipRed10s,
	); err != nil {
		return fmt.Errorf("generate red clip: %w", err)
	}

	// clip_blue_8s — with simulated rotation metadata (rotate=90)
	if err := runFFmpeg(ctx, c.ffmpegBin,
		"-y",
		"-f", "lavfi", "-i", "color=c=blue:s=1080x1920:r=30",
		"-f", "lavfi", "-i", "sine=frequency=660:sample_rate=44100",
		"-t", "8",
		"-c:v", "libx264", "-preset", "fast", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-metadata:s:v:0", "rotate=90",
		clipBlue8s,
	); err != nil {
		return fmt.Errorf("generate blue clip: %w", err)
	}

	// clip_green_5s — already portrait 720×1280, no rotation
	if err := runFFmpeg(ctx, c.ffmpegBin,
		"-y",
		"-f", "lavfi", "-i", "color=c=green:s=720x1280:r=30",
		"-f", "lavfi", "-i", "sine=frequency=880:sample_rate=44100",
		"-t", "5",
		"-c:v", "libx264", "-preset", "fast", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		clipGreen5s,
	); err != nil {
		return fmt.Errorf("generate green clip: %w", err)
	}

	return nil
}

func generateNormalizedFixtures() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c := NewWithBin("ffmpeg", "ffprobe")

	normRed10s = filepath.Join(fixturesDir, "norm_red_10s.mp4")
	if err := c.NormalizeClip(ctx, clipRed10s, normRed10s); err != nil {
		return fmt.Errorf("normalize red: %w", err)
	}

	normGreen5s = filepath.Join(fixturesDir, "norm_green_5s.mp4")
	if err := c.NormalizeClip(ctx, clipGreen5s, normGreen5s); err != nil {
		return fmt.Errorf("normalize green: %w", err)
	}

	return nil
}

// runFFmpeg is a thin helper for fixture generation (no Composer.run wrapping needed).
func runFFmpeg(ctx context.Context, bin string, args ...string) error {
	return NewWithBin(bin, "ffprobe").run(ctx, args...)
}

// ─── probe helpers ──────────────────────────────────────────────────────────

type probeStreams struct {
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"` // e.g. "30/1"
	AvgFR      string `json:"avg_frame_rate"`
	Duration   string `json:"duration"`
	Tags       struct {
		Rotate string `json:"rotate"`
	} `json:"tags"`
}

func probe(t *testing.T, path string) probeStreams {
	t.Helper()
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	).Output()
	require.NoError(t, err, "ffprobe failed on %s", path)

	var ps probeStreams
	require.NoError(t, json.Unmarshal(out, &ps))
	return ps
}

func videoStream(t *testing.T, ps probeStreams) probeStream {
	t.Helper()
	for _, s := range ps.Streams {
		if s.CodecType == "video" {
			return s
		}
	}
	t.Fatal("no video stream found")
	return probeStream{}
}

func hasAudioStream(ps probeStreams) bool {
	for _, s := range ps.Streams {
		if s.CodecType == "audio" {
			return true
		}
	}
	return false
}

func parseDuration(t *testing.T, ps probeStreams) float64 {
	t.Helper()
	for _, s := range ps.Streams {
		if s.CodecType == "video" && s.Duration != "" {
			var d float64
			_, err := fmt.Sscanf(s.Duration, "%f", &d)
			require.NoError(t, err)
			return d
		}
	}
	t.Fatal("no duration found")
	return 0
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestProbe(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	pr, err := c.Probe(ctx, clipRed10s)
	require.NoError(t, err)

	require.Equal(t, 1080, pr.Width, "width")
	require.Equal(t, 1920, pr.Height, "height")
	require.True(t, pr.HasAudio, "fixture has audio")
	require.InDelta(t, 10.0, pr.Duration, 0.5, "duration ~10s")
}

func TestProbe_NoFile(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	_, err = c.Probe(ctx, "/nonexistent/file.mp4")
	require.Error(t, err)
}

func TestNormalizeClip(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "normalized.mp4")
	require.NoError(t, c.NormalizeClip(ctx, clipRed10s, out))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	// Original resolution preserved (no scaling in Phase 1).
	require.Equal(t, 1080, vs.Width, "width must match original")
	require.Equal(t, 1920, vs.Height, "height must match original")
	require.True(t, hasAudioStream(ps), "NormalizeClip must preserve audio")
	require.Equal(t, "30/1", vs.RFrameRate, "frame rate must be CFR 30fps")
}

func TestNormalizeClip_Rotation(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "rotated_normalized.mp4")
	require.NoError(t, c.NormalizeClip(ctx, clipBlue8s, out))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	// Autorotate decodes to portrait; original resolution preserved.
	require.True(t, hasAudioStream(ps), "audio must be preserved")
	// Rotation metadata is handled by autorotate; dims depend on FFmpeg version.
	// The key assertion is that normalization completes without error.
	require.Greater(t, vs.Width, 0, "valid width")
	require.Greater(t, vs.Height, 0, "valid height")
}

func TestScaleClip(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	norm := filepath.Join(tmp, "norm.mp4")
	require.NoError(t, c.NormalizeClip(ctx, clipGreen5s, norm))

	dims := PanelDimsFor(2) // 720×640
	out := filepath.Join(tmp, "scaled.mp4")
	require.NoError(t, c.ScaleClip(ctx, norm, out, dims))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, 720, vs.Width, "scaled width")
	require.Equal(t, 640, vs.Height, "scaled height")
	require.True(t, hasAudioStream(ps), "ScaleClip must preserve audio")
}

func TestScaleClip_Identity(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	norm := filepath.Join(tmp, "norm.mp4")
	require.NoError(t, c.NormalizeClip(ctx, clipGreen5s, norm))

	dims := PanelDimsFor(1) // 720×1280 — same as source
	out := filepath.Join(tmp, "identity.mp4")
	require.NoError(t, c.ScaleClip(ctx, norm, out, dims))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, 720, vs.Width)
	require.Equal(t, 1280, vs.Height)
}

func TestGenerateBlackPanel(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "black.mp4")
	dims := PanelDimsFor(2) // 720×640
	target := 7.5
	require.NoError(t, c.GenerateBlackPanel(ctx, out, dims, target, "Offline"))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, 720, vs.Width)
	require.Equal(t, 640, vs.Height)
	require.True(t, hasAudioStream(ps), "black panel must have a silent audio track")

	dur := parseDuration(t, ps)
	require.InDelta(t, target, dur, 0.5, "duration should be close to target")
}

func TestStackPanels_1(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	dims := PanelDimsFor(1)
	panel := makePanel(t, ctx, c, clipGreen5s, dims, filepath.Join(t.TempDir(), "p0.mp4"))

	out := filepath.Join(t.TempDir(), "stack1.mp4")
	panels := []PanelInput{{Path: panel}}
	require.NoError(t, c.StackPanels(ctx, out, panels, 0))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, dims.Width, vs.Width)
	require.Equal(t, dims.Height, vs.Height)
}

func TestStackPanels_2(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(2)

	p1 := makePanel(t, ctx, c, clipRed10s, dims, filepath.Join(tmp, "p1.mp4"))
	p2 := makePanel(t, ctx, c, clipGreen5s, dims, filepath.Join(tmp, "p2.mp4"))

	out := filepath.Join(tmp, "stack2.mp4")
	panels := []PanelInput{
		{Path: p1},
		{Path: p2},
	}
	require.NoError(t, c.StackPanels(ctx, out, panels, 0))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	// 2-panel vstack: total height = 640*2 = 1280
	require.Equal(t, 720, vs.Width)
	require.Equal(t, 1280, vs.Height)
	require.True(t, hasAudioStream(ps), "2-panel stack must have audio")
}

func TestStackPanels_4(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(4) // 360×640

	panels := make([]PanelInput, 4)
	srcs := []string{clipRed10s, clipBlue8s, clipGreen5s, clipRed10s}
	for i, src := range srcs {
		p := makePanel(t, ctx, c, src, dims, filepath.Join(tmp, fmt.Sprintf("p%d.mp4", i)))
		panels[i] = PanelInput{Path: p}
	}

	out := filepath.Join(tmp, "stack4.mp4")
	require.NoError(t, c.StackPanels(ctx, out, panels, 0))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	// 2×2 grid: 360*2=720 wide, 640*2=1280 tall
	require.Equal(t, 720, vs.Width)
	require.Equal(t, 1280, vs.Height)
}

func TestStackPanels_AudioRotation(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(2)

	p0 := makePanel(t, ctx, c, clipRed10s, dims, filepath.Join(tmp, "p0.mp4"))
	p1 := makePanel(t, ctx, c, clipBlue8s, dims, filepath.Join(tmp, "p1.mp4"))

	out := filepath.Join(tmp, "stack_audio1.mp4")
	panels := []PanelInput{
		{Path: p0},
		{Path: p1},
	}
	// audioIdx=1 → use panel 1's audio
	require.NoError(t, c.StackPanels(ctx, out, panels, 1))

	ps := probe(t, out)
	require.True(t, hasAudioStream(ps), "output must have audio stream from panel 1")
}

func TestConcatSections(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(2)

	// Build two section files from normalized clips.
	s1 := makePanel(t, ctx, c, clipRed10s, dims, filepath.Join(tmp, "s1.mp4"))
	s2 := makePanel(t, ctx, c, clipGreen5s, dims, filepath.Join(tmp, "s2.mp4"))

	d1 := parseDuration(t, probe(t, s1))
	d2 := parseDuration(t, probe(t, s2))

	out := filepath.Join(tmp, "final.mp4")
	require.NoError(t, c.ConcatSections(ctx, out, []string{s1, s2}))

	ps := probe(t, out)
	total := parseDuration(t, ps)
	expected := d1 + d2
	require.True(t, math.Abs(total-expected) < 1.0,
		"concat duration %.2f should be close to %.2f", total, expected)
}

// ─── escapeDrawtext + titleFontSize ──────────────────────────────────────────

func TestEscapeDrawtext(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"sleeping", "sleeping"},
		{"10:30 AM", `10\:30 AM`},
		{`a\b`, `a\\b`},
		{"it's", `it\'s`},
		{`it's 10:30\n`, `it\'s 10\:30\\n`},
	}
	for _, tc := range tests {
		got := escapeDrawtext(tc.in)
		require.Equal(t, tc.want, got, "escapeDrawtext(%q)", tc.in)
	}
}

func TestTitleFontSize(t *testing.T) {
	tests := []struct {
		dims PanelDims
		want int
	}{
		{PanelDims{720, 1280}, 64},
		{PanelDims{720, 640}, 32},
		{PanelDims{720, 427}, 21},
		{PanelDims{360, 640}, 32},
		{PanelDims{100, 200}, 18}, // clamped minimum
	}
	for _, tc := range tests {
		got := titleFontSize(tc.dims)
		require.Equal(t, tc.want, got, "titleFontSize(%v)", tc.dims)
	}
}

// ─── OverlayTitle ────────────────────────────────────────────────────────────

func TestOverlayTitle_EmptyTitle(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	panel := makePanel(t, ctx, c, clipGreen5s, PanelDimsFor(2), filepath.Join(tmp, "p.mp4"))

	out := filepath.Join(tmp, "titled.mp4")
	require.NoError(t, c.OverlayTitle(ctx, panel, out, "", PanelDimsFor(2)))

	// Empty title → symlink, not re-encode.
	target, err := os.Readlink(out)
	require.NoError(t, err)
	require.Equal(t, panel, target)
}

func TestOverlayTitle_NoDrawtext(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()

	c := NewWithBin("ffmpeg", "ffprobe") // hasDrawtext=false by default

	tmp := t.TempDir()
	panel := makePanel(t, ctx, c, clipGreen5s, PanelDimsFor(2), filepath.Join(tmp, "p.mp4"))

	out := filepath.Join(tmp, "titled.mp4")
	require.NoError(t, c.OverlayTitle(ctx, panel, out, "sleeping", PanelDimsFor(2)))

	// Without drawtext → symlink fallback.
	target, err := os.Readlink(out)
	require.NoError(t, err)
	require.Equal(t, panel, target)
}

func TestOverlayTitle_WithDrawtext(t *testing.T) {
	requireDrawtext(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(2)
	panel := makePanel(t, ctx, c, clipGreen5s, dims, filepath.Join(tmp, "p.mp4"))

	out := filepath.Join(tmp, "titled.mp4")
	require.NoError(t, c.OverlayTitle(ctx, panel, out, "sleeping", dims))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, dims.Width, vs.Width)
	require.Equal(t, dims.Height, vs.Height)
	require.True(t, hasAudioStream(ps), "audio must be preserved")
}

func TestOverlayTitle_SpecialChars(t *testing.T) {
	requireDrawtext(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)

	tmp := t.TempDir()
	dims := PanelDimsFor(2)
	panel := makePanel(t, ctx, c, clipGreen5s, dims, filepath.Join(tmp, "p.mp4"))

	out := filepath.Join(tmp, "titled.mp4")
	require.NoError(t, c.OverlayTitle(ctx, panel, out, "it's 10:30", dims))

	ps := probe(t, out)
	vs := videoStream(t, ps)
	require.Equal(t, dims.Width, vs.Width)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// makePanel normalizes a clip and scales it to target dims.
// Audio is preserved through both steps (no silent-audio mux hack needed).
func makePanel(t *testing.T, ctx context.Context, c *Composer, src string, dims PanelDims, out string) string {
	t.Helper()
	norm := out + ".norm.mp4"
	require.NoError(t, c.NormalizeClip(ctx, src, norm))
	require.NoError(t, c.ScaleClip(ctx, norm, out, dims))
	return out
}
