# Velo — Technical Architecture Plan

## Context

Velo is a greenfield iOS app + Go backend where 1–4 friends create a session, record short clips throughout the day, and receive an auto-generated split-screen reel at the session deadline. This plan defines the technical stack, architecture, project structure, and key design decisions to build the MVP described in `veloprd.md`.

---

## 1. PRD Decisions & Conflict Resolutions

### Conflicts Resolved

| # | Conflict | Decision |
|---|----------|----------|
| 1 | **S3 events vs client confirmation** — PRD §5.3 says "S3 event notifies the Go API" but §7 API contract has `POST /sessions/:id/clips` for client confirmation. These overlap. | **Use client confirmation only.** Client uploads to S3 via presigned URL, then calls `POST /sessions/:id/clips` to confirm. Backend records `arrived_at` at confirmation time. Drop S3 event notifications — they add SQS/SNS infrastructure complexity with no MVP benefit. Update PRD §5.3 accordingly. |
| 2 | **WebSocket** — PRD §5.1 says "REST + WebSocket for status updates" but no user flow specifies what requires WebSocket. | **Drop WebSocket for MVP.** All real-time needs (participant joins, reel ready, reminders) are low-frequency and well-served by APNs push notifications + client polling on screen focus. Add WebSocket in v2 if richer real-time features are needed. Update PRD §5.1 to "REST". |
| 3 | **Reel orientation** — PRD says "720p" but doesn't specify portrait vs landscape. Clips are recorded in portrait on phones. | **Portrait 720×1280.** Matches recording orientation, viewing posture, and short-form video conventions. |

### Open Questions Resolved

| # | Question | Decision |
|---|----------|----------|
| 1 | Zero submitters — should the session produce no reel or notify participants? | **Send "No clips were submitted" push notification** to all members. Mark session as `complete` with no reel_url. Trivial to implement, better UX. |
| 2 | Invite link after join — does the link remain active? | **Same link stays active** until the session deadline or max participants (4) reached. One token per session, simpler model. |
| 3 | Clips in same time-slot — both play or only the first? | **Play sequentially** within that participant's panel. Never drop user content. |

### Additional Rules Identified

| Rule | Rationale |
|------|-----------|
| If a clip's `recorded_at` is before the participant's `joined_at`, clamp it to `joined_at` | Prevents late joiners' clips from back-aligning before their join point (PRD §3.3 requirement) |
| If a user already has an active session and tries to join another, **block with clear error message** | PRD SESSION-07: only 1 active session per user at a time. PRD is silent on the join-while-active edge case. |
| Add `reminder_2h_sent` and `reminder_30m_sent` boolean flags to sessions table | Needed for idempotent reminder delivery by the scheduler |

---

## 2. Technical Stack (Finalized)

### Backend

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Router | **go-chi/chi/v5** | Idiomatic Go, stdlib `net/http` compatible, clean middleware chain. Gin's custom context doesn't compose well. |
| Database driver | **jackc/pgx/v5** + pgxpool | Fastest Go Postgres driver, pure Go, built-in connection pooling |
| Migrations | **golang-migrate/migrate** | Versioned SQL migrations, CLI + library, well-maintained |
| Redis client | **redis/go-redis/v9** | De facto standard, full Redis API coverage |
| JWT | **golang-jwt/jwt/v5** | Standard JWT library for Go |
| AWS SDK | **aws/aws-sdk-go-v2** | S3 presigned URLs, official SDK |
| Push | **sideshow/apns2** | Lightweight APNs client for Go |
| Video | **FFmpeg** via `os/exec` | Subprocess calls, no CGo dependency. Multi-pass composition for reliability. |
| Config | **caarlos0/env** or stdlib `os.Getenv` | 12-factor env-based config |

### iOS

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| UI | **SwiftUI** (iOS 17+) | @Observable macro, modern NavigationStack, cleaner MVVM |
| Camera | **AVFoundation** | `AVCaptureSession` + `AVCaptureMovieFileOutput` for hold-to-record |
| Compression | **AVAssetExportSession** | H.264 encoding, max 720p, client-side before upload |
| Networking | **URLSession** + async/await | No third-party HTTP client needed. Native, lightweight. |
| Background upload | **Background URLSession** | Reliable uploads when app is backgrounded |
| Auth | **AuthenticationServices** | Sign in with Apple framework |
| Secure storage | **Keychain Services** | JWT + refresh token storage |
| Player | **AVPlayer** | Full-screen reel playback with scrubbing |
| Package manager | **SPM** | Standard, no CocoaPods/Carthage overhead |

