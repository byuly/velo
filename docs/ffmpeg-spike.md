# FFmpeg Spike — Issue #5

De-risks the multi-pass reel pipeline before Task 19 implementation.
Validated on **FFmpeg 8.0.1** (macOS arm64, Homebrew).

---

## 1. Setup

```bash
brew install ffmpeg   # installs ffmpeg + ffprobe
ffmpeg -version       # confirm 8.0.1
ffprobe -version
```

> **Note on drawtext**: The standard `brew install ffmpeg` formula does **not** include
> `libfreetype`, so the `drawtext` filter is unavailable. Timestamp overlays require
> `brew install ffmpeg-full`. The `NormalizeClip` and `GenerateBlackPanel` functions
> accept a `timestamp`/`name` parameter that is currently reserved for when the
> full formula is present.

### Generate test fixtures

```bash
# Portrait red clip, 10s, simulated VFR
ffmpeg -f lavfi -i "color=c=red:s=1080x1920:r=30" \
       -f lavfi -i "sine=frequency=440:sample_rate=44100" \
       -t 10 -c:v libx264 -preset fast -crf 23 \
       -c:a aac -b:a 128k -ac 2 -vsync vfr \
       clip_red_10s.mp4

# Portrait blue clip, 8s, rotation metadata
ffmpeg -f lavfi -i "color=c=blue:s=1080x1920:r=30" \
       -f lavfi -i "sine=frequency=660:sample_rate=44100" \
       -t 8 -c:v libx264 -preset fast -crf 23 \
       -c:a aac -b:a 128k -ac 2 \
       -metadata:s:v:0 rotate=90 \
       clip_blue_8s.mp4

# Smaller green clip, 5s, no rotation
ffmpeg -f lavfi -i "color=c=green:s=720x1280:r=30" \
       -f lavfi -i "sine=frequency=880:sample_rate=44100" \
       -t 5 -c:v libx264 -preset fast -crf 23 \
       -c:a aac -b:a 128k -ac 2 \
       clip_green_5s.mp4
```

The test suite (`composer_test.go` → `TestMain`) generates these automatically into a
temp dir so no binaries are committed.

---

## 2. iPhone Quirks

### VFR normalization

iPhone records at **variable frame rate** (30/60fps burst, ProMotion, etc.). FFmpeg filter
graphs require a fixed-rate stream or they stall/duplicate frames unpredictably.

**Correct approach (FFmpeg ≥ 5):**
```
-vf fps=30
```

**Deprecated** (still works but warns):
```
-vsync cfr
```

The `fps` filter is inserted first in the `-vf` chain before `scale` to avoid processing
extra frames.

### Rotation metadata

Modern iPhones record in **landscape sensor orientation** and embed a `rotate` tag
(90°, 180°, or 270°). FFmpeg 4+ enables `autorotate` by default so `-i input.mp4` is
decoded already rotated correctly. The `scale` filter then produces the expected
portrait dimensions.

If you need to disable autorotate (e.g. to inspect raw sensor output):
```bash
ffmpeg -noautorotate -i input.mp4 ...
```

For older FFmpeg builds without autorotate, use the `transpose` filter:
```
# 90° CW → 1, 90° CCW → 2
-vf "transpose=1,scale=W:H"
```

---

## 3. Pass Commands

### Pass 1 — Normalize

VFR → CFR 30fps, scale to panel dimensions, strip audio:

```bash
ffmpeg -y -i input.mp4 \
  -vf "fps=30,scale=720:640" \
  -an \
  -c:v libx264 -preset fast -crf 23 \
  normalized.mp4
```

With timestamp overlay (requires `ffmpeg-full`):
```bash
ffmpeg -y -i input.mp4 \
  -vf "fps=30,scale=720:640,drawtext=text='00\:30':fontcolor=white:fontsize=24:x=10:y=10:box=1:boxcolor=black@0.5:boxborderw=5" \
  -an -c:v libx264 -preset fast -crf 23 \
  normalized.mp4
```

### Pass 2 — Black panel

Silent black panel with centered name (requires `ffmpeg-full` for drawtext):

