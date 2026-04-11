# Velo Server MVP — Task Breakdown

## Package Structure (target)
```
server/internal/
├── config/config.go          (exists — extend with JWT_SECRET, APPLE_BUNDLE_ID, BASE_URL)
├── middleware/
│   ├── logger.go             (exists)
│   └── auth.go               (new — JWT auth middleware)
├── domain/                   (new — pure data types, zero infra deps)
│   ├── user.go, session.go, slot.go, clip.go
│   └── errors.go             (AppError with code/message/status)
├── repository/               (new — interfaces + pgx implementations)
│   ├── user.go / user_pg.go
│   ├── session.go / session_pg.go   (includes participants, slots, slot_participations)
│   ├── clip.go / clip_pg.go
│   └── token.go / token_pg.go
├── service/                  (new — business logic, depends on repo interfaces)
│   ├── auth.go, user.go, session.go, clip.go
│   ├── reel.go               (job processor + ReelComposer interface)
│   └── scheduler.go          (30s ticker for deadlines + reminders)
├── handler/                  (new — HTTP handlers)
│   ├── response.go           (JSON helpers, error mapping)
│   ├── auth.go, user.go, session.go, clip.go, reel.go
├── auth/                     (new — Apple validation + JWT)
│   ├── apple.go              (AppleValidator interface + JWKS impl)
│   └── jwt.go                (JWTManager: create/parse access tokens)
├── storage/                  (new — S3 abstraction)
│   ├── storage.go            (ObjectStorage interface)
│   └── memory.go             (in-memory stub for dev/tests)
├── push/                     (new — APNs abstraction)
│   ├── push.go               (Pusher interface)
│   └── noop.go               (no-op for dev)
├── queue/                    (new — Redis job queue)
│   ├── queue.go              (JobQueue interface)
│   └── redis.go              (Redis LIST impl)
└── testutil/                 (new — shared test infra)
    ├── db.go                 (testcontainers-go Postgres + migrations)
    └── fixtures.go           (factory functions for test data)
```

## New Dependencies
`golang-jwt/jwt/v5`, `google/uuid`, `stretchr/testify`, `testcontainers/testcontainers-go` + postgres module, `golang-migrate/migrate/v4`, `aws/aws-sdk-go-v2` (interface only for now)

---

## Task Dependency Order
```
[1,2,3] → [4,5,6,7] → [8,9] → [10,12,13,16] → [11,14,15,17] → [18,19] → [20] → [21]
 foundation   auth+repos   auth/user    session/clip/infra  services+handlers  worker    wire   e2e
              (parallel)    svc+handler  (parallel)          (parallel)         (parallel)
```

Tasks at the same level can be parallelized. Strict sequential ordering between levels.

---

## Tasks

### Task 1: Domain Types & Errors
- [x] Status: complete — commit `c249e3e`
- Pure data structs matching migration schema
- `AppError` type with predefined errors (ErrNotFound, ErrUnauthorized, ErrSessionFull, ErrAlreadyInSession, ErrSessionNotActive, ErrForbidden, ErrInviteExpired, ErrDuplicateClip, ErrInvalidInput)
- **Tests:** Validation — display name length, section_count bounds (1-6), max_section_duration_s (10/15/20/30), deadline in past, error uniqueness
- **Files:** `internal/domain/{user,session,slot,clip,errors}.go` + `*_test.go`

### Task 2: Test Infrastructure
- [x] Status: complete — commit `066a651`
- testcontainers-go: spin up Postgres 16, apply migrations, return `*pgxpool.Pool` + cleanup
- Fixtures: `CreateUser`, `CreateSession`, `CreateSlot`, `CreateParticipant`, `CreateClip`
- **Tests:** Meta-test: spin up container, apply migration, query pg_tables, tear down
- **Files:** `internal/testutil/{db,fixtures}.go` + `db_test.go`

### Task 3: Response Helpers
- [x] Status: complete — commit `8ec4c21`
- `handler.JSON(w, status, v)`, `handler.Error(w, err)`, `handler.Decode(r, v)`, `handler.UserID(ctx)`
- **Tests:** Correct content-type, status codes, AppError mapping, unknown error → 500
- **Files:** `internal/handler/response.go` + `response_test.go`

### Task 4: JWT + Auth Middleware
- [x] Status: complete — commits `fcd5427`, `183e005`, `087b208`, `034e137`
- `JWTManager`: `CreateAccessToken(userID)`, `ParseAccessToken(token)`. HS256, **60-min TTL** (shortened from 24h; re-evaluate once refresh flow is wired end-to-end)
- JTI claim added for per-token revocation; `AccessTokenClaims` return type
- Redis token blocklist (`auth/blocklist.go`) + logout endpoint (`POST /auth/logout`)
- Auth middleware: extract Bearer, parse, check blocklist, inject user_id into context
- **Tests:** Round-trip, expired, wrong signing method, malformed. Middleware: valid/missing/invalid/expired → 200/401
- **Files:** `internal/auth/jwt.go` + `jwt_test.go`, `internal/auth/blocklist.go` + `blocklist_test.go`, `internal/middleware/auth.go` + `auth_test.go`, `internal/handler/auth.go` + `auth_test.go`