### Infrastructure (MVP/Beta)

| Component | Technology |
|-----------|-----------|
| Hosting | Single **EC2 t3.large** running Docker Compose |
| Services | API container + Worker container + PostgreSQL 16 + Redis 7 |
| File storage | **AWS S3** (raw clips + reels) + **CloudFront** CDN (reel delivery) |
| Domain/TLS | Route 53 + ACM certificate |
| Cost | ~$30–50/mo for 10–20 beta users |

Code is 12-factor from day 1 (env config, health checks, graceful shutdown) for easy migration to ECS + RDS + ElastiCache when scaling.

---

## 3. Project Structure

```
velo/
├── veloprd.md
├── server/
│   ├── cmd/
│   │   ├── api/
│   │   │   └── main.go              # API server entrypoint
│   │   └── worker/
│   │       └── main.go              # Worker entrypoint
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go            # Env-based configuration struct
│   │   ├── auth/
│   │   │   ├── apple.go             # Apple identity token validation (JWKS)
│   │   │   └── jwt.go               # JWT issue, validate, refresh
│   │   ├── middleware/
│   │   │   ├── auth.go              # JWT auth middleware
│   │   │   ├── logging.go           # Request logging
│   │   │   └── cors.go              # CORS headers
│   │   ├── handler/
│   │   │   ├── auth.go              # POST /auth/apple, POST /auth/refresh
│   │   │   ├── user.go              # GET/PATCH/DELETE /users/me
│   │   │   ├── session.go           # CRUD sessions, join, invite
│   │   │   └── clip.go              # Upload URL, confirm upload, get reel
│   │   ├── model/
│   │   │   ├── user.go
│   │   │   ├── session.go
│   │   │   ├── clip.go
│   │   │   └── participant.go
│   │   ├── repository/
│   │   │   ├── user.go
│   │   │   ├── session.go
│   │   │   ├── clip.go
│   │   │   └── participant.go
│   │   ├── service/
│   │   │   ├── auth.go              # Auth business logic
│   │   │   ├── session.go           # Session business logic
│   │   │   ├── clip.go              # Upload + validation logic
│   │   │   └── reel.go              # Alignment algorithm + FFmpeg orchestration
│   │   ├── worker/
│   │   │   ├── scheduler.go         # Cron: deadline detection, reminder dispatch
│   │   │   ├── queue.go             # Redis job queue (RPUSH/BLPOP)
│   │   │   └── reel_job.go          # Reel generation job processor
│   │   ├── push/
│   │   │   └── apns.go              # APNs notification sender
│   │   └── storage/
│   │       └── s3.go                # S3 presigned URLs, download, upload
│   ├── migrations/
│   │   ├── 000001_create_users.up.sql
│   │   ├── 000001_create_users.down.sql
│   │   └── ...                      # Versioned migration pairs
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile.api
│   ├── Dockerfile.worker
│   └── docker-compose.yml
├── ios/
│   └── Velo/
│       ├── Velo.xcodeproj
│       ├── App/
│       │   ├── VeloApp.swift         # @main, app lifecycle
│       │   └── AppState.swift        # Global auth state, navigation root
│       ├── Models/
│       │   ├── User.swift
│       │   ├── Session.swift
│       │   ├── Clip.swift
│       │   └── Reel.swift
│       ├── Views/
│       │   ├── Welcome/
│       │   │   └── WelcomeView.swift
│       │   ├── Onboarding/
│       │   │   └── OnboardingView.swift
│       │   ├── Home/
│       │   │   ├── HomeView.swift
│       │   │   └── CalendarGridView.swift
│       │   ├── Session/
│       │   │   ├── CreateSessionView.swift
│       │   │   ├── SessionView.swift
│       │   │   └── ClipSlotView.swift
│       │   ├── Camera/
│       │   │   └── CameraView.swift
│       │   ├── Player/
│       │   │   └── ReelPlayerView.swift
│       │   └── Settings/
│       │       └── SettingsView.swift
│       ├── ViewModels/
│       │   ├── AuthViewModel.swift
│       │   ├── HomeViewModel.swift
│       │   ├── CreateSessionViewModel.swift
│       │   ├── SessionViewModel.swift
│       │   ├── CameraViewModel.swift
│       │   └── ReelPlayerViewModel.swift
│       ├── Services/
│       │   ├── APIClient.swift       # URLSession + async/await, auth interceptor
│       │   ├── AuthService.swift     # Apple sign-in, token management
│       │   ├── UploadService.swift   # Background URLSession for S3 uploads
│       │   ├── PushService.swift     # APNs registration + handling
│       │   └── KeychainService.swift # Secure JWT/refresh token storage
│       ├── Camera/
│       │   └── CameraManager.swift   # AVCaptureSession wrapper
│       └── Resources/
│           └── Assets.xcassets
└── .gitignore
```

