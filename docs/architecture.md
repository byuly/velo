# Velo — Technical Architecture

## Context

Velo is a greenfield iOS app + Go backend where 1–4 friends create a session, record short clips throughout the day, and receive an auto-generated split-screen reel at the session deadline.

---

## 1. Key Decisions

| Topic | Decision |
|-------|----------|
| **Section-based reel model** | Replaces free-form clip alignment. MVP uses `named_slots` mode only (auto-slot deferred to v1.1). Each section has a fixed max duration. Creator picks preset time windows, each slot = one section. Within a section, each participant can record 1+ clips totaling ≤ max section duration; remainder is black + name. |
| **Audio** | One panel's audio plays at a time; rotate active audio source across sections. Advanced mixing/layering deferred to post-MVP. |
| Upload confirmation | Client uploads to S3 via presigned URL, then calls `POST /sessions/:id/clips` to confirm. No S3 event notifications. |
| **arrived_at via S3 HeadObject** | At `POST /clips` confirmation, API calls S3 HeadObject → `arrived_at` = S3 `LastModified` (actual upload time, not confirmation time). Fixes delayed-confirmation clamping bug. |
| **Upload gap recovery** | Client-side retry queue (CoreData). Persists unconfirmed clips locally; retries on next launch or connectivity change. No S3 event notification infra needed for MVP. |
| Real-time updates | REST only. Push notifications (APNs) + client polling on screen focus handle all real-time needs. No WebSocket. |
| Reel orientation | Portrait 720×1280. |
| **Zero submitters** | Push notification: "Session ended — start a new one?" No guilt, forward-looking tone. Mark session `complete` with no `reel_url`. |
| **Invite links** | Universal Links (`https://velo.app/join/{token}`). Requires domain registration + `apple-app-site-association` file. Works from iMessage, WhatsApp, Instagram DMs, all apps. Same token stays active until deadline or 4-participant cap. |
| Simultaneous clips | Play sequentially within that participant's panel in a section. Never drop content. |
| Late joiner clips | If `recorded_at` is before `joined_at`, clamp to `joined_at`. |
| Join while active | Block with a clear error. Only 1 active session per user at a time. |
| Reminder idempotency | `reminder_2h_sent` and `reminder_30m_sent` boolean flags on sessions table. |
| **Home screen intercept** | If user has an active session → app opens to `SessionView` directly (1 tap to record). If no active session → calendar (default). |
| **Reel retention** | 90-day CDN expiry. "Save to Camera Roll" button in `ReelPlayerView`. Expiry warning at 75 days: "This reel expires in 15 days. Save it to keep it forever." Expiry warning uses `completed_at + 75 days`, not `created_at`. |
| **Session cancellation** | Creator can cancel an active session before the deadline (`DELETE /sessions/:id`). Sets status to `cancelled`. Frees the creator's (and participants') single active-session slot. |

---

## 2. Technical Stack

### Backend

| Component | Technology |
|-----------|-----------|
| Router | **go-chi/chi/v5** |
| Database driver | **jackc/pgx/v5** + pgxpool |
| Migrations | **golang-migrate/migrate** |
| Redis client | **redis/go-redis/v9** |
| JWT | **golang-jwt/jwt/v5** |
| AWS SDK | **aws/aws-sdk-go-v2** |
| Push | **sideshow/apns2** |
| Video | **FFmpeg** via `os/exec` |
| Config | **caarlos0/env** |

### iOS

| Component | Technology |
|-----------|-----------|
| UI | **SwiftUI** (iOS 17+) |
| Camera | **AVFoundation** (`AVCaptureSession` + `AVCaptureMovieFileOutput`) |
| Compression | **AVAssetExportSession** (H.264, max 720p, client-side) |
| Networking | **URLSession** + async/await |
| Background upload | **Background URLSession** |
| Auth | **AuthenticationServices** (Sign in with Apple) |
| Secure storage | **Keychain Services** |
| Player | **AVPlayer** |
| Package manager | **SPM** |

### Infrastructure (MVP/Beta)

| Component | Technology |
|-----------|-----------|
| Hosting | Single EC2 t3.large running Docker Compose |
| Services | API container + Worker container + PostgreSQL 16 + Redis 7 |
| File storage | AWS S3 (raw clips + reels) + CloudFront CDN (reel delivery) |
| Domain/TLS | Route 53 + ACM certificate |
| Cost | ~$30–50/mo for 10–20 beta users |

