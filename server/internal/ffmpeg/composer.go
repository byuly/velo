package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// escapeDrawtext escapes characters that have special meaning in FFmpeg
// drawtext filter option strings: backslash, single-quote, colon.
// Backslash must be escaped first to avoid double-escaping.
func escapeDrawtext(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	return s
}

// titleFontSize returns a font size appropriate for the given panel dimensions.
// Targets roughly 5% of panel height, clamped to [18, 64].
func titleFontSize(dims PanelDims) int {
	fs := dims.Height / 20
	if fs < 18 {
		fs = 18
	}
	if fs > 64 {
		fs = 64
	}
	return fs
}

// PanelDims holds the target width/height for a single participant panel.
type PanelDims struct{ Width, Height int }

// PanelDimsFor returns the correct panel dimensions for the given participant count.
// 1→720×1280  2→720×640  3→720×427  4→360×640
// Returns an error for unsupported counts (n <= 0 or n > 4).
func PanelDimsFor(n int) (PanelDims, error) {
	switch n {
	case 1:
		return PanelDims{720, 1280}, nil
	case 2:
		return PanelDims{720, 640}, nil
	case 3:
		return PanelDims{720, 427}, nil
	case 4:
		return PanelDims{360, 640}, nil
	default:
		return PanelDims{}, fmt.Errorf("unsupported participant count %d (must be 1-4)", n)
	}
}

// PanelInput is one pre-processed panel file feeding into StackPanels.
// All panels must contain an audio track (real or silent AAC).
type PanelInput struct {
	Path string // path to processed .mp4
}

// Composer wraps the ffmpeg and ffprobe binaries.
type Composer struct {
	ffmpegBin   string
	ffprobeBin  string
	hasDrawtext bool
}

// New creates a Composer using ffmpeg/ffprobe found in PATH.
func New() (*Composer, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found in PATH: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hasDT := probeDrawtext(ctx, ffmpeg)
	if !hasDT {
		slog.Warn("ffmpeg drawtext filter unavailable — title overlays will be skipped (install ffmpeg with libfreetype)")
	}

	return &Composer{ffmpegBin: ffmpeg, ffprobeBin: ffprobe, hasDrawtext: hasDT}, nil
}

// NewWithBin creates a Composer with explicit binary paths (for tests).
// Drawtext is disabled by default; use SetDrawtext to enable.
func NewWithBin(ffmpeg, ffprobe string) *Composer {
	return &Composer{ffmpegBin: ffmpeg, ffprobeBin: ffprobe}
}

// SetDrawtext enables or disables drawtext support (for tests).
func (c *Composer) SetDrawtext(enabled bool) { c.hasDrawtext = enabled }

// HasDrawtext reports whether the drawtext filter is available.
func (c *Composer) HasDrawtext() bool { return c.hasDrawtext }

// probeDrawtext returns true if the ffmpeg binary supports the drawtext filter.
func probeDrawtext(ctx context.Context, ffmpegBin string) bool {
	cmd := exec.CommandContext(ctx, ffmpegBin, "-filters")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "drawtext")
}

// NormalizeClip normalizes VFR→CFR at 30fps at the original resolution.
// Audio is preserved. This is Phase 1 of the two-phase pipeline (eager,
// runs at slot-end). Scaling is deferred to ScaleClip (Phase 2, at deadline).
func (c *Composer) NormalizeClip(ctx context.Context, input, output string) error {
	return c.run(ctx,
		"-y",
		"-i", input,
		"-vf", "fps=30",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		output,
	)
}

// ScaleClip scales a pre-normalized clip to the target panel dimensions with a
// lightweight re-encode (input is already CRF 23 from Phase 1). Audio is
// stream-copied. This is Phase 2 of the two-phase pipeline (runs at deadline
// when participant count is known).
func (c *Composer) ScaleClip(ctx context.Context, input, output string, dims PanelDims) error {
	vf := fmt.Sprintf("scale=%d:%d", dims.Width, dims.Height)
	return c.run(ctx,
		"-y",
		"-i", input,
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "copy",
		output,
	)
}

// GenerateBlackPanel creates a silent black panel of the given duration with
// a silent AAC audio track. The name parameter is reserved for a future
// drawtext overlay (requires libfreetype / brew install ffmpeg-full).
func (c *Composer) GenerateBlackPanel(ctx context.Context, output string,
	dims PanelDims, duration float64, name string) error {

	colorSrc := fmt.Sprintf("color=c=black:s=%dx%d:r=30", dims.Width, dims.Height)

	return c.run(ctx,
		"-y",
		"-f", "lavfi", "-i", colorSrc,
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
		"-t", fmt.Sprintf("%.3f", duration),
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		output,
	)
}