```bash
ffmpeg -y \
  -f lavfi -i "color=c=black:s=720x640:r=30" \
  -f lavfi -i "anullsrc=r=44100:cl=stereo" \
  -t 8.000 \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  black_panel.mp4
```

### Pass 3 — Stack panels

**2 panels (vstack):**
```bash
ffmpeg -y -i panel0.mp4 -i panel1.mp4 \
  -filter_complex "[0:v][1:v]vstack=inputs=2[v];[0:a]anull[a]" \
  -map "[v]" -map "[a]" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  section.mp4
```

**4 panels (2×2 grid):**
```bash
ffmpeg -y -i p0.mp4 -i p1.mp4 -i p2.mp4 -i p3.mp4 \
  -filter_complex \
    "[0:v][1:v]hstack=inputs=2[top];[2:v][3:v]hstack=inputs=2[bot];[top][bot]vstack=inputs=2[v];[0:a]anull[a]" \
  -map "[v]" -map "[a]" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  section.mp4
```

### Pass 4 — Concat

```bash
# concat.txt:
#   file '/abs/path/section1.mp4'
#   file '/abs/path/section2.mp4'

ffmpeg -y -f concat -safe 0 -i concat.txt -c copy final_reel.mp4
```

> **Important**: use absolute paths in the concat list and `-safe 0`. The `pipe:` protocol
> is **not** on the concat demuxer's whitelist, so the list must be written to a temp file
> on disk rather than fed via stdin.

---

## 4. Filter Graph Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Wrong label name | `Output pad "v" of the filter graph is not connected` | Use `[v]`/`[a]` consistently across `-filter_complex` and `-map` |
| `inputs=N` mismatch | `[vstack @ ...] Requested N inputs but received M` | Count `-i` flags exactly; each input needs a corresponding `[N:v]` label |
| Audio stream missing | `[audioIdx:a] does not contain audio stream` | Run `GenerateBlackPanel` (adds silent AAC) before `StackPanels` |
| Pipe protocol blocked | `Protocol 'pipe' not on whitelist 'crypto,data'` | Write concat list to a temp file; do not use `pipe:0` with `-f concat` |
| `drawtext` not found | `No such filter: 'drawtext'` | Standard `brew install ffmpeg` lacks libfreetype; use `brew install ffmpeg-full` |
| VFR in filter graph | Frame duplication / sync drift | Always place `fps=30` first in the `-vf` chain |

---

## 5. Go Integration

The `internal/ffmpeg` package wraps all four passes:

```
server/internal/ffmpeg/
  composer.go        — Composer struct, all pass methods, buildFilterGraph, run helper
  composer_test.go   — TestMain generates fixtures; 8 tests cover all passes
```

Key types:
- `PanelDims` — width/height for one panel slot
- `PanelDimsFor(n int)` — returns correct dims for 1/2/3/4 participants
- `PanelInput` — path + HasAudio flag fed into `StackPanels`
- `Composer` — created via `New()` (PATH) or `NewWithBin(ffmpeg, ffprobe)` (tests)

Task 19 (`service/reel.go`) will define the `ReelComposer` interface and wire in this
package as the concrete implementation.

---

## 6. Spike Results

All 8 tests pass (`go test ./internal/ffmpeg/...`):

| Test | What it covers |
|------|---------------|
| `TestNormalizeClip_basic` | VFR→CFR 30fps, scale to panel dims, audio stripped |
| `TestNormalizeClip_withRotation` | `rotate=90` metadata respected via autorotate |
| `TestGenerateBlackPanel_basic` | Silent black panel, exact duration, AAC stream present |
| `TestGenerateBlackPanel_duration` | Duration matches slot length within 100ms |
| `TestConcatSections_two` | Two normalized clips concatenated losslessly |
| `TestConcatSections_three` | Three-file concat, frame count matches sum |
| `TestStackPanels_two` | 2-panel vstack, correct output resolution, audio from panel 0 |
| `TestStackPanels_four` | 4-panel 2×2 grid, correct output resolution, audio from panel 2 |

### Issues discovered and resolved