### Task 5: Apple Identity Token Validation
- [ ] Status: not started
- `AppleValidator` interface with JWKS fetch + RS256 validation
- Mock via `httptest.Server` for tests
- **Tests:** Valid token, expired, wrong audience, wrong issuer, JWKS fetch failure
- **Files:** `internal/auth/apple.go` + `apple_test.go`
- **Modify:** `internal/config/config.go` — add `AppleBundleID`

### Task 6: User Repository
- [ ] Status: not started
- Interface: `GetByID`, `GetByAppleSub`, `UpsertByAppleSub`, `Update`, `Delete`, `UpdateAPNsToken`
- **Tests (integration):** Create, get by ID, not found, get by apple_sub, upsert (new + existing), update, delete cascade
- **Files:** `internal/repository/user.go` + `user_pg.go` + `user_pg_test.go`

### Task 7: Refresh Token Repository
- [ ] Status: not started
- Interface: `Create`, `GetByHash`, `Delete`, `DeleteByUserID`. SHA-256 hash storage
- **Tests (integration):** Create, get by hash, not found, expired, delete by user, delete single
- **Files:** `internal/repository/token.go` + `token_pg.go` + `token_pg_test.go`

### Task 8: Auth Service + Handler
- [ ] Status: not started
- Service: `SignInWithApple(identityToken) → {accessToken, refreshToken, user}`, `Refresh(refreshToken) → accessToken`
- Handler: `POST /auth/apple`, `POST /auth/refresh`
- **Tests:** Service unit tests (mocked repos/Apple). Handler httptest: success/missing body/invalid token
- **Files:** `internal/service/auth.go` + `auth_test.go`, `internal/handler/auth.go` + `auth_test.go`

### Task 9: User Service + Handler
- [ ] Status: not started
- Service: `GetMe`, `UpdateMe` (name ≤ 40 chars), `DeleteMe` (delete tokens, exclude from sessions)
- Handler: `GET/PATCH/DELETE /users/me`
- **Tests:** Service unit tests. Handler httptest
- **Files:** `internal/service/user.go` + `user_test.go`, `internal/handler/user.go` + `user_test.go`

### Task 10: Session Repository
- [ ] Status: not started
- Sessions, participants, slots, slot_participations. `AddParticipant` implements advisory lock (architecture §4.3.1)
- **Tests (integration):** Create with slots, get by ID/invite token, update status, cancel, add participant (success/full/already in session/duplicate), active session for user, slots CRUD, scheduler queries
- **Files:** `internal/repository/session.go` + `session_pg.go` + `session_pg_test.go`

### Task 11: Session Service + Handler
- [ ] Status: not started
- Service: `Create`, `GetByID`, `Join`, `Cancel`, `GetInvite`, `GetSlots`, `SkipSlot`, `UnskipSlot`
- Routes: `POST /sessions`, `GET /sessions/{id}`, `DELETE /sessions/{id}`, `POST /sessions/join/{token}`, `GET /sessions/{id}/invite`, `GET /sessions/{id}/slots`, `POST .../skip`, `POST .../unskip`
- **Tests:** 17 service unit tests, 11 handler httptest cases
- **Files:** `internal/service/session.go` + `session_test.go`, `internal/handler/session.go` + `session_test.go`

### Task 12: Storage Interface (S3 Abstraction)
- [x] Status: complete — commit `a87f570`
- `Storage` interface: `Download`, `Upload`, `ReelURL`. `S3Client` implementation + `MemoryStorage` stub
- Note: interface is scoped to reel generation needs (download clips, upload reel, CDN URL). `GenerateUploadURL` / `HeadObject` for clip upload flow are still needed (Task 14)
- **Tests:** Mock S3 operations via MemoryStorage
- **Files:** `internal/storage/storage.go` + `mem.go` + `storage_test.go`

### Task 13: Clip Repository
- [ ] Status: not started
- Interface: `Create`, `GetByID`, `GetBySessionID`, `GetBySessionAndUser`, `GetTotalDurationForSlot`
- **Tests (integration):** Create, duplicate s3_key, get by session, get by session+user, total duration
- **Files:** `internal/repository/clip.go` + `clip_pg.go` + `clip_pg_test.go`

### Task 14: Clip Service + Handler
- [ ] Status: not started
- Service: `GetUploadURL`, `Confirm` (HeadObject → arrived_at, timestamp validation ≤30min, clamping, duration check)
- Handler: `POST /sessions/{id}/clips/upload-url`, `POST /sessions/{id}/clips`
- **Tests:** Upload URL (success/not active/not participant). Confirm (success/clamped/before joined_at/duplicate/exceeds duration/S3 not found)
- **Files:** `internal/service/clip.go` + `clip_test.go`, `internal/handler/clip.go` + `clip_test.go`

### Task 15: Reel Handler
- [ ] Status: not started
- `GET /sessions/{id}/reel` — returns status + reel_url. Checks participant access
- **Tests:** Complete/generating/active/failed/not participant
- **Files:** `internal/handler/reel.go` + `reel_test.go`

