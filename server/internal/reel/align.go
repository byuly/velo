package reel

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/byuly/velo/server/internal/ffmpeg"
	"github.com/google/uuid"
)

// AlignResult contains a ComposeRequest and the set of clips that need
// downloading and normalizing before composition can begin.
type AlignResult struct {
	Request      ffmpeg.ComposeRequest
	ClipsToFetch []ClipFetch
}

// ClipFetch describes one clip that must be downloaded from S3 and normalized.
type ClipFetch struct {
	S3Key    string // S3 object key (source)
	RawPath  string // local download target
	NormPath string // local normalization output (referenced by ComposeRequest)
}

// Align maps denormalized session data to an FFmpeg ComposeRequest.
// Returns nil if no active participants have clips (zero-submitter edge case).
// This is a pure function — no I/O.
func Align(data *SessionData, workDir string) *AlignResult {
	// 1. Index: which users have clips?
	userHasClips := make(map[uuid.UUID]bool)
	for _, c := range data.Clips {
		userHasClips[c.UserID] = true
	}

	// 2. Filter participants: active + has clips.
	var active []ParticipantRow
	for _, p := range data.Participants {
		if p.Status == "active" && userHasClips[p.UserID] {
			active = append(active, p)
		}
	}

	// 3. Zero-submitter check.
	if len(active) == 0 {
		return nil
	}

	// 4. Order: creator first, then by joined_at.
	creatorID := data.Session.CreatorID
	sort.SliceStable(active, func(i, j int) bool {
		iCreator := creatorID != nil && active[i].UserID == *creatorID
		jCreator := creatorID != nil && active[j].UserID == *creatorID
		if iCreator != jCreator {
			return iCreator
		}
		return active[i].JoinedAt.Before(active[j].JoinedAt)
	})

	// 5. Build lookup maps.
	clipsBySlotUser := make(map[uuid.UUID]map[uuid.UUID][]ClipRow)
	for _, c := range data.Clips {
		byUser, ok := clipsBySlotUser[c.SlotID]
		if !ok {
			byUser = make(map[uuid.UUID][]ClipRow)
			clipsBySlotUser[c.SlotID] = byUser
		}
		byUser[c.UserID] = append(byUser[c.UserID], c)
	}

	participationBySlotUser := make(map[uuid.UUID]map[uuid.UUID]ParticipationRow)
	for _, sp := range data.Participations {
		byUser, ok := participationBySlotUser[sp.SlotID]
		if !ok {
			byUser = make(map[uuid.UUID]ParticipationRow)
			participationBySlotUser[sp.SlotID] = byUser
		}
		byUser[sp.UserID] = sp
	}

	// 6. Build sections.
	sectionDuration := float64(data.Session.MaxSectionDurationS)
	var sections []ffmpeg.SectionRequest
	var allFetches []ClipFetch
	seen := make(map[uuid.UUID]bool) // deduplicate clips across lookups

	for slotIdx, slot := range data.Slots {
		var participants []ffmpeg.ParticipantPanel

		for _, p := range active {
			panel := ffmpeg.ParticipantPanel{
				Name:     p.DisplayNameSnapshot,
				Duration: sectionDuration,
			}

			// Check participation for skip/title.
			if byUser, ok := participationBySlotUser[slot.ID]; ok {
				if sp, ok := byUser[p.UserID]; ok {
					if sp.Status == "skipped" {
						panel.LocalPaths = nil
						if sp.Title != nil {
							panel.Title = *sp.Title
						}
						participants = append(participants, panel)
						continue
					}
					if sp.Title != nil {
						panel.Title = *sp.Title
					}
				}
			}

			// Collect clips for this participant in this slot.
			if byUser, ok := clipsBySlotUser[slot.ID]; ok {
				if clips, ok := byUser[p.UserID]; ok {
					for _, c := range clips {
						normPath := filepath.Join(workDir, "norm", fmt.Sprintf("%s.mp4", c.ID))
						panel.LocalPaths = append(panel.LocalPaths, normPath)

						if !seen[c.ID] {
							seen[c.ID] = true
							allFetches = append(allFetches, ClipFetch{
								S3Key:    c.S3Key,
								RawPath:  filepath.Join(workDir, "raw", fmt.Sprintf("%s.mp4", c.ID)),
								NormPath: normPath,
							})
						}
					}
				}
			}

			participants = append(participants, panel)
		}

		sections = append(sections, ffmpeg.SectionRequest{
			Participants: participants,
			AudioIdx:     slotIdx % len(active),
		})
	}

	return &AlignResult{
		Request: ffmpeg.ComposeRequest{
			WorkDir:  workDir,
			Sections: sections,
		},
		ClipsToFetch: allFetches,
	}
}