| Issue | Root Cause | Fix |
|-------|-----------|-----|
| `drawtext` not found | Standard `brew install ffmpeg` lacks libfreetype | Removed drawtext from default path; `timestamp`/`name` params reserved for `brew install ffmpeg-full` |
| `00:00` broke filter string | Colon is FFmpeg option delimiter; must escape as `\:` in drawtext text values | `escapeDrawtext()` helper replaces `:` → `\:` |
| `pipe:` protocol blocked in concat | concat demuxer whitelist excludes `pipe:` | Write list to `os.CreateTemp` file on disk, remove after run |

---

## 7. Reel Engine Architecture

Maps PRD §5.3 to concrete Go code across Tasks 17–19.

### 7.1 — Data model → reel structure

```
Session
  ├── Slots [ordered by slot_order] — each slot = one Section in the reel
  │     └── Participants [creator first, then joined_at order]
  │           └── Clips [filtered to slot time window by recorded_at, sorted]
  └── Participants [active only, excludes zero-clip + deleted accounts]
```

### 7.2 — Composition algorithm

PRD §5.3 steps mapped to Composer methods:

```
func compose(session, slots, participants, clips, workDir) → []sectionFile {
  for sectionIdx, slot := range slots {
    dims := PanelDimsFor(len(participants))
    audioIdx := sectionIdx % len(participants)   // rotate audio source per section

    var panels []PanelInput
    for _, p := range participants {
      slotClips := clipsForParticipantInSlot(p, slot, clips)

      if len(slotClips) == 0 {
        // Pass 2: black panel for full slot duration
        f := GenerateBlackPanel(dims, slot.Duration, p.DisplayName)
        panels = append(panels, PanelInput{Path: f, HasAudio: true})
      } else {
        // Pass 1: normalize each clip, then concat them for this participant's panel
        var normFiles []string
        for _, clip := range slotClips {
          f := NormalizeClip(s3Download(clip.S3Key), dims, formatTimestamp(clip.RecordedAt))
          normFiles = append(normFiles, f)
        }
        panelFile := ConcatSections(normFiles)  // concat participant's clips
        // pad remainder with black if total < slot duration
        panels = append(panels, PanelInput{Path: panelFile, HasAudio: true})
      }
    }
    // Pass 3: stack all participant panels for this section
    sectionFile := StackPanels(panels, audioIdx)
    sections = append(sections, sectionFile)
  }
  // Pass 4: concat all sections into final reel
  return ConcatSections(sections)
}
```

### 7.3 — Job pipeline (Tasks 17–19)

```
[Scheduler Task 18]                  [Queue Task 17]          [ReelProcessor Task 19]
  Tick() every 30s                     Redis LIST               Process(sessionID)
  → find sessions past deadline   →    LPUSH jobID        →    download clips from S3
  → SET status=generating              BRPOP jobID              run compose() above
  → enqueue job                                                  upload reel to S3
                                                                 SET status=complete
                                                                 push notification
                                                                 retry ≤3 on failure
```

### 7.4 — ReelComposer interface (Task 19 boundary)

```go
// In internal/service/reel.go (Task 19):
type ReelComposer interface {
    Compose(ctx context.Context, req ComposeRequest) (outputPath string, err error)
}

type ComposeRequest struct {
    SessionID   uuid.UUID
    WorkDir     string           // job-scoped temp dir, caller owns cleanup
    Sections    []SectionRequest
}

type SectionRequest struct {
    Participants []ParticipantPanel
    AudioIdx     int
}

type ParticipantPanel struct {
    LocalPaths  []string  // pre-downloaded clip files (normalized order)
    Name        string    // display name for black panel
    Duration    float64   // slot duration in seconds
}
```

`internal/ffmpeg.Composer` implements `ReelComposer`. The stub in `reel_stub.go` returns a fake URL.

### 7.5 — Temp file management

Each reel job owns one `os.MkdirTemp("", "reel-<sessionID>-*")` directory.
All intermediate files (normalized clips, panel videos, section videos) live here.
The processor cleans it up with `defer os.RemoveAll(workDir)` after S3 upload completes.

### 7.6 — Audio rotation

One participant's audio is kept per section; all others are silenced.
Rotation formula: `audioIdx = sectionIdx % len(participants)`.
This ensures each participant's mic is heard roughly equally across a multi-section reel.

### 7.7 — Task build order

```
Task 17 (queue)  →  Task 18 (scheduler)  →  Task 19 (processor + ReelComposer interface)
                                                    ↑
                                         wire in internal/ffmpeg.Composer
```

