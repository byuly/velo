# Velo

Velo is an iOS app where a group of friends (up to 4) records short clips throughout the day and receives an auto-generated split-screen reel when the session ends.

---

## How it works

Friends create a **session** with named time slots — Morning, Midday, Evening, etc. — and a deadline. Each person records a clip during each slot from their own phone. When the deadline hits, Velo stitches everyone's clips together into a portrait split-screen reel and sends it to the group.

```mermaid
flowchart LR
    A[Create session\nwith time slots] --> B[Share invite link]
    B --> C[Friends join\nvia Universal Link]
    C --> D[Everyone records\nduring each slot]
    D --> E[Deadline passes]
    E --> F[Reel generated\nautomatically]
    F --> G[Push notification\nreel is ready]
```

---

## Session lifecycle

```mermaid
stateDiagram-v2
    [*] --> active : Creator opens session
    active --> active : Participants join & record clips
    active --> cancelled : Creator cancels
    active --> generating : Deadline passes — worker claims session
    generating --> complete : Reel uploaded to S3 + CDN URL set
    generating --> failed : 3 consecutive FFmpeg failures
    complete --> [*]
    failed --> [*]
    cancelled --> [*]
```

---

## Architecture

```mermaid
graph TD
    subgraph iOS["iOS App (SwiftUI)"]
        CAM[Camera\nAVFoundation]
        UP[Upload Service\nBackground URLSession]
        AC[APIClient\nSign in with Apple]
    end

    subgraph Backend["Backend (Go)"]
        API[API Server\nchi router · :8080]
        WRK[Worker\nscheduler + reel pipeline]
    end

    subgraph Infra["Infrastructure"]
        PG[(PostgreSQL 16)]
        RDS[(Redis 7)]
        S3C[S3 — clips bucket\n7-day lifecycle]
        S3R[S3 — reels bucket\n90-day lifecycle]
        CF[CloudFront CDN]
        APNS[APNs]
    end

    CAM --> UP
    UP -->|presigned PUT| S3C
    AC -->|REST + JWT| API
    API --> PG
    API --> RDS
    API -->|GenerateUploadURL / HeadObject| S3C
    WRK --> PG
    WRK -->|download clips| S3C
    WRK -->|upload reel| S3R
    S3R --> CF
    WRK --> APNS
    APNS -->|push notification| iOS
```

---

## Upload flow

The iOS app never uploads through the API server. It gets a short-lived presigned URL and pushes directly to S3, then confirms with the API.

```mermaid
sequenceDiagram
    participant iOS
    participant API
    participant S3

    iOS->>API: POST /sessions/:id/clips/upload-url
    API-->>iOS: { upload_url, s3_key }

    iOS->>S3: PUT upload_url (raw clip, ~10–50 MB)
    S3-->>iOS: 200 OK

    iOS->>API: POST /sessions/:id/clips { s3_key, recorded_at, duration_ms }
    API->>S3: HeadObject(s3_key) → arrived_at
    API-->>iOS: { clip }
```

If confirmation fails (network drop, app killed), the app persists the pending clip in CoreData and retries on next launch. The backend deduplicates by `s3_key`.

---

## Reel generation

When the session deadline passes, the worker claims the session and runs a multi-pass FFmpeg pipeline.

```mermaid
flowchart TD
    A[Worker claims session\nstatus → generating] --> B[Download clips from S3]
    B --> C[Normalize each clip\nVFR → CFR 30fps · CRF 23]
    C --> D[Scale to panel size\nbased on participant count]
    D --> E{Participant count}
    E -->|1| F[720×1280 full screen]
    E -->|2| G[720×640 vertical stack]
    E -->|3| H[720×427 vertical stack]
    E -->|4| I[360×640 2×2 grid]
    F & G & H & I --> J[Concat sections\nrotate audio per section]
    J --> K[Upload reel to S3]
    K --> L[Update session status → complete\nset reel_url]
    L --> M[Push notify all participants]
```

---

## Tech stack

| Layer | Technology |
|---|---|
| iOS | SwiftUI · AVFoundation · Background URLSession · Sign in with Apple · CoreData |
| API server | Go · chi · pgx/v5 · golang-jwt |
| Worker | Go · FFmpeg (multi-pass via exec) |
| Database | PostgreSQL 16 |
| Cache / blocklist | Redis 7 |
| File storage | AWS S3 |
| Reel delivery | CloudFront CDN |
| Push notifications | APNs (sideshow/apns2) |
| Infrastructure | Docker Compose · EC2 t3.large (MVP) |

---

## Local development

### Prerequisites

- Go 1.22+
- Docker + Docker Compose
- `ffmpeg` (`brew install ffmpeg`)
- `golang-migrate` CLI (`brew install golang-migrate`)
- `golangci-lint` (for linting)

### Setup

```bash
# 1. Start Postgres (and optionally Redis)
docker compose up -d
docker compose --profile redis up -d   # add Redis for token blocklist

# 2. Copy and fill in env
cp server/.env.example server/.env
# Edit .env — set JWT_SECRET, APPLE_APP_ID, and AWS credentials

# 3. Run the API server (auto-migrates on startup)
cd server
make run
```

---

## Make commands

All commands run from `server/`.

| Command | Description |
|---|---|
| `make build` | Compile `api` and `worker` binaries |
| `make run` | Build and start the API server locally |
| `make test` | Run all tests (unit + integration) |
| `make lint` | Run golangci-lint |
| `make docker-build` | Build the Docker image |
| `make docker-up` | Start all services with Docker Compose |
| `make docker-down` | Stop and remove containers |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |

---

## Project structure

```
velo/
├── server/
│   ├── cmd/api/          # API server entrypoint
│   ├── cmd/worker/       # Reel worker entrypoint (one-shot, triggered every 5 min)
│   ├── internal/
│   │   ├── auth/         # Apple identity token validation, JWT, token blocklist
│   │   ├── domain/       # Core types: User, Session, Slot, Clip
│   │   ├── handler/      # HTTP handlers
│   │   ├── middleware/   # Auth, logger
│   │   ├── reel/         # Alignment algorithm, scheduler, reel service
│   │   ├── ffmpeg/       # FFmpeg multi-pass pipeline
│   │   ├── storage/      # S3 interface + in-memory stub
│   │   ├── queue/        # Redis job queue
│   │   └── testutil/     # Shared test helpers (testcontainers + fixtures)
│   ├── migrations/       # SQL migrations (golang-migrate)
│   ├── Dockerfile
│   └── docker-compose.yml
└── swifty/               # iOS app (SwiftUI)
```