Code is 12-factor from day 1 for easy migration to ECS + RDS + ElastiCache when scaling.

---

## 3. Project Structure

> Exact files and sub-packages will be defined during implementation of each phase. This section captures layout conventions, not a file manifest.

```
velo/
├── veloprd.md
├── architecture.md
├── server/
│   ├── cmd/api/          # API server entrypoint
│   ├── cmd/worker/       # Worker entrypoint
│   ├── internal/         # All application code (Go convention)
│   ├── migrations/       # SQL migration files (golang-migrate)
│   ├── Dockerfile
│   └── docker-compose.yml
├── ios/
│   └── Velo/
│       ├── App/          # VeloApp entry point, AppState
│       ├── Models/       # Codable data models
│       ├── Views/        # SwiftUI views (grouped by feature)
│       ├── ViewModels/   # @Observable view models
│       ├── Services/     # APIClient, UploadService, etc.
│       ├── Camera/       # AVFoundation capture wrapper
│       └── Resources/    # Assets, etc.
└── .gitignore
```

### Conventions

- **Go backend**: `handler → service → repository` layering inside `internal/`. Domain packages (auth, session, clip, reel, push, storage) emerge as needed.
- **iOS**: One ViewModel per major screen. Views grouped by feature folder. Services are singleton-style classes injected via environment.
- **Migrations**: Numbered sequentially (`000001_...up.sql` / `down.sql`). One migration per schema change.
- **File naming**: Go uses snake_case. Swift uses PascalCase matching the type name.

---

## 4. Backend Architecture

### 4.1 Authentication Flow

```
iOS                         Go API                      Apple Servers
 │                            │                              │
 ├─ Sign in with Apple ──────►│                              │
 │  (identity token)          ├─ Fetch Apple JWKS ──────────►│
 │                            │◄─ Public keys ───────────────┤
 │                            ├─ Validate identity token     │
 │                            ├─ Upsert user (apple_sub)     │
 │                            ├─ Issue access JWT (24h)      │
 │                            ├─ Issue refresh token (90d)   │
 │◄─ { access_token,          │   stored in DB               │
 │     refresh_token } ───────┤                              │
 │                            │                              │
 │  (on 401)                  │                              │
 ├─ POST /auth/refresh ──────►│                              │
 │◄─ { access_token } ────────┤                              │
```

- **Access token**: JWT, 24h expiry, contains `user_id` and `exp`
- **Refresh token**: opaque UUID, 90-day expiry, stored in `refresh_tokens` table (SHA-256 hashed)
- iOS stores both in Keychain
- `APIClient` intercepts 401 → auto-refreshes → retries original request once

### 4.2 API Endpoints

All endpoints return JSON. Authenticated endpoints require `Authorization: Bearer <jwt>`.

**Error format:**
```json
{ "error": { "code": "SESSION_FULL", "message": "This session already has 4 participants" } }
```

| Endpoint | Method | Auth | Request Body | Response |
|----------|--------|------|-------------|----------|
| `/auth/apple` | POST | No | `{ identity_token }` | `{ access_token, refresh_token, user }` |
| `/auth/refresh` | POST | No | `{ refresh_token }` | `{ access_token }` |
| `/users/me` | GET | Yes | — | `{ user }` |
| `/users/me` | PATCH | Yes | `{ display_name?, avatar_url? }` | `{ user }` |
| `/users/me` | DELETE | Yes | — | `204 No Content` |
| `/sessions` | POST | Yes | `{ name?, mode, section_count, max_section_duration_s, deadline, slots? }` | `{ session }` |
| `/sessions/:id` | GET | Yes | — | `{ session, participants, slots, my_clips }` |
| `/sessions/:id/invite` | GET | Yes | — | `{ invite_url, invite_token }` (Universal Link format) |
| `/sessions/join/:token` | POST | Yes | — | `{ session }` |
| `/sessions/:id/slots` | GET | Yes | — | `{ slots[] }` (named_slots mode) |
| `/sessions/:id/slots/:slot_id/skip` | POST | Yes | — | `{ slot_participation }` |
| `/sessions/:id/slots/:slot_id/unskip` | POST | Yes | — | `{ slot_participation }` |
| `/sessions/:id/clips/upload-url` | POST | Yes | — | `{ upload_url, s3_key }` |
| `/sessions/:id/clips` | POST | Yes | `{ s3_key, recorded_at, duration_ms, slot_id? }` | `{ clip }` |
| `/sessions/:id/reel` | GET | Yes | — | `{ reel_url, status }` |
| `/sessions/:id` | DELETE | Yes | — | `204 No Content` (creator only, while status = 'active') |