---

## 8. Recording Engine Architecture

End-to-end design covering the full clip lifecycle: iOS capture → client upload → server ingestion → FFmpeg composition.

### 8.1 — Overview

```
[iPhone: Hold button]
  → CameraManager (AVFoundation)
      → AVCaptureSession (.hd1280x720, landscape sensor + rotate=90 metadata)
      → AVCaptureMovieFileOutput → tmp/clip-{uuid}.mov
      → recorded_at captured at didFinishRecordingTo callback

[iPhone: Confirm tap]
  → ClipUploadService
      → POST /sessions/{id}/clips/upload-url  → { upload_url, s3_key }
      → PUT {upload_url} raw .mov             → S3: velo-clips
      → POST /sessions/{id}/clips             → confirmed Clip row in DB

[Server: Slot ends_at + 10 min grace]          ← Phase 1 (eager)
  → Scheduler (Task 18) detects slot end
  → SlotNormalizer downloads raw clips for that slot
      → NormalizeClip at 720×1280 (VFR→CFR 30fps, CRF 23, no scale)
      → Upload normalized clip to S3: normalized/{sessionID}/{clipID}.mp4
      → SET clips.normalized_s3_key in DB

[Server: Deadline hit]                          ← Phase 2 (fast)
  → Scheduler (Task 18) detects deadline
  → Enqueues sessionID to Redis LIST (Task 17)
  → ReelProcessor (Task 19) pulls job:
      → Download pre-normalized clips (already small, ~1-2 Mbps)
      → Scale to PanelDimsFor(final participant count)
      → Stack panels + concat sections
      → S3 upload reel → velo-reels + CloudFront
      → SET sessions.status = complete, reel_url = CDN URL
      → Push notify all participants
```

---

### 8.2 — Capture layer

**Portrait orientation strategy**

The reel output is portrait `720×1280`. Two strategies exist for capturing portrait on iPhone:

| Strategy | How | Risk |
|----------|-----|------|
| **A — metadata (chosen)** | `.hd1280x720` preset. Sensor runs landscape. Device embeds `rotate=90` in file. FFmpeg autorotate decodes portrait. | None — this is the standard iPhone recording path |
| B — force portrait | Set `AVCaptureConnection.videoOrientation = .portrait`. ISP rotates frames in software before encode. | Green-frame artifacts at clip head on older A-series chips due to buffer reuse at capture start |

Strategy A is chosen. The FFmpeg spike already proves it: `TestNormalizeClip_withRotation` passes with `rotate=90` metadata and produces correct `720×1280` output.

**Capture preset: `.hd1280x720`**

- Raw sensor frame: `1280×720` (landscape)
- After autorotate decode: `720×1280` (portrait)
- Matches `PanelDimsFor(1)` exactly → no upscaling ever needed
- Multi-participant panels are all ≤ 720px wide → always scaling down

Do not use `.hd1920x1080`. There is no perceptible quality gain for ≤30s casual clips; the larger buffer produces larger upload files with no downstream benefit — FFmpeg re-encodes at CRF 23 regardless.

**`recorded_at` capture point**

```swift
// AVCaptureFileOutputRecordingDelegate
func captureOutput(_ output: AVCaptureFileOutput,
                   didFinishRecordingTo url: URL,
                   from connections: [AVCaptureConnection],
                   error: Error?) {
    recordedAt = Date()   // ← captured here, not at button-press
}
```

The file buffer is not flushed until this callback fires. Capturing at button-press risks a timestamp 0.1–0.5s early. The server's timestamp clamping logic (`ClipTimestampTolerance = 30 min`) is generous for clock drift, but the `recorded_at` value must be as accurate as possible for slot alignment.

---

### 8.3 — Compression decision

**No client-side compression for v1. Upload the raw `.mov` directly.**

At `.hd1280x720`, the iPhone encodes at roughly 8–12 Mbps. A 30s clip is ~30–45 MB.

`AVAssetExportSession` with `.AVAssetExportPreset1280x720` does not reduce this meaningfully — it re-encodes at the same resolution with a similar bitrate. The only way to reduce bitrate client-side is via `AVAssetWriter` + `AVVideoCompressionPropertiesKey` targeting a specific Mbps (e.g. 3 Mbps). This is significantly more complex and adds 15–30s of post-recording wait time on-device.

