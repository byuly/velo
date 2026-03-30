package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// paddingToleranceSec is the minimum gap (in seconds) between a panel's actual
// duration and the section duration before black-panel padding is appended.
// Avoids generating sub-frame padding for negligible rounding differences.
const paddingToleranceSec = 0.1

// ComposeRequest describes the inputs for a reel composition.
// All clip paths must point to pre-normalized files on local disk.
// WorkDir is a job-scoped temp directory; the caller owns its lifecycle.
type ComposeRequest struct {
	WorkDir  string
	Sections []SectionRequest
}

// SectionRequest describes one section (slot) of the reel.
type SectionRequest struct {
	Participants []ParticipantPanel
	AudioIdx     int // which participant's audio to keep (rotated per section)
}

// ParticipantPanel describes one participant's contribution to a section.
type ParticipantPanel struct {
	LocalPaths []string // pre-normalized clip files on disk
	Name       string   // display name (for black panel overlay, future)
	Title      string   // per-participant section title (e.g. "sleeping"); empty = no overlay
	Duration   float64  // section max duration in seconds
}

// Engine orchestrates the multi-step FFmpeg composition pipeline.
// It uses a Composer for all low-level FFmpeg operations.
type Engine struct {
	composer *Composer
}

// NewEngine creates an Engine backed by the given Composer.
func NewEngine(c *Composer) *Engine {
	return &Engine{composer: c}
}

// Compose produces a final reel from the given request.
// Returns the path to the output file inside req.WorkDir.
func (e *Engine) Compose(ctx context.Context, req ComposeRequest) (string, error) {
	if len(req.Sections) == 0 {
		return "", fmt.Errorf("compose: no sections provided")
	}

	var sectionFiles []string

	for sIdx, section := range req.Sections {
		if len(section.Participants) == 0 {
			return "", fmt.Errorf("compose: section %d has no participants", sIdx)
		}

		dims := PanelDimsFor(len(section.Participants))
		var panels []PanelInput

		for pIdx, p := range section.Participants {
			panelPath, err := e.buildParticipantPanel(ctx, req.WorkDir, sIdx, pIdx, p, dims)
			if err != nil {
				return "", fmt.Errorf("section %d participant %d: %w", sIdx, pIdx, err)
			}
			panels = append(panels, PanelInput{Path: panelPath})
		}

		sectionFile := filepath.Join(req.WorkDir, fmt.Sprintf("section_%d.mp4", sIdx))
		if err := e.composer.StackPanels(ctx, sectionFile, panels, section.AudioIdx); err != nil {
			return "", fmt.Errorf("stack section %d: %w", sIdx, err)
		}
		sectionFiles = append(sectionFiles, sectionFile)
	}

	outputPath := filepath.Join(req.WorkDir, "reel.mp4")

	if len(sectionFiles) == 1 {
		if err := os.Rename(sectionFiles[0], outputPath); err != nil {
			return "", fmt.Errorf("rename single section: %w", err)
		}
	} else {
		if err := e.composer.ConcatSections(ctx, outputPath, sectionFiles); err != nil {
			return "", fmt.Errorf("concat sections: %w", err)
		}
	}

	return outputPath, nil
}

// buildParticipantPanel produces a single participant's panel for one section.
// If the participant has no clips, a full-duration black panel is generated.
// Otherwise clips are scaled, concatenated, and padded to the section duration.
func (e *Engine) buildParticipantPanel(
	ctx context.Context,
	workDir string,
	sIdx, pIdx int,
	p ParticipantPanel,
	dims PanelDims,
) (string, error) {
	prefix := fmt.Sprintf("s%d_p%d", sIdx, pIdx)

	if len(p.LocalPaths) == 0 {
		out := filepath.Join(workDir, prefix+"_black.mp4")
		if err := e.composer.GenerateBlackPanel(ctx, out, dims, p.Duration, p.Name); err != nil {
			return "", fmt.Errorf("generate black panel: %w", err)
		}
		if p.Title != "" {
			titled := filepath.Join(workDir, prefix+"_black_titled.mp4")
			if err := e.composer.OverlayTitle(ctx, out, titled, p.Title, dims); err != nil {
				return "", fmt.Errorf("overlay title on black panel: %w", err)
			}
			out = titled
		}
		return out, nil
	}

	// Scale each clip to target panel dimensions.
	var scaledPaths []string
	for i, clipPath := range p.LocalPaths {
		scaled := filepath.Join(workDir, fmt.Sprintf("%s_scaled_%d.mp4", prefix, i))
		if err := e.composer.ScaleClip(ctx, clipPath, scaled, dims); err != nil {
			return "", fmt.Errorf("scale clip %d: %w", i, err)
		}
		scaledPaths = append(scaledPaths, scaled)
	}

	// Concatenate multiple clips into a single panel stream.
	var panelPath string
	if len(scaledPaths) == 1 {
		panelPath = scaledPaths[0]
	} else {
		panelPath = filepath.Join(workDir, prefix+"_concat.mp4")
		if err := e.composer.ConcatSections(ctx, panelPath, scaledPaths); err != nil {
			return "", fmt.Errorf("concat clips: %w", err)
		}
	}

	// Pad with black if the panel is shorter than the section duration.
	pr, err := e.composer.Probe(ctx, panelPath)
	if err != nil {
		return "", fmt.Errorf("probe panel: %w", err)
	}

	if pr.Duration+paddingToleranceSec < p.Duration {
		remainder := p.Duration - pr.Duration
		blackPad := filepath.Join(workDir, prefix+"_pad.mp4")
		if err := e.composer.GenerateBlackPanel(ctx, blackPad, dims, remainder, p.Name); err != nil {
			return "", fmt.Errorf("generate padding: %w", err)
		}
		padded := filepath.Join(workDir, prefix+"_padded.mp4")
		if err := e.composer.ConcatSections(ctx, padded, []string{panelPath, blackPad}); err != nil {
			return "", fmt.Errorf("concat padding: %w", err)
		}
		panelPath = padded
	}

	// Apply title overlay as the final step of panel construction.
	if p.Title != "" {
		titled := filepath.Join(workDir, prefix+"_titled.mp4")
		if err := e.composer.OverlayTitle(ctx, panelPath, titled, p.Title, dims); err != nil {
			return "", fmt.Errorf("overlay title: %w", err)
		}
		panelPath = titled
	}

	return panelPath, nil
}