### 4.3 Database Schema

**users**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `apple_sub` | TEXT UNIQUE | Apple identity token subject |
| `display_name` | TEXT | |
| `avatar_url` | TEXT | S3 URL, nullable |
| `apns_token` | TEXT | Device push token |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Profile change tracking, cache invalidation |

**sessions**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `creator_id` | UUID FK | References users; nullable (ON DELETE SET NULL) |
| `name` | TEXT | Optional, max 40 chars |
| `mode` | ENUM | `named_slots` / `auto_slot` (enum kept for forward compat; MVP only accepts `named_slots`) |
| `section_count` | INT | 1–6 (intent) |
| `max_section_duration_s` | INT | Max duration per section in seconds (10, 15, 20, or 30) |
| `deadline` | TIMESTAMPTZ | |
| `invite_token` | TEXT UNIQUE | Valid until deadline or 4-participant cap |
| `status` | ENUM | `active` / `generating` / `complete` / `failed` / `cancelled` |
| `reel_url` | TEXT | CDN URL, nullable |
| `retry_count` | INT | Default 0 |
| `reminder_2h_sent` | BOOLEAN | Default false |
| `reminder_30m_sent` | BOOLEAN | Default false |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Status transition tracking |
| `completed_at` | TIMESTAMPTZ | Nullable; set when reel generation finishes |

**session_slots**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `session_id` | UUID FK | References sessions |
| `name` | TEXT | e.g., "Morning", "Midday", or custom |
| `starts_at` | TIME | Slot start time (no TZ — interpreted as server local time for MVP) |
| `ends_at` | TIME | Slot end time; may be less than `starts_at` for overnight slots (e.g., Night: 22:00–02:00) — slot assignment queries must handle this |
| `slot_order` | INT | Order in the reel |

**slot_participations**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `slot_id` | UUID FK | References session_slots |
| `user_id` | UUID FK | References users |
| `status` | ENUM | `recording` / `skipped` |

**session_participants**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `session_id` | UUID FK | |
| `user_id` | UUID FK | Nullable if account deleted |
| `display_name_snapshot` | TEXT | Captured at join time |
| `joined_at` | TIMESTAMPTZ | Used for late joiner marker |
| `status` | ENUM | `active` / `excluded` |

**clips**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `session_id` | UUID FK | |
| `user_id` | UUID FK | |
| `slot_id` | UUID FK | Nullable; references session_slots (populated for named_slots; nullable column kept for future auto_slot support) |
| `s3_key` | TEXT UNIQUE | Unique; used for client-side retry deduplication |
| `recorded_at` | TIMESTAMPTZ | Device capture time, used for alignment |
| `arrived_at` | TIMESTAMPTZ | S3 HeadObject `LastModified` (actual upload time) |
| `recorded_at_clamped` | BOOLEAN | True if outside ±30 min tolerance or before `joined_at` |
| `duration_ms` | INT | |
| `created_at` | TIMESTAMPTZ | |

**refresh_tokens**

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `user_id` | UUID FK | |
| `token_hash` | TEXT | SHA-256 |
| `expires_at` | TIMESTAMPTZ | 90 days from issue |
| `created_at` | TIMESTAMPTZ | |

**Indexes:**
```sql
-- Scheduler: find active sessions past their deadline
CREATE INDEX idx_sessions_deadline_status ON sessions (deadline) WHERE status = 'active';
-- Note: idx_sessions_invite_token and idx_clips_s3_key are omitted — UNIQUE column constraints already create these indexes
-- Prevent duplicate joins; NULLs from account deletion are excluded
CREATE UNIQUE INDEX idx_participants_session_user ON session_participants (session_id, user_id) WHERE user_id IS NOT NULL;
-- Join flow: check "1 active session per user" constraint
CREATE INDEX idx_participants_user_active ON session_participants (user_id) WHERE status = 'active';
-- Reel generation: fetch all clips for a session
CREATE INDEX idx_clips_session ON clips (session_id);
-- Token refresh: look up by hash
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens (token_hash);
-- Fetch slots for a session
CREATE INDEX idx_slots_session ON session_slots (session_id);
-- One skip/recording decision per user per slot
CREATE UNIQUE INDEX idx_slot_participations_slot_user ON slot_participations (slot_id, user_id);
```