---

## 4. Backend Architecture

### 4.1 Authentication Flow

```
iOS                         Go API                      Apple Servers
 │                            │                              │
 ├─ Sign in with Apple ──────►│                              │
 │  (identity token)          │                              │
 │                            ├─ Fetch Apple JWKS ──────────►│
 │                            │◄─ Public keys ───────────────┤
 │                            ├─ Validate identity token     │
 │                            ├─ Extract `sub` claim         │
 │                            ├─ Upsert user (apple_sub)     │
 │                            ├─ Issue access JWT (24h)      │
 │                            ├─ Issue refresh token (90d)   │
 │◄─ { access_token,         │   (stored in DB)             │
 │     refresh_token } ──────┤                              │
 │                            │                              │
 │  (on 401)                  │                              │
 ├─ POST /auth/refresh ─────►│                              │
 │  { refresh_token }         ├─ Validate refresh token     │
 │                            ├─ Issue new access JWT       │
 │◄─ { access_token } ──────┤                              │
```

- **Access token**: JWT, 24h expiry, contains `user_id` and `exp`
- **Refresh token**: opaque UUID, 90-day expiry, stored in `refresh_tokens` table
- iOS stores both in Keychain
- `APIClient` intercepts 401 responses → auto-refreshes → retries original request
- Add `refresh_tokens` table: `id`, `user_id`, `token_hash` (SHA-256), `expires_at`, `created_at`

### 4.2 API Design

All endpoints return JSON. Authenticated endpoints require `Authorization: Bearer <jwt>`.

**Standard error format:**
```json
{
  "error": {
    "code": "SESSION_FULL",
    "message": "This session already has 4 participants"
  }
}
```

**Refined endpoint details:**

| Endpoint | Method | Auth | Request Body | Response |
|----------|--------|------|-------------|----------|
| `/auth/apple` | POST | No | `{ identity_token }` | `{ access_token, refresh_token, user }` |
| `/auth/refresh` | POST | No | `{ refresh_token }` | `{ access_token }` |
| `/users/me` | GET | Yes | — | `{ user }` |
| `/users/me` | PATCH | Yes | `{ display_name?, avatar_url? }` | `{ user }` |
| `/users/me` | DELETE | Yes | — | `204 No Content` |
| `/sessions` | POST | Yes | `{ name?, clip_count, clip_length_tier, deadline }` | `{ session }` |
| `/sessions/:id` | GET | Yes | — | `{ session, participants, my_clips }` |
| `/sessions/:id/invite` | GET | Yes | — | `{ invite_url, invite_token }` |
| `/sessions/join/:token` | POST | Yes | — | `{ session }` |
| `/sessions/:id/clips/upload-url` | POST | Yes | — | `{ upload_url, s3_key }` |
| `/sessions/:id/clips` | POST | Yes | `{ s3_key, recorded_at, duration_ms }` | `{ clip }` |
| `/sessions/:id/reel` | GET | Yes | — | `{ reel_url, status }` |

### 4.3 Database Schema Refinements

Add to the PRD schema:

**refresh_tokens** (new table)

| Field | Type | Notes |
|-------|------|-------|
| `id` | UUID | PK |
| `user_id` | UUID FK | References users |
| `token_hash` | TEXT | SHA-256 of the refresh token |
| `expires_at` | TIMESTAMPTZ | 90 days from issue |
| `created_at` | TIMESTAMPTZ | |

**sessions** (add fields)

| Field | Type | Notes |
|-------|------|-------|
| `reminder_2h_sent` | BOOLEAN | Default false |
| `reminder_30m_sent` | BOOLEAN | Default false |