The real bitrate reduction happens server-side: `NormalizeClip` re-encodes at libx264 CRF 23, which typically yields 1–2 Mbps for portrait content — a 5–8× reduction.

**v1.1 revisit trigger:** If beta users on slow 4G report upload failures or wait times >30s for a 30s clip, introduce `AVAssetWriter`-based 3 Mbps compression post-capture.

---

### 8.4 — Client upload flow

```
POST /sessions/{sessionID}/clips/upload-url
  Auth: JWT (participant of session)
  Response: { "upload_url": "https://s3...", "s3_key": "clips/{sessionID}/{userID}/{uuid}.mp4" }

PUT {upload_url}
  No auth header — presigned URL carries AWS SigV4 in query params
  Content-Type: video/mp4
  Body: raw .mov bytes

POST /sessions/{sessionID}/clips
  Auth: JWT
  Body: { "s3_key": "...", "recorded_at": "ISO8601", "duration_ms": 12400, "slot_id": "uuid" }
  Response: 201 Clip | 200 Clip (duplicate, idempotent)
```

**Presigned URL expiry:** 15 minutes. Generous for slow connections; short enough to limit replay attacks.

**S3 key format:** `clips/{sessionID}/{userID}/{uuid}.mp4`
Scoped to session + user. The confirm endpoint validates the prefix — a user cannot confirm an upload belonging to a different user.

**Retry queue (CoreData)**

If step 3 (`POST /clips`) fails after a successful S3 PUT (network drop post-upload):

1. `ClipUploadService` writes a `PendingClipConfirmation` CoreData record: `{ s3Key, recordedAt, durationMs, slotId, sessionId, createdAt }`
2. UI shows "Uploaded — confirming..." instead of hard failure
3. On next app launch or session load: `AppState.retryPendingClipConfirmations()` iterates pending records, fires step 3 only (S3 PUT is already done), removes on success

Idempotent on backend: if `s3_key` already exists in `clips` table, return the existing row (`200 OK`).

---

### 8.5 — Server clip ingestion

**`POST /sessions/{sessionID}/clips/upload-url`**

Validation:
- Session `status = active`
- Requesting user is an `active` participant

**`POST /sessions/{sessionID}/clips`**

Validation (in order):

| Step | Rule |
|------|------|
| `s3_key` prefix check | Must begin with `clips/{sessionID}/{userID}/` — cross-user confirmation blocked |
| `duration_ms` | `> 0` and `≤ session.max_section_duration_s × 1000` |
| `slot_id` | Must be a valid `session_slots` row for this session |
| Duplicate check | If `s3_key` exists in DB → return existing clip (`200`) |
| `arrived_at` | S3 `HeadObject.LastModified` — NOT `time.Now()`. Captures actual upload time. |
| Timestamp clamp | If `\|recorded_at − arrived_at\| > 30 min`: set `recorded_at = arrived_at`, `recorded_at_clamped = true` |

`arrived_at` being `S3.HeadObject.LastModified` is critical. It reflects when the bytes actually landed in S3, not when the server processed the confirmation request (which could be minutes later due to CoreData retry). This makes the drift detection accurate.

---

### 8.6 — Two-phase FFmpeg pipeline

Processing is split into two phases to minimize deadline-to-reel latency. The expensive work (codec re-encode) runs eagerly as slots close; the layout-dependent work (scale + stack) runs at deadline when final participant count is known.

#### Phase 1 — Slot-end normalization (eager)

**Trigger:** Scheduler (Task 18) detects slot `ends_at + 10 min grace` has passed and the slot has un-normalized clips.

```
For each clip in the slot (per-clip, not per-participant):
  1. Download raw clip from S3 (velo-clips)
  2. NormalizeClip(path, 720×1280, timestamp)
       → VFR→CFR 30fps
       → libx264 CRF 23, preset fast
       → original portrait resolution (720×1280) — NO scaling
       → audio PRESERVED (needed for audio rotation in Phase 2)
  3. Upload normalized clip → S3: normalized/{sessionID}/{clipID}.mp4
  4. SET clips.normalized_s3_key = "normalized/..." in DB
```