### 4.3.1 Concurrency: "1 active session per user" enforcement

The `idx_participants_user_active` partial index supports fast lookups but does not prevent
race conditions on concurrent joins. The join-session handler must:

1. `BEGIN` transaction
2. `SELECT id FROM session_participants WHERE user_id = $1 AND status = 'active' FOR UPDATE`
   — locks the user's active participation row (or acquires an advisory lock if no row exists:
   `SELECT pg_advisory_xact_lock(hashtext($user_id::text))`)
3. Check result — if row exists, reject with `ALREADY_IN_SESSION`
4. `INSERT INTO session_participants ...`
5. `COMMIT`

This serializes concurrent join attempts for the same user without affecting other users.

### 4.4 Upload Flow

```
iOS                           Go API                         S3
 │                              │                              │
 ├─ POST /clips/upload-url ────►│                              │
 │                              ├─ Generate presigned PUT URL ─►│
 │◄─ { upload_url, s3_key } ───┤  (5-min expiry)              │
 │                              │                              │
 ├─ PUT upload_url ─────────────────────────────────────────────►│
 │◄─ 200 OK ─────────────────────────────────────────────────────┤
 │                              │                              │
 ├─ POST /clips ───────────────►│                              │
 │  { s3_key, recorded_at,      ├─ HeadObject(s3_key) ─────────►│
 │    duration_ms, slot_id? }   │◄─ LastModified ───────────────┤
 │                              ├─ arrived_at = LastModified    │
 │                              ├─ Validate |rec - arr| ≤ 30m  │
 │                              ├─ Clamp if outside tolerance  │
 │                              ├─ Clamp if rec < joined_at    │
 │◄─ { clip } ─────────────────┤                              │
```

**Client-side retry queue:** The iOS app persists unconfirmed clips locally (CoreData). If `POST /clips` confirmation fails (network error, app killed), the app retries on next launch or connectivity change. This eliminates the need for S3 event notification infrastructure at MVP scale.

### 4.5 Section-Based Alignment Algorithm

```
Input:
  - session:        { mode, section_count, max_section_duration_s }
  - slots[]:        { id, name, starts_at, ends_at, slot_order } (named_slots mode only)
  - participants[]: { user_id, display_name_snapshot, joined_at }
  - clips[]:        { user_id, s3_key, recorded_at, duration_ms, slot_id }
  - skip_marks[]:   { slot_id, user_id } (named_slots mode only)

1. Exclude participants with 0 clips or status = 'excluded'
2. Order remaining participants: creator first, then by joined_at

3. BUILD SECTIONS (Named Slots):
     - For each slot (in slot_order), create a section
     - Assign clips to sections by slot_id (or by recorded_at falling within slot time window)
     - Participants who marked a slot as "skip" → black panel for that section

4. FOR EACH SECTION:
   - For each participant:
     - Gather their clips for this section, ordered by recorded_at
     - Total duration = sum of clip durations
     - If total < max_section_duration_s: pad remainder with black + name
     - If no clips (or skipped): full black panel + name
   - Audio: select one panel as the active audio source (rotate per section)
   - Timestamps: small recorded_at per panel corner; large section label centered

5. Insert "joined session" interstitial panels (1500ms, full-width)
   at each non-creator participant's joined_at position

Output: ordered section list (with panels per section) → FFmpeg
```

### 4.6 FFmpeg Composition (Two-Phase Pipeline)

> Full design in `docs/ffmpeg-spike.md` Section 8.6. This section summarizes the architecture.

Processing is split into two phases to minimize deadline-to-reel latency. The expensive codec re-encode runs eagerly as slots close; the layout-dependent work runs at deadline when final participant count is known.

Panel dimensions by participant count:
- 1 participant: 720×1280 (full screen)
- 2 participants: 720×640 each (vertical stack)
- 3 participants: 720×427 each (vertical stack)
- 4 participants: 360×640 each (2×2 grid)

#### Phase 1 — Slot-end normalization (eager)

Triggered when slot `ends_at + 10 min grace` passes. For each clip in the slot:

```bash
# VFR→CFR 30fps, CRF 23 re-encode at ORIGINAL resolution (720×1280)
# No scaling — panel dims depend on final participant count (unknown until deadline)
# Audio PRESERVED — needed for audio rotation in Phase 2
ffmpeg -y -i raw_clip.mov \
  -vf "fps=30" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k -ac 2 \
  normalized.mp4
```

Normalized clips uploaded to S3 (`normalized/{sessionID}/{clipID}.mp4`). The `clips.normalized_s3_key` column tracks completion; NULL = not yet processed.

> **Note:** `drawtext` (timestamp overlay) requires `brew install ffmpeg-full` which includes libfreetype. Standard `brew install ffmpeg` does not support it. Timestamp overlays are deferred — the `timestamp`/`name` parameters in `NormalizeClip`/`GenerateBlackPanel` are reserved for when the full formula is present. See spike doc Section 6 for details.

#### Phase 2 — Deadline composition (fast)

Triggered when session deadline passes. Downloads pre-normalized clips (~1–2 Mbps each, already re-encoded).

```bash
# Scale pass — lightweight re-encode (input already CRF 23 from Phase 1)
ffmpeg -y -i normalized.mp4 \
  -vf "scale=720:640" \
  -c:v libx264 -preset fast -crf 23 \
  scaled.mp4
```

Then per section, per participant:
1. **Scale** normalized clips to `PanelDimsFor(finalParticipantCount)`
2. **Concat** participant's clips for the section
3. **Pad** remainder with black panel (silent AAC + color=black)

Per section:
4. **Stack** panels — vstack (2–3 participants) or 2×2 grid (4 participants)
5. **Audio rotation** — one participant's audio per section: `audioIdx = sectionIdx % len(participants)`

Final:
6. **Concat** all sections into final reel (`-f concat -safe 0`)
7. Upload to S3, update `sessions.reel_url`, push notify

**Late clip handling:** If `normalized_s3_key` is NULL (clip arrived after slot normalization window, e.g. CoreData retry), normalize inline during Phase 2 as a fallback.

#### NormalizeClip split

The spike's `NormalizeClip(path, dims, timestamp)` combines VFR fix + scale. The two-phase design requires:
- `NormalizeClip(path, timestamp)` — Phase 1: VFR→CFR + CRF 23 at original resolution, audio preserved
- `ScaleClip(path, dims)` — Phase 2: scale-only pass

### 4.6.1 FFmpeg Risk Mitigation

- **Filter graph construction**: `filter_complex` strings are fragile — one wrong label breaks the output silently. The `buildFilterGraph` helper in `internal/ffmpeg/composer.go` constructs filter graphs from typed inputs. Tested with 1/2/4 participant fixtures.
- **iPhone video normalization**: iPhones output variable frame rate + rotation metadata. Use `-vf fps=30` (not the deprecated `-vsync cfr`) and rely on FFmpeg's autorotate (enabled by default since FFmpeg 4+).
- **Error handling**: FFmpeg reports errors as stderr text with exit codes. Always capture and log full stderr on failure. Don't try to parse it structurally — just log it for debugging and treat non-zero exit as failure.
- **Disk I/O**: Multi-pass writes intermediate files to disk. Use a temp directory on fast storage, clean up intermediates after each successful reel, and monitor disk usage on the worker instance.
- **Keep passes small**: Resist combining passes into one giant filter_complex. The multi-pass approach is intentional — each invocation stays simple, debuggable, and independently testable.

### 4.7 Job Queue & Scheduler

The worker process runs a `time.Ticker` every 30 seconds with four queries per tick:

1. **Slot-end normalization**: slots where `ends_at + 10 min grace <= now()` with un-normalized clips → download raw clips → `NormalizeClip` at 720×1280 → upload to S3 → set `clips.normalized_s3_key`
2. **Deadline detection**: sessions where `status = 'active' AND deadline <= now()` → atomically set `status = 'generating'` → `RPUSH velo:reel_jobs {session_id}`
3. **2h reminder**: sessions where deadline is within the next 2h window and `reminder_2h_sent = false` → send push → set flag
4. **30m reminder**: same pattern

**Queue**: Redis LIST — `RPUSH` to enqueue, `BLPOP velo:reel_jobs 5` to dequeue (5s timeout for graceful shutdown). Job durability via PostgreSQL `sessions.status`, not Redis.