**Key indexes:**
```sql
CREATE INDEX idx_sessions_deadline_status ON sessions (deadline) WHERE status = 'active';
CREATE UNIQUE INDEX idx_sessions_invite_token ON sessions (invite_token);
CREATE UNIQUE INDEX idx_participants_session_user ON session_participants (session_id, user_id);
CREATE INDEX idx_participants_user_active ON session_participants (user_id) WHERE status = 'active';
CREATE INDEX idx_clips_session ON clips (session_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens (token_hash);
```

### 4.4 Upload Flow

```
iOS                           Go API                         S3
 │                              │                              │
 ├─ POST /clips/upload-url ───►│                              │
 │                              ├─ Generate presigned PUT URL ─►│
 │                              │  (5-min expiry)              │
 │◄─ { upload_url, s3_key } ──┤                              │
 │                              │                              │
 ├─ PUT upload_url ────────────────────────────────────────────►│
 │  (raw video, Background URLSession)                         │
 │◄─ 200 OK ──────────────────────────────────────────────────┤
 │                              │                              │
 ├─ POST /clips ──────────────►│                              │
 │  { s3_key, recorded_at,     ├─ Record arrived_at = now()   │
 │    duration_ms }             ├─ Validate |rec - arr| ≤ 30m │
 │                              ├─ Clamp if outside tolerance  │
 │                              ├─ Also clamp if rec < joined_at│
 │                              ├─ Create clip record          │
 │◄─ { clip } ────────────────┤                              │
```

### 4.5 Clip Alignment Algorithm

This is the core technical challenge. The algorithm produces an ordered list of **reel segments** from raw clips and join events.

```
Input:
  - participants[]: { user_id, display_name_snapshot, joined_at }
  - clips[]: { user_id, s3_key, recorded_at, duration_ms }  (validated)

Algorithm:

1. Exclude participants with 0 clips or status='excluded'
2. Order remaining participants: creator first, then by joined_at
3. Build event timeline:
   - For each clip: { type: "clip", user_id, recorded_at, duration_ms, s3_key }
   - For each non-creator participant: { type: "join", user_id, display_name, timestamp: joined_at }
4. Sort events by timestamp (recorded_at or joined_at)
5. Convert events to reel segments:
   - "clip" event → segment {
       duration: clip.duration_ms,
       panels: [
         for each participant:
           if participant == clip.user_id → { type: "video", s3_key, timestamp_label }
           else → { type: "black", display_name }
       ]
     }
   - "join" event → segment {
       duration: 1500ms (fixed),
       type: "interstitial",
       text: "{display_name} joined the session",
       timestamp_label
     }

Output: ordered list of segments → fed to FFmpeg composition
```

**Why no merge window (MVP):** Simultaneous playback of overlapping clips requires sub-segmenting with complex FFmpeg trim/overlay logic. Since clips are typically recorded minutes or hours apart, the visual difference from sequential playback is negligible. Each clip gets its own segment — dramatically simpler FFmpeg composition. Can add merge-window logic in v2 if users request it.

### 4.6 FFmpeg Composition (Multi-Pass)

Multi-pass approach for reliability and debuggability:

**Pass 1: Pre-process clips** (parallelizable)
```bash
# Scale each clip to panel dimensions, add timestamp overlay
ffmpeg -i input.mp4 \
  -vf "scale=720:h_panel,drawtext=text='9\:42 AM':fontsize=24:x=10:y=10:fontcolor=white" \
  -c:v libx264 -preset fast -crf 23 \
  processed_clip_{id}.mp4
```

Panel heights: 1 participant → 1280, 2 → 640, 3 → 427, 4 → 640 (2×2 grid: 360×640 per panel)

**Pass 2: Generate black panels**
```bash
# Black panel with name overlay for empty slots
ffmpeg -f lavfi -i "color=black:s=720x{h_panel}:d={duration_s}" \
  -vf "drawtext=text='Alex':fontsize=36:x=(w-tw)/2:y=(h-th)/2:fontcolor=white@0.5" \
  black_{participant}_{segment}.mp4
```

**Pass 3: Stack panels per segment**
```bash
# 2 participants: vertical stack
ffmpeg -i top.mp4 -i bottom.mp4 \
  -filter_complex "[0:v][1:v]vstack=inputs=2" \
  segment_{n}.mp4

# 4 participants: 2x2 grid
ffmpeg -i tl.mp4 -i tr.mp4 -i bl.mp4 -i br.mp4 \
  -filter_complex "[0:v][1:v]hstack=inputs=2[top];[2:v][3:v]hstack=inputs=2[bot];[top][bot]vstack=inputs=2" \
  segment_{n}.mp4
```

