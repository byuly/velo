# FFmpeg Engine Architecture

## 1. Package Structure

Two-layer design: **Engine** (orchestration) wraps **Composer** (FFmpeg primitives).

```mermaid
classDiagram
    class Engine {
        -composer *Composer
        +Compose(ctx, ComposeRequest) (string, error)
        -buildParticipantPanel(ctx, workDir, sIdx, pIdx, ParticipantPanel, PanelDims) (string, error)
    }

    class Composer {
        -ffmpegBin string
        -ffprobeBin string
        -hasDrawtext bool
        +NormalizeClip(ctx, input, output) error
        +ScaleClip(ctx, input, output, PanelDims) error
        +GenerateBlackPanel(ctx, output, PanelDims, duration, name) error
        +OverlayTitle(ctx, input, output, title, PanelDims) error
        +StackPanels(ctx, output, []PanelInput, audioIdx) error
        +ConcatSections(ctx, output, []string) error
        +Probe(ctx, path) (ProbeResult, error)
        -run(ctx, args...) error
    }

    class ComposeRequest {
        +WorkDir string
        +Sections []SectionRequest
    }

    class SectionRequest {
        +Participants []ParticipantPanel
        +AudioIdx int
    }

    class ParticipantPanel {
        +LocalPaths []string
        +Name string
        +Title string
        +Duration float64
    }

    class PanelDims {
        +Width int
        +Height int
    }

    class PanelInput {
        +Path string
    }

    class ProbeResult {
        +Duration float64
        +Width int
        +Height int
        +HasAudio bool
    }

    Engine --> Composer : uses
    Engine ..> ComposeRequest : accepts
    ComposeRequest *-- SectionRequest
    SectionRequest *-- ParticipantPanel
    Composer ..> PanelDims : uses
    Composer ..> PanelInput : uses
    Composer ..> ProbeResult : returns
```

---

## 2. Engine.Compose Pipeline

Full orchestration flow from `ComposeRequest` to final `reel.mp4`.

```mermaid
flowchart TD
    A[ComposeRequest] --> B{For each section}
    B --> C["PanelDimsFor(len(participants))"]
    C --> D{For each participant}

    D --> E{Has clips?}
    E -- No --> F["GenerateBlackPanel(full duration)"]
    E -- Yes --> G[ScaleClip × N clips]
    G --> H{Multiple clips?}
    H -- Yes --> I["ConcatSections(scaled clips)"]
    H -- No --> J[Use single scaled clip]
    I --> K{Panel shorter than section?}
    J --> K
    K -- Yes --> L["GenerateBlackPanel(remainder)"]
    L --> M["ConcatSections(panel + padding)"]
    K -- No --> N[Panel ready]
    M --> N
    F --> N

    N --> N2{Has title?}
    N2 -- Yes --> N3["OverlayTitle(centered text)"]
    N2 -- No --> O
    N3 --> O

    O[Collect all panels for section]
    O --> P["StackPanels(panels, audioIdx)"]
    P --> Q[section_N.mp4]

    Q --> R{Multiple sections?}
    R -- Yes --> S["ConcatSections(all sections)"]
    R -- No --> T[Rename to reel.mp4]
    S --> U[reel.mp4]
    T --> U
```

---

## 3. Two-Phase Pipeline

How the engine fits into the larger system. Phase 1 runs eagerly at slot-end; Phase 2 runs at deadline.

```mermaid
sequenceDiagram
    participant Scheduler
    participant S3
    participant Composer
    participant Engine

    rect rgb(230, 245, 255)
    note over Scheduler,Composer: Phase 1 — Slot-end normalization (eager) — DEFERRED, not yet implemented
    Scheduler->>S3: Download raw clip
    S3-->>Scheduler: raw_clip.mov
    Scheduler->>Composer: NormalizeClip(raw, normalized)
    note right of Composer: VFR→CFR 30fps, CRF 23<br/>Original resolution, audio preserved<br/>(currently runs inline in Phase 2 instead)
    Composer-->>Scheduler: normalized.mp4
    Scheduler->>S3: Upload normalized clip
    end

    rect rgb(255, 245, 230)
    note over Scheduler,Engine: Phase 2 — Deadline composition (fast)
    Scheduler->>S3: Download pre-normalized clips
    S3-->>Scheduler: normalized clips on local disk
    Scheduler->>Engine: Compose(ComposeRequest)
    note right of Engine: ScaleClip → Concat → Pad<br/>→ OverlayTitle → StackPanels<br/>→ ConcatSections
    Engine-->>Scheduler: reel.mp4
    Scheduler->>S3: Upload final reel
    end
```

---

## 4. Panel Layouts

`PanelDimsFor(n)` output by participant count. All reels are portrait 720×1280.

```
1 participant        2 participants       3 participants       4 participants
720×1280             720×640 each         720×427 each         360×640 each

┌──────────┐         ┌──────────┐         ┌──────────┐         ┌─────┬─────┐
│          │         │  Player  │         │ Player A │         │  A  │  B  │
│          │         │    A     │         ├──────────┤         │     │     │
│  Player  │         ├──────────┤         │ Player B │         ├─────┼─────┤
│    A     │         │  Player  │         ├──────────┤         │  C  │  D  │
│          │         │    B     │         │ Player C │         │     │     │
│          │         └──────────┘         └──────────┘         └─────┴─────┘
└──────────┘

Layout: single       Layout: vstack       Layout: vstack×3     Layout: 2×2 grid
```

---

## 5. Composer Method → FFmpeg Command Mapping

| Method | FFmpeg command | Key flags | Phase |
|---|---|---|---|
| `NormalizeClip` | `ffmpeg -i input -vf fps=30 -c:v libx264 -crf 23 -c:a aac` | VFR→CFR, preserves audio | 1 |
| `ScaleClip` | `ffmpeg -i input -vf scale=W:H -c:v libx264 -crf 23 -c:a copy` | Re-encode video, copy audio | 2 |
| `GenerateBlackPanel` | `ffmpeg -f lavfi -i color=black -f lavfi -i anullsrc -t D` | Synthetic sources | 2 |
| `OverlayTitle` | `ffmpeg -i input -vf "drawtext=text='...':x=(w-text_w)/2:y=(h-text_h)/2"` | Requires libfreetype; symlinks on fallback | 2 |
| `StackPanels` | `ffmpeg -i p0 -i p1 ... -filter_complex "vstack\|hstack"` | `buildFilterGraph(n, audioIdx)` | 2 |
| `ConcatSections` | `ffmpeg -f concat -safe 0 -i list.txt -c copy` | Lossless concat | 2 |
| `Probe` | `ffprobe -print_format json -show_streams` | Duration, dims, audio detect | both |

---

## 6. Audio Rotation

One participant's audio is kept per section; all others are silenced.

```
audioIdx = sectionIdx % len(participants)
```

For a 2-participant, 4-section reel:

| Section | audioIdx | Audio from |
|---------|----------|------------|
| 0 | 0 | Alice |
| 1 | 1 | Bob |
| 2 | 0 | Alice |
| 3 | 1 | Bob |