**Retry**: on failure, increment `retry_count` and re-enqueue with exponential delay (30s → 2min → 10min). After 3 failures: `status = 'failed'`, push notify creator.

**Concurrency:** MVP runs a single worker goroutine processing one reel at a time. On a t3.large
(2 vCPU), FFmpeg multi-pass saturates the CPU during composition. Concurrent reel generation would
degrade all reels. If multiple sessions hit deadline simultaneously, they queue and process serially.
The two-phase split reduces deadline-time processing significantly — Phase 2 (scale + stack + concat) is much faster than the full pipeline. Post-beta: ECS with dedicated worker tasks enables parallel processing.

### 4.8 Push Notifications

| Event | Recipients | Key Payload Fields |
|-------|-----------|-------------------|
| `participant_joined` | All session members | `session_id`, `display_name` |
| `reminder_2h` | Members with remaining sections to record | `session_id`, `deadline` |
| `reminder_30m` | Members with remaining sections to record | `session_id`, `deadline` |
| `reel_ready` | All session members | `session_id`, `reel_url` |
| `reel_failed` | Session creator only | `session_id` |
| `session_ended_no_clips` | All session members | `session_id` — "Session ended — start a new one?" |

---

## 5. iOS Architecture

### 5.1 Navigation

```
VeloApp (@main)
 └─ AppState (auth check + active session intercept)
     ├─ Unauthenticated:
     │   └─ WelcomeView
     │       └─ OnboardingView (first launch only)
     │
     └─ Authenticated:
         ├─ Active session exists → SessionView (1 tap to record)
         │   └─ CameraView
         │
         └─ No active session → NavigationStack
             └─ HomeView (calendar)
                 ├─ CreateSessionView → SessionView
                 ├─ SessionView (active)
                 │   └─ CameraView
                 ├─ ReelPlayerView (completed, save-to-camera-roll, expiry warning)
                 └─ SettingsView
```

- `AppState` is `@Observable`, holds auth state, navigation path, and **active session check**
- On launch: `AppState` queries for active session → if found, routes directly to `SessionView`
- `VeloApp.onOpenURL` handles Universal Links (`https://velo.app/join/{token}`) → join flow
- Push notification tap routes to `SessionView` or `ReelPlayerView` by payload type

### 5.2 Key Patterns

**ViewModels** use `@Observable` (iOS 17+):
```swift
@Observable
final class SessionViewModel {
    var session: Session?
    var clips: [Clip] = []
    var isLoading = false
    var error: AppError?
    private let api: APIClient
}
```

**APIClient** with 401 interceptor → refresh → retry once:
```swift
final class APIClient {
    func request<T: Decodable>(_ endpoint: Endpoint) async throws -> T {
        // attach Bearer token, on 401: refresh token then retry
    }
}
```

**UploadService** uses Background URLSession:
```swift
let config = URLSessionConfiguration.background(withIdentifier: "com.velo.upload")
config.isDiscretionary = false
config.sessionSendsLaunchEvents = true
```

**CameraManager**:
- `AVCaptureSession` + `AVCaptureMovieFileOutput`
- `recorded_at` captured at `fileOutput(_:didStartRecordingTo:from:)` callback
- Enforces max duration per session `max_section_duration_s`
- Returns local file URL → compression via `AVAssetExportSession` → upload

**UploadRetryQueue** (CoreData-backed):
```swift
// CoreData entity: PendingClipConfirmation
// Fields: s3Key, sessionId, recordedAt, durationMs, slotId?, createdAt
// On app launch + on connectivity change (NWPathMonitor):
//   fetch all pending → retry POST /clips for each → delete on success
```
- Ensures no clips are lost due to network failures or app termination between S3 upload and API confirmation
- Retries are idempotent (backend deduplicates by `s3_key`)

---

## 6. Infrastructure

### Docker Compose

```yaml
services:
  api:
    build: { context: ., dockerfile: Dockerfile }
    command: ["/usr/local/bin/api"]
    ports: ["8080:8080"]
    env_file: .env
    depends_on: [postgres, redis]

  worker:
    build: { context: ., dockerfile: Dockerfile }
    command: ["/usr/local/bin/worker"]
    env_file: .env
    depends_on: [postgres, redis]

  postgres:
    image: postgres:16-alpine
    volumes: [pgdata:/var/lib/postgresql/data]
    environment:
      POSTGRES_DB: velo
      POSTGRES_USER: velo
      POSTGRES_PASSWORD: ${DB_PASSWORD}

  redis:
    image: redis:7-alpine

volumes:
  pgdata:
```