**Why no scaling here:** Panel dimensions depend on final participant count (`PanelDimsFor(n)`), which isn't known until deadline. A 3rd participant could join after a slot closes. Normalizing at original resolution keeps the re-encode resolution-independent.

**Why no concatenation here:** A participant may have multiple clips in a slot. Concatenation order depends on `recorded_at` sorting, which is stable, but concatenation also requires knowing panel dims (for black-panel padding if total < max_section_duration). Defer to Phase 2.

**Grace period (10 minutes):** Covers slow uploads and CoreData retry queue. The scheduler ticks every 30s, so the slot-end + 10min condition is checked with fine granularity.

**What this saves:** The VFR→CFR + CRF 23 re-encode is the most CPU-intensive step (~80% of total pipeline time). A raw 30s clip at 8–12 Mbps re-encodes to ~1–2 Mbps. By the time the deadline fires, the heavy lifting is done.

#### Phase 2 — Deadline composition (fast)

**Trigger:** Scheduler (Task 18) detects session deadline has passed → enqueue to Redis LIST → ReelProcessor (Task 19) picks up the job.

```
dims := PanelDimsFor(len(finalParticipants))

For each section (slot), for each participant:
  1. Download pre-normalized clips (clips.normalized_s3_key)
       → If normalized_s3_key is NULL (late arrival): normalize inline as fallback
  2. ScaleClip(path, dims)            ← fast, scale-only pass, no re-encode
  3. ConcatClips(scaledClips)         ← concat participant's clips for this section
  4. Pad remainder with GenerateBlackPanel(dims, remainingDuration, displayName)

For each section:
  5. StackPanels(participantPanels, audioIdx)   ← vstack / 2×2 grid

Final:
  6. ConcatSections(allSections)                ← final reel
  7. Upload reel → velo-reels (S3) + CloudFront
  8. SET sessions.status = complete, reel_url = CDN URL
  9. Push notify all participants
```

**Late clip handling:** If a clip arrives after its slot's normalization window (e.g. CoreData retry 30 minutes later), `normalized_s3_key` will be NULL. Phase 2 normalizes it inline before scaling. This is the fallback path, not the happy path.

**NormalizeClip split:** The current `NormalizeClip(path, dims, timestamp)` combines VFR fix + scale. The two-phase design requires:
- `NormalizeClip(path, timestamp)` — VFR→CFR + CRF 23 at original resolution, audio preserved (Phase 1)
- `ScaleClip(path, dims)` — scale-only pass, no re-encode (Phase 2)

Implementation detail for the task; the intent is documented here.

#### Data model addition

```
clips table:
  + normalized_s3_key  TEXT  NULL   -- S3 key of the normalized clip; NULL = not yet processed
```

Phase 2 checks this column: non-null → download from normalized key. NULL → normalize inline (fallback).

#### Pipeline summary

```
Phase 1 (slot end + 10 min):
  velo-clips (S3) → NormalizeClip(720×1280) → normalized/{sessionID}/{clipID}.mp4

Phase 2 (deadline):
  normalized clips → ScaleClip(PanelDimsFor(n)) → ConcatClips → pad black
    → StackPanels(audioIdx) → ConcatSections → velo-reels (S3) + CloudFront
```

Audio rotation: `audioIdx = sectionIdx % len(participants)` — one participant's audio per section, others silenced.

---

### 8.7 — Panel dimensions and reel output

**Clip capture resolution (all cases)**

All clips are recorded at `.hd1280x720`. The sensor shoots landscape and embeds `rotate=90` metadata. FFmpeg autorotate decodes it as portrait.

- Capture preset: `.hd1280x720`
- Raw file: `1280×720` landscape
- Decoded (after autorotate): `720×1280` portrait
- Uploaded as-is — no client-side resize

**All clips are always uploaded at 720×1280.** The server decides how to scale based on participant count at composition time — the iOS client never needs to know the final layout.