**Pass 4: Concatenate all segments**
```bash
# concat demuxer
ffmpeg -f concat -safe 0 -i segments.txt -c copy output_reel.mp4
```

**Pass 5: Upload final reel to S3** → update session with reel_url → push notify

### 4.7 Job Queue & Scheduler

**Scheduler** (runs in worker process):
- Go `time.Ticker` every 30 seconds
- Three queries per tick:
  1. `SELECT id FROM sessions WHERE status='active' AND deadline <= now()` → atomically `UPDATE status='generating'` → enqueue reel job to Redis
  2. `SELECT id FROM sessions WHERE status='active' AND deadline BETWEEN now() + interval '1h 59m 30s' AND now() + interval '2h 0m 30s' AND NOT reminder_2h_sent` → send reminder push → mark sent
  3. Same pattern for 30-minute reminder

**Redis job queue:**
- Simple LIST-based reliable queue
- `RPUSH velo:reel_jobs {session_id}` to enqueue
- `BLPOP velo:reel_jobs 5` to dequeue (5s timeout for graceful shutdown)
- Job state tracked in PostgreSQL (`sessions.status`), not Redis — durable

**Retry strategy:**
- On failure: increment `retry_count` on session, re-enqueue if `retry_count < 3`
- Exponential backoff: 30s, 2min, 10min (via scheduled re-enqueue)
- After 3 failures: set `status='failed'`, push notify session creator

### 4.8 Push Notifications

| Event | Recipients | Payload |
|-------|-----------|---------|
| `participant_joined` | All session members | `{ type, session_id, display_name }` |
| `reminder_2h` | Members with remaining clip slots | `{ type, session_id, deadline }` |
| `reminder_30m` | Members with remaining clip slots | `{ type, session_id, deadline }` |
| `reel_ready` | All session members | `{ type, session_id, reel_url }` |
| `reel_failed` | Session creator only | `{ type, session_id }` |
| `no_clips_submitted` | All session members | `{ type, session_id }` |

---

## 5. iOS Architecture

### 5.1 Navigation

```
VeloApp (@main)
 └─ AppState (auth check)
     ├─ Unauthenticated:
     │   └─ WelcomeView → Sign in with Apple
     │       └─ OnboardingView (first launch only)
     │
     └─ Authenticated:
         └─ NavigationStack
             └─ HomeView (calendar)
                 ├─ CreateSessionView → SessionView
                 ├─ SessionView (active session)
                 │   └─ CameraView (hold-to-record)
                 ├─ ReelPlayerView (completed session)
                 └─ SettingsView
```

- `AppState` is an `@Observable` class holding auth state and the navigation path
- Deep link handling: `VeloApp.onOpenURL` parses `velo://join/{token}` → navigates to join flow
- Push notification tap: routes to `SessionView` or `ReelPlayerView` based on payload type

### 5.2 Key Patterns

**@Observable ViewModels (iOS 17+):**
```swift
@Observable
final class SessionViewModel {
    var session: Session?
    var clips: [Clip] = []
    var isLoading = false
    var error: AppError?

    private let api: APIClient

    func loadSession(id: UUID) async { ... }
    func confirmClipUpload(s3Key: String, recordedAt: Date, durationMs: Int) async { ... }
}
```

**APIClient with auth interceptor:**
```swift
final class APIClient {
    func request<T: Decodable>(_ endpoint: Endpoint) async throws -> T {
        var request = endpoint.urlRequest
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        let (data, response) = try await session.data(for: request)
        if (response as? HTTPURLResponse)?.statusCode == 401 {
            try await refreshToken()
            return try await self.request(endpoint) // retry once
        }
        return try decoder.decode(T.self, from: data)
    }
}
```

**Background upload:**
```swift
final class UploadService {
    private lazy var backgroundSession: URLSession = {
        let config = URLSessionConfiguration.background(withIdentifier: "com.velo.upload")
        config.isDiscretionary = false
        config.sessionSendsLaunchEvents = true
        return URLSession(configuration: config, delegate: self, delegateQueue: nil)
    }()

    func uploadClip(to presignedURL: URL, fileURL: URL) -> URLSessionUploadTask { ... }
}
```