### S3

- `velo-clips`: raw clip uploads, 7-day lifecycle expiry
- `velo-reels`: generated reels, 90-day lifecycle expiry, served via CloudFront
- CORS on `velo-clips` for presigned PUT from iOS

### Universal Links

- Domain: `velo.app` (registered via Route 53)
- Serve `/.well-known/apple-app-site-association` from the API or CDN:
```json
{
  "applinks": {
    "apps": [],
    "details": [{
      "appID": "TEAMID.com.velo.app",
      "paths": ["/join/*"]
    }]
  }
}
```
- iOS app handles `https://velo.app/join/{token}` via `onOpenURL` → join flow
- Works from iMessage, WhatsApp, Instagram DMs, and all apps (no custom scheme needed)

### Deployment

- EC2 t3.large (2 vCPU, 8GB — FFmpeg headroom)
- TLS via Caddy or nginx + Let's Encrypt
- SSH deploys for beta; GitHub Actions CI/CD added when team grows

---

## 7. Implementation Order

### Phase 1 — Server Foundation
1. Go module, project structure, Docker Compose
2. Config package (env-based)
3. PostgreSQL migrations (all tables + indexes)
4. Chi router + middleware (logging, CORS, auth)
5. Sign in with Apple validation + JWT issuance
6. S3 presigned URL generation
7. Health check endpoint

### Phase 1.5 — FFmpeg Spike (de-risk reel engine — see issue #5)
1. Record sample iPhone clips (2 clips, different lighting)
2. Normalize VFR → CFR, handle rotation metadata
3. Validate multi-pass pipeline: scale → overlay → vstack → concat
4. Test all layouts: 1-panel, 2-panel, 4-panel
5. Document working commands in `docs/ffmpeg-spike.md`

### Phase 2 — Core iOS
1. Xcode project, SPM setup
2. AppState + navigation skeleton
3. WelcomeView + AuthService (Sign in with Apple → backend → Keychain)
4. APIClient with auth interceptor + token refresh
5. OnboardingView (display name + avatar)
6. HomeView + CalendarGridView (empty state)
7. CreateSessionView → POST /sessions
8. Universal Link handling (`https://velo.app/join/{token}` + `apple-app-site-association`)

### Phase 3 — Recording
1. CameraManager (hold-to-record, duration enforcement)
2. CameraView (record button, timer, preview, retake)
3. AVAssetExportSession compression
4. UploadService (Background URLSession → S3)
5. SessionView (section/slot cards, upload progress, participant list)
6. Clip confirmation (POST /sessions/:id/clips)

### Phase 4 — Reel Engine
1. Section-based alignment algorithm (`service/reel.go`) — named slots path
2. FFmpeg multi-pass wrapper
3. Reel job processor
4. Redis queue + worker dequeue loop
5. Scheduler: deadline detection + enqueue
6. Scheduler: reminder push dispatch
7. APNs sender
8. Session status lifecycle (active → generating → complete/failed)

### Phase 5 — Polish & Integration
1. ReelPlayerView (AVPlayer, scrubbing, timestamp overlay)
2. Push notification routing (tap → correct screen)
3. Account deletion (exclude from sessions, data wipe)
4. Error states across all flows
5. Edge cases (0 submitters, retry exhaustion)
6. SettingsView
7. End-to-end testing

---

## 8. Verification

### Backend
- `go test ./...` unit tests per package
- Integration tests with testcontainers-go (Postgres + Redis)
- Section-based alignment fixtures: solo, 2-person, 4-person, late joiner, zero submitters, named slots
- FFmpeg output verified: resolution, panel layout, timestamp overlay
- Scheduler tested with mocked time

### iOS
- Unit tests for ViewModels with mock APIClient
- UI tests: auth → create session → record → confirm upload
- Device testing for camera + upload (simulator can't use camera)
- Deep link routing from Safari

### End-to-End
- Device A creates session → invite → Device B joins → both record → deadline passes → reel push arrives on both
- Verify panel assignment, timestamp labels, black panels
- Verify edge cases: solo reel, late joiner marker, account deletion mid-session