### Task 16: Push Notification Interface
- [ ] Status: not started
- `Pusher` interface: `Send`, `SendMulti`. `NoopPusher` logs but doesn't send
- **Tests:** NoopPusher doesn't panic, returns nil
- **Files:** `internal/push/push.go` + `noop.go` + `noop_test.go`

### Task 17: Redis Job Queue
- [x] Status: complete (built, intentionally bypassed for MVP) — commits in feat/reel-orchestration
- `JobQueue` interface: `Enqueue`, `Dequeue`. Redis LIST (`RPUSH`/`BLPOP`)
- **MVP note:** The scheduler calls `reel.Service.Generate()` inline and does not enqueue to Redis. Job durability is provided by `sessions.status` in Postgres. This queue will be wired in when the scheduler moves to the dedicated worker process.
- **Tests:** `internal/queue/redis_test.go`
- **Files:** `internal/queue/queue.go` + `redis.go` + `redis_test.go`

### Task 18: Scheduler
- [x] Status: complete (deadline detection; reminders deferred) — commits in feat/reel-orchestration
- `Scheduler.Run(ctx)` — 30s ticker, `poll()` calls `store.ClaimDueSessions()` (Postgres `FOR UPDATE SKIP LOCKED`), then calls `service.Generate()` inline per session
- `Scheduler.RunOnce(ctx)` — single-pass mode: claims all due sessions, processes them, returns. Used by `cmd/worker` for EventBridge-triggered runs.
- **Package divergence:** lives in `internal/reel/scheduler.go`, not `internal/service/scheduler.go`
- **Reminder logic (2h/30m) not yet implemented** — `reminder_2h_sent` / `reminder_30m_sent` columns exist in schema, scheduler does not yet query or set them
- **Separated from API process** — `cmd/worker/main.go` calls `RunOnce()` and exits. Triggered by EventBridge every 5 min in production.
- **Tests:** `internal/reel/scheduler_test.go`
- **Files:** `internal/reel/scheduler.go` + `scheduler_test.go`

### Task 19: Reel Job Processor
- [x] Status: complete — commits in feat/reel-orchestration
- `reel.Service.Generate(ctx, sessionID)` — fetches session data, runs `Align()`, downloads clips from S3, normalizes (VFR→CFR), composes via `ffmpeg.Engine`, uploads to S3, marks session complete
- `reel.Store` — DB queries: `ClaimDueSessions`, `FetchSessionData`, `CompleteSession`, `FailSession`
- Retry: `FailSession` increments `retry_count`, re-queues to `active` if < 3, else marks `failed`
- **Package divergence:** lives in `internal/reel/service.go` + `internal/reel/store.go`, not `internal/service/reel.go`. No `ReelComposer` interface — uses `*ffmpeg.Engine` directly.
- Push notification on complete/fail **not yet implemented** (push package not built, see Task 16)
- **Tests:** `internal/reel/service_test.go`, `internal/reel/store_test.go`, `internal/reel/align_test.go`
- **Files:** `internal/reel/service.go` + `store.go` + `align.go` + `*_test.go`

### Task 20: Wire Everything Together
- [~] Status: partially complete — commits in feat/reel-orchestration
- **Done:** Reel pipeline wired into `cmd/worker/main.go` (run-and-exit via `RunOnce()`). API process (`cmd/api/main.go`) serves HTTP only — no FFmpeg, no scheduler. `/health` and `/auth/logout` routes mounted. Auto-migration on startup for both API and worker. Docker + ECS/Fargate deployment config complete.
- **Remaining:** Auth service (Sign in with Apple, refresh), user/session/clip handlers, remaining routes.
- **Modify:** `cmd/api/main.go`, `cmd/worker/main.go`, config, `.env.example`, `docker-compose.yml`

### Task 21: End-to-End Integration Tests
- [ ] Status: not started
- Full lifecycle: 2 users → auth → create session → join → clips → skip → deadline → scheduler → reel → verify
- Plus: cancellation, account deletion, one-active-session, invite expiry, duplicate clip idempotency
- **Files:** `internal/integration/flow_test.go` + `helpers_test.go`

---

## Key Design Decisions
1. **Repository interfaces in `repository/` package** — domain stays pure, no import cycles
2. **`AddParticipant` encapsulates advisory lock** — service layer doesn't manage transactions for this
3. **Storage + Push are interfaces from day one** — `MemoryStorage` and `NoopPusher` until AWS connected
4. **Timestamp validation in service layer** — handler passes raw input, service calls HeadObject + applies clamping
5. **Scheduler is a testable struct** — `Tick()` called directly in tests, no timer dependency
6. **ReelComposer interface** — `StubComposer` for now, real FFmpeg slots in later
7. **testcontainers-go for integration tests** — fresh Postgres per test, full isolation

## Critical Reference Files
- `server/migrations/000001_init.up.sql` — schema all repos must match
- `architecture.md` — API contracts, §4.3.1 concurrency, upload flow, timestamp validation
- `veloprd.md` — product requirements and scope decisions