**CameraManager:**
- Wraps `AVCaptureSession` with `AVCaptureMovieFileOutput`
- `recorded_at` captured at `fileOutput(_:didStartRecordingTo:from:)` callback
- Enforces min/max duration based on session's clip_length_tier
- Returns local file URL for compression → upload

---

## 6. Infrastructure

### Docker Compose (MVP)

```yaml
services:
  api:
    build: { dockerfile: Dockerfile.api }
    ports: ["8080:8080"]
    env_file: .env
    depends_on: [postgres, redis]

  worker:
    build: { dockerfile: Dockerfile.worker }
    env_file: .env
    depends_on: [postgres, redis]
    # FFmpeg installed in worker image

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

### S3 Bucket Config
- Bucket: `velo-clips` (raw clips) with lifecycle: delete objects after 7 days
- Bucket: `velo-reels` (generated reels) with lifecycle: delete after 90 days
- CloudFront distribution pointed at `velo-reels` bucket
- CORS on `velo-clips` for presigned PUT uploads from iOS

### Deployment (Beta)
- EC2 t3.large (2 vCPU, 8GB RAM — FFmpeg needs headroom)
- Docker Compose with `docker compose up -d`
- TLS via Caddy reverse proxy or nginx + Let's Encrypt
- SSH-based deploys for beta. CI/CD (GitHub Actions) added when team grows.

---

## 7. Implementation Order

Follows PRD milestones with refined task breakdown:

### Phase 1 — Foundation (Server)
1. Initialize Go module, project structure, Docker Compose
2. Config package (env-based)
3. PostgreSQL migrations (all tables + indexes)
4. Chi router setup with middleware (logging, CORS, auth)
5. Sign in with Apple token validation + JWT issuance
6. S3 presigned URL generation
7. Basic health check endpoint

### Phase 2 — Core iOS
1. Xcode project setup, SPM config
2. AppState + navigation skeleton
3. WelcomeView + AuthService (Sign in with Apple → backend → Keychain)
4. APIClient with auth interceptor + token refresh
5. OnboardingView (display name + avatar)
6. HomeView + CalendarGridView (empty state)
7. CreateSessionView → POST /sessions
8. Deep link handling (velo://join/{token})

### Phase 3 — Recording
1. CameraManager (AVCaptureSession, hold-to-record, duration enforcement)
2. CameraView (UI: record button, timer, preview, retake)
3. AVAssetExportSession compression pipeline
4. UploadService (Background URLSession → S3 presigned URL)
5. SessionView (clip slots, upload progress, participant list)
6. Clip confirmation flow (POST /sessions/:id/clips)

### Phase 4 — Reel Engine
1. Clip alignment algorithm (`service/reel.go`)
2. FFmpeg wrapper: pre-process, stack, concat
3. Reel generation job processor
4. Redis queue + worker dequeue loop
5. Scheduler: deadline detection + job enqueue
6. Scheduler: reminder push dispatch
7. APNs push notification sender
8. Session status lifecycle transitions (active → generating → complete/failed)

### Phase 5 — Polish & Integration
1. ReelPlayerView (AVPlayer, scrubbing, timestamp overlay)
2. Push notification routing (tap → correct screen)
3. Account deletion flow (exclude from sessions, data wipe)
4. Error states across all flows
5. Edge cases (0 submitters → notify, deadline during processing, retry exhaustion)
6. SettingsView (display name, avatar, notifications toggle, delete account)
7. End-to-end testing

---

## 8. Verification

### Backend
- `go test ./...` for unit tests on each package
- Integration tests with Dockerized Postgres + Redis (testcontainers-go)
- Test the alignment algorithm with fixtures: solo, 2-person, 4-person, late joiner, zero submitters
- Test FFmpeg composition with sample clips → verify output resolution, panel layout, timestamp overlay
- Test scheduler with mocked time: deadline detection, reminder dispatch, retry logic

### iOS
- Unit tests for ViewModels (mock APIClient)
- UI tests for critical flows: auth → create session → record → confirm upload
- Manual testing on device for camera + upload (simulator can't use camera)
- Test deep link routing (velo://join/{token}) from Safari

### End-to-End
- Create session on device A → invite link → join on device B → both record clips → wait for deadline → verify reel arrives via push on both devices
- Verify reel layout: correct panel assignment, timestamp labels, black panels for missing slots
- Verify edge cases: solo session reel, late joiner marker, account deletion mid-session