// StackPanels stacks N panels into a single section video.
// audioIdx selects which panel's audio track is kept in the output.
func (c *Composer) StackPanels(ctx context.Context, output string,
	panels []PanelInput, audioIdx int) error {

	args := []string{"-y"}
	for _, p := range panels {
		args = append(args, "-i", p.Path)
	}

	filterComplex, videoLabel, audioLabel := buildFilterGraph(len(panels), audioIdx)
	args = append(args,
		"-filter_complex", filterComplex,
		"-map", videoLabel,
		"-map", audioLabel,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		output,
	)

	return c.run(ctx, args...)
}

// ConcatSections concatenates section videos into the final reel using the
// concat demuxer. A temporary list file is written to disk because the concat
// demuxer's pipe: protocol is not on its own whitelist.
func (c *Composer) ConcatSections(ctx context.Context, output string, sections []string) error {
	listFile, err := os.CreateTemp("", "ffmpeg-concat-*.txt")
	if err != nil {
		return fmt.Errorf("create concat list: %w", err)
	}
	defer os.Remove(listFile.Name())

	for _, s := range sections {
		escaped := strings.ReplaceAll(s, "'", "'\\''")
		fmt.Fprintf(listFile, "file '%s'\n", escaped)
	}
	if err := listFile.Close(); err != nil {
		return fmt.Errorf("close concat list: %w", err)
	}

	return c.run(ctx,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",
		output,
	)
}

// OverlayTitle burns a centered title onto a video clip using drawtext.
// If drawtext is unavailable or title is empty, the input is symlinked to
// the output path (no-op) to keep the pipeline uniform.
func (c *Composer) OverlayTitle(ctx context.Context, input, output, title string, dims PanelDims) error {
	if !c.hasDrawtext || title == "" {
		return os.Symlink(input, output)
	}

	escaped := escapeDrawtext(title)
	fontSize := titleFontSize(dims)

	vf := fmt.Sprintf(
		"drawtext=text='%s':fontcolor=white:fontsize=%d:x=(w-text_w)/2:y=(h-text_h)/2:box=1:boxcolor=black@0.5:boxborderw=8",
		escaped, fontSize,
	)

	return c.run(ctx,
		"-y",
		"-i", input,
		"-vf", vf,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "copy",
		output,
	)
}

// buildFilterGraph constructs the -filter_complex string and output map labels.
// n is the number of input panels; audioIdx selects the audio source.
// Returns (filterComplex, videoLabel, audioLabel).
func buildFilterGraph(n, audioIdx int) (string, string, string) {
	switch n {
	case 1:
		return "[0:v]null[v];[0:a]anull[a]", "[v]", "[a]"

	case 2:
		filter := fmt.Sprintf(
			"[0:v][1:v]vstack=inputs=2[v];[%d:a]anull[a]",
			audioIdx,
		)
		return filter, "[v]", "[a]"

	case 3:
		filter := fmt.Sprintf(
			"[0:v][1:v][2:v]vstack=inputs=3[v];[%d:a]anull[a]",
			audioIdx,
		)
		return filter, "[v]", "[a]"

	case 4:
		filter := fmt.Sprintf(
			"[0:v][1:v]hstack=inputs=2[top];[2:v][3:v]hstack=inputs=2[bot];[top][bot]vstack=inputs=2[v];[%d:a]anull[a]",
			audioIdx,
		)
		return filter, "[v]", "[a]"

	default:
		// Fallback: vstack all panels.
		var vInputs strings.Builder
		for i := range n {
			fmt.Fprintf(&vInputs, "[%d:v]", i)
		}
		filter := fmt.Sprintf(
			"%svstack=inputs=%d[v];[%d:a]anull[a]",
			vInputs.String(), n, audioIdx,
		)
		return filter, "[v]", "[a]"
	}
}

// ProbeResult holds metadata extracted from a media file via ffprobe.
type ProbeResult struct {
	Duration float64
	Width    int
	Height   int
	HasAudio bool
}

// Probe inspects a media file and returns its video metadata.
func (c *Composer) Probe(ctx context.Context, path string) (ProbeResult, error) {
	cmd := exec.CommandContext(ctx, c.ffprobeBin,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe %s: %w", path, err)
	}

	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return ProbeResult{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var result ProbeResult
	for _, s := range payload.Streams {
		switch s.CodecType {
		case "video":
			result.Width = s.Width
			result.Height = s.Height
			if s.Duration != "" {
				fmt.Sscanf(s.Duration, "%f", &result.Duration)
			}
		case "audio":
			result.HasAudio = true
		}
	}
	return result, nil
}

// run executes ffmpeg with the given args, captures combined stdout+stderr,
// and returns a wrapped error with output on non-zero exit.
func (c *Composer) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, c.ffmpegBin, args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		preview := strings.Join(args, " ")
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		return fmt.Errorf("ffmpeg [%s] failed: %w\n%s", preview, err, combined.String())
	}
	return nil
}