| Participants | Clip uploaded | FFmpeg scales to | Layout | Final reel |
|:---:|:---:|:---:|:---:|:---:|
| 1 | 720×1280 | 720×1280 | single | 720×1280 |
| 2 | 720×1280 | 720×640 | vstack | 720×1280 |
| 3 | 720×1280 | 720×427 | vstack ×3 | 720×1281 |
| 4 | 720×1280 | 360×640 | 2×2 grid | 720×1280 |

**Visual layout per participant count:**

**1 participant** — full portrait panel, no scaling needed:
```
┌──────────┐
│          │
│  720×    │
│  1280    │
│          │
│          │
└──────────┘
```

**2 participants** — each clip scaled from 720×1280 → 720×640, vertically stacked:
```
┌──────────┐
│  720×640 │  ← participant A (top)
├──────────┤
│  720×640 │  ← participant B (bottom)
└──────────┘
```

**3 participants** — each clip scaled from 720×1280 → 720×427, vertically stacked:
```
┌──────────┐
│  720×427 │  ← participant A
├──────────┤
│  720×427 │  ← participant B
├──────────┤
│  720×427 │  ← participant C
└──────────┘
```

**4 participants** — each clip scaled from 720×1280 → 360×640, 2×2 grid:
```
┌─────┬─────┐
│360× │360× │  ← A (top-left), B (top-right)
│ 640 │ 640 │
├─────┼─────┤
│360× │360× │  ← C (bottom-left), D (bottom-right)
│ 640 │ 640 │
└─────┴─────┘
```

Codec: H.264 (libx264), CRF 23, preset fast. Audio: AAC 128 kbps stereo. Frame rate: 30fps CFR. Container: MP4.

---

### 8.8 — Implementation task map

| Task | Scope | Depends on |
|------|-------|-----------|
| CameraManager.swift | iOS: AVFoundation capture, preview layer, file output | — |
| CameraPreviewView.swift | iOS: UIViewRepresentable for preview layer | CameraManager |
| CameraView wiring | iOS: replace mockup preview, hook hold gesture | CameraManager |
| ClipUploadService.swift | iOS: presigned URL → S3 PUT → confirm | Upload URL endpoint |
| PendingClipStore.swift | iOS: CoreData retry queue | ClipUploadService |
| s3/client.go | Backend: PresignPut, HeadObject | AWS SDK v2 in go.mod |
| store/clip_store.go | Backend: InsertClip, ClipByS3Key, SlotByID, IsActiveParticipant | DB schema (exists) |
| handler/clip_handler.go | Backend: GetUploadURL + ConfirmClip | s3/client, clip_store |
| Route wiring in main.go | Backend: mount clip routes, init S3 client | clip_handler |
| DB migration | Backend: add `normalized_s3_key` column to `clips` table | DB schema (exists) |
| NormalizeClip split | Backend: split into NormalizeClip (no scale) + ScaleClip (scale only) | ffmpeg.Composer |
| Task 17: queue | Backend: Redis LIST job queue | — |
| Task 18: scheduler | Backend: deadline detection + slot-end detection, enqueue | Task 17 |
| Task 18b: SlotNormalizer | Backend: Phase 1 — normalize clips at slot end + 10 min grace | Task 18, NormalizeClip split, s3/client |
| Task 19: ReelProcessor | Backend: Phase 2 — download normalized → scale → stack → concat → upload → notify | Tasks 17–18, ffmpeg.Composer |

**Recommended build order:**
1. Backend upload-url endpoint (unblocks iOS upload testing)
2. Backend confirm endpoint + clip store
3. iOS CameraManager (can be developed in parallel with backend)
4. iOS ClipUploadService + retry queue
5. NormalizeClip split + DB migration (add `normalized_s3_key`)
6. Task 17 → 18 + 18b (queue, scheduler, slot normalizer)
7. Task 19 (reel composition — Phase 2 only, downloads pre-normalized clips)

---

### 8.9 — Deferred items

| Feature | Notes |
|---------|-------|
| Front camera toggle | Back camera only for v1 |
| Timestamp text overlay | Requires `brew install ffmpeg-full` (drawtext filter); reserved |
| Client-side bitrate compression | Revisit in v1.1 if 4G upload UX is poor |
| `auto_slot` mode | PRD defers to v1.1; `named_slots` only in MVP |
| Real AVPlayer in ReelPlayerView | Separate task; currently a simulated mockup |
