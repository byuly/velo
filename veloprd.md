# Velo — Product Requirements Document
**MVP v1.0 · iOS · SwiftUI + Go**

---

## Table of Contents
1. [Overview](#1-overview)
2. [Product Scope](#2-product-scope)
3. [User Flows](#3-user-flows)
4. [Feature Requirements](#4-feature-requirements)
5. [Technical Architecture](#5-technical-architecture)
6. [iOS App Architecture](#6-ios-app-architecture)
7. [API Contract](#7-api-contract)
8. [Non-Functional Requirements](#8-non-functional-requirements)
9. [Milestones](#9-milestones)
10. [Open Questions](#10-open-questions)

---

## 1. Overview

Velo is a private iOS app where users create a session, invite up to 3 friends, and record short clips throughout the day. At the session deadline, the app automatically generates a split-screen reel showing everyone's day side-by-side — aligned by timestamp. There are no persistent groups, no editing, and no public audience. Just effortless, intimate day-sharing. Solo sessions are equally welcome — a single-person vlog is a valid and encouraged use case.

> **Core Magic:** Each reel panel is timestamp-aligned. If someone is still asleep when a friend is at the gym, their panel is black. When they wake up and record a clip, it appears alongside whatever their friends are doing at that moment. The app does all the alignment — users just record and watch.

### 1.1 Problem Statement
Existing social platforms require intentional content creation, editing, and public performance. Close friends want to share their day naturally — not produce it. There is no lightweight tool that automatically weaves multiple people's day-clips into a coherent shared story.

### 1.2 Key Terms

| Term | Definition |
|------|-----------|
| **Session** | A time-bounded event where 1–4 participants record clips throughout the day. Has a deadline, mode, and settings. Produces one reel. |
| **Section** | A segment of the final reel. Each section has a fixed max duration (set at session creation). All participants' clips for that section play side-by-side in the reel. |
| **Slot** | A named time window (e.g., "Morning 6AM–10AM") during which participants are expected to record. Used in Named Slots mode. Each slot maps to one section in the reel. |
| **Clip** | A single short video recorded by a participant. One or more clips fit within a section, totaling ≤ the max section duration. |
| **Reel** | The auto-generated split-screen video combining all participants' clips, organized by sections. Delivered at the session deadline. |
| **Panel** | One participant's visual area within a reel section. Shows their clip(s) or black + name if they have no content for that section. |
| **Bucket** | (Auto-slot mode only) A group of clips clustered by timestamp proximity. Each bucket becomes one section in the reel. |
| **Intent** | The expected number of sections/clips per day, set by the session creator. In auto-slot mode, this guides expectations but isn't strictly enforced. |
| **Skip** | A participant's explicit opt-out from a specific slot (Named Slots mode). Their panel shows black + name for that section. |

### 1.3 Target Users
- Friends (2–4 people) with an existing close relationship, or solo users who want a personal day log
- Ages 18–30 comfortable with short-form video
- Users who want connection without the pressure of social media performance

### 1.4 Success Metrics (MVP)

| Metric | Target |
|---|---|
| Session completion rate | ≥ 60% of invited members submit at least 1 clip |
| Reel generation success | ≥ 95% of sessions produce a valid reel by deadline |
| D7 retention | ≥ 40% of users participate in a second session within 7 days |
| Upload success rate | ≥ 99% of submitted clips successfully upload |

---

## 2. Product Scope

### 2.1 In Scope — MVP
- User authentication (Sign in with Apple)
- Session creation — the session is the invite unit, no persistent groups
- Session invite link for up to 3 additional participants (4 total including creator)
- Session modes: **Named Slots** (preset time windows) or **Auto-Slot** (free recording, clustered at deadline)
- Session settings: section count (1–6), max section duration (e.g., 10s, 15s, 20s, 30s), deadline
- Solo sessions — 1-person vlogs produce a single-panel reel
- In-app short-form video recording
- Clip upload to backend with progress feedback
- Automated split-screen reel generation organized by sections (top/bottom layout)
- Section-aligned panels; black fill + participant name for sections where a member has no clip
- Late joiner handling — a "joined session" marker panel appears at join time; their clips do not back-align with clips recorded before they joined
- If a member submits no clips, they are excluded from the reel entirely
- If a member deletes their account during a session, they are excluded from the reel
- Reel delivery to all session members via push notification
- In-app reel playback with save-to-camera-roll option
- Calendar view — sessions stored on the day they were created
- Reel expiry warning at 75 days ("This reel expires in 15 days. Save it to keep it forever.")
- Invite links via Universal Links (`https://velo.app/join/{token}`)
- Active session intercept — app opens directly to SessionView if user has an active session

### 2.2 Out of Scope — MVP
- Persistent groups (no group object — sessions are self-contained)
- Emoji reactions or any social engagement features
- Android support
- Public sharing or exporting reels
- More than 4 participants per session
- More than 1 active session per user at a time
- Video editing tools (save-to-camera-roll is export, not editing)
- Direct messaging
- Push notification customization

---

## 3. User Flows

### 3.1 Onboarding
1. User downloads app, lands on welcome screen
2. Signs in with Apple (no username/password)
3. Prompted to set a display name and optional avatar
4. Lands on Home screen (calendar view, empty state with CTA to create a session)

### 3.2 Session Creation
1. Creator taps **New Session**
2. Configures session settings:
   - **Session name:** optional, max 40 chars (defaults to "Session — [date]")
   - **Session mode:** Named Slots or Auto-Slot
   - **Section count:** 1–6 sections expected per day (intent)
   - **Max section duration:** fixed length per section in seconds (10s, 15s, 20s, or 30s)
   - **Deadline:** date + time picker (must be at least 1 hour from now)
3. If **Named Slots** mode:
   - Creator picks from preset time windows: Morning (6–10), Midday (10–14), Afternoon (14–18), Evening (18–22), Night (22–2), or defines custom slots
   - Each slot maps to one section in the final reel
4. If **Auto-Slot** mode:
   - Creator sets section count intent (e.g., "4 clips today")
   - Participants record freely throughout the day
   - At deadline, backend clusters all clips by `recorded_at` timestamps to form buckets; each bucket = one section
5. Creator optionally invites up to 3 friends via a shareable Universal Link (`https://velo.app/join/{token}`)
6. Session goes live immediately — creator can start recording right away
7. Session card appears on the calendar on today's date

### 3.3 Joining a Session
1. Invited user taps the Universal Link (valid until session deadline)
2. If not signed in, they are prompted to sign in with Apple first
3. They are added to the session and can see current participants
4. A **"[Name] joined the session"** marker is inserted into the reel timeline at the moment they join — their clips will only be aligned from this point forward
5. If **Named Slots** mode: participant can mark any slot as "skip" (their panel shows black + name for skipped sections)
6. They can begin recording clips immediately

### 3.4 Clip Recording
1. User opens an active session
2. **Named Slots mode:** sees slot cards (e.g., Morning, Midday, etc.) with status (recorded / pending / skipped). **Auto-Slot mode:** sees section count intent and a record button.
3. Taps record to open the camera
4. Hold-to-record; each clip must be ≤ max section duration. Within a section, a participant can record 1 or more clips totaling ≤ the max section duration.
5. Preview clip — option to retake or confirm
6. Clip uploads in the background with a progress indicator; `recorded_at` (device time) is captured at the moment of recording
7. User can record and submit clips in any order throughout the day
8. If total content for a section < max section duration, the remainder is filled with black + participant name (e.g., 10s section: Tom records 10s full, Max records 3s + 7s black)
9. App sends reminder push notifications at 2 hours and 30 minutes before the deadline if sections remain
10. **Upload gap recovery:** app persists unconfirmed clips locally (CoreData). On next launch or connectivity change, retries `POST /clips` confirmation automatically.

### 3.5 Reel Generation & Delivery
1. At the session deadline, the backend triggers the reel generation job
2. Members who submitted no clips are excluded entirely
3. Members who deleted their account before the deadline are excluded entirely
4. The backend validates each clip's `recorded_at` against its `arrived_at` (S3 `LastModified` via HeadObject) — clips outside a ± 30 minute tolerance are flagged and clamped
5. **Section assembly:**
   - **Named Slots mode:** each slot = one section; clips are assigned to the slot whose time window contains their `recorded_at`
   - **Auto-Slot mode:** all clips across all participants are clustered by temporal proximity into buckets; each bucket = one section
6. Within each section, participants' clips play side-by-side in their panels
7. When a member has no clip for a section (or marked "skip"), their panel is black with their name displayed
8. **Audio:** only one panel's audio plays at any given moment; rotate which panel is the active audio source across sections
9. **Timestamp display:** small individual `recorded_at` per panel in corner; large section label or averaged time centered on the segment
10. "Joined session" markers are rendered as a brief full-width panel at the appropriate point in the timeline
11. Split-screen reel is composed (top/bottom panels)
12. Final reel is uploaded to the CDN and all session members are notified via push
13. If zero participants submitted clips: push notification "Session ended — start a new one?" (no guilt, forward-looking tone)
14. Reel appears in the session card on the calendar

### 3.6 Calendar & Reel Viewing
1. Home screen shows a monthly calendar (but see §6.1 — active session intercept may bypass this)
2. Dates with sessions show a dot indicator
3. Tapping a date reveals session cards for that day
4. Tapping a session card opens the reel player if ready, or shows a **Generating…** state
5. Reel plays full-screen with scrubbing controls and timestamp overlay
6. **Save to Camera Roll** button available in `ReelPlayerView`
7. At 75 days post-creation, a banner warns: "This reel expires in 15 days. Save it to keep it forever."

---

## 4. Feature Requirements

### 4.1 Authentication

| ID | Requirement | Priority |
|---|---|---|
| AUTH-01 | Sign in with Apple — primary auth method | P0 |
| AUTH-02 | JWT-based session tokens issued by Go backend | P0 |
| AUTH-03 | Silent token refresh on expiry — no re-login required | P0 |
| AUTH-04 | Account deletion with full data wipe; user excluded from any in-progress sessions | P1 |

### 4.2 Sessions

| ID | Requirement | Priority |
|---|---|---|
| SESSION-01 | Creator sets optional session name (max 40 chars) | P0 |
| SESSION-02 | Creator sets session mode: `named_slots` or `auto_slot` | P0 |
| SESSION-03 | Creator sets section count (1–6) and max section duration (10s / 15s / 20s / 30s) | P0 |
| SESSION-04 | Creator sets a deadline (min 1 hour from now) | P0 |
| SESSION-05 | Session generates a unique invite deep link (valid until deadline) | P0 |
| SESSION-06 | Max 4 participants per session (creator + 3 invitees) | P0 |
| SESSION-07 | Only 1 active session per user at a time | P0 |
| SESSION-08 | Session status lifecycle: `active → generating → complete / failed` | P0 |
| SESSION-09 | Session card stored on the calendar on the creation date | P0 |
| SESSION-10 | All session members notified when a new participant joins | P1 |
| SESSION-11 | Reminder push 2 hours before deadline | P1 |
| SESSION-12 | Reminder push 30 minutes before deadline | P1 |

### 4.3 Video Recording & Upload

| ID | Requirement | Priority |
|---|---|---|
| CLIP-01 | Hold-to-record with visual countdown timer | P0 |
| CLIP-02 | Enforce max clip length ≤ max section duration; total clips per section ≤ max section duration | P0 |
| CLIP-03 | Preview + retake before confirming upload | P0 |
| CLIP-04 | Background upload with progress tracking | P0 |
| CLIP-05 | `recorded_at` captured at moment of recording (device time, at `captureOutput` callback) | P0 |
| CLIP-06 | Section/slot UI showing filled/pending/skipped status | P0 |
| CLIP-07 | Video compressed client-side before upload (H.264, max 720p) | P0 |
| CLIP-08 | Auto re-upload on failure with user notification | P1 |

### 4.4 Reel Generation

| ID | Requirement | Priority |
|---|---|---|
| REEL-01 | Triggered automatically at session deadline | P0 |
| REEL-02 | Members with zero submitted clips are excluded from the reel | P0 |
| REEL-03 | Members who deleted their account before deadline are excluded | P0 |
| REEL-04 | `recorded_at` validated against `arrived_at` (S3 HeadObject `LastModified`); clamped if outside ± 30 min tolerance | P0 |
| REEL-05 | Clips organized into sections: by slot assignment (named_slots) or by temporal clustering into buckets (auto_slot) | P0 |
| REEL-06 | Layout scales by participant count: 1 = full-screen; 2 = 50/50; 3 = 33/33/33; 4 = two rows | P0 |
| REEL-07 | Black panel + member name when no clip exists for a section (or slot marked "skip") | P0 |
| REEL-08 | "Joined session" marker rendered as a brief full-width interstitial panel at join time | P0 |
| REEL-09 | Timestamp display: small individual `recorded_at` per panel in corner; large section label centered on segment | P0 |
| REEL-09a | Audio: one panel's audio plays at a time; rotate active audio source across sections | P0 |
| REEL-10 | Single-submitter sessions produce a single-panel reel (solo vlog) | P0 |
| REEL-11 | Final reel delivered to all session members via push notification | P0 |
| REEL-12 | Generation completes within 5 minutes of deadline | P1 |
| REEL-13 | Reel stored on CDN; accessible for 90 days | P1 |

### 4.5 Calendar View

| ID | Requirement | Priority |
|---|---|---|
| CAL-01 | Monthly calendar grid as the home screen | P0 |
| CAL-02 | Dot indicator on dates with sessions | P0 |
| CAL-03 | Tap a date to see session cards for that day | P0 |
| CAL-04 | Session card shows: session name, status, participant avatars | P0 |
| CAL-05 | Tap a completed session to open the reel player | P0 |
| CAL-06 | Tap an active session to open the clip recording view | P0 |
| CAL-07 | Save to Camera Roll button in `ReelPlayerView` | P0 |
| CAL-08 | Expiry warning banner at 75 days: "This reel expires in 15 days. Save it to keep it forever." | P1 |

---

## 5. Technical Architecture

### 5.1 Stack

| Layer | Technology |
|---|---|
| iOS Client | SwiftUI + AVFoundation |
| Backend API | Go (Chi) — REST |
| Database | PostgreSQL — users, sessions, participants, clips metadata |
| File Storage | AWS S3 (raw clips) + CloudFront CDN (final reels) |
| Job Queue | Redis + Go worker pool |
| Video Processing | FFmpeg via Go subprocess |
| Push Notifications | APNs via Go (`apple/apns2`) |
| Auth | Sign in with Apple + JWT |
| Hosting | MVP: Single EC2 t3.large running Docker Compose (API + Worker + PostgreSQL + Redis). Post-MVP: ECS + RDS + ElastiCache |

### 5.2 Data Flow

```
iOS App  ──[HTTPS]──────────────►  Go API  ──►  PostgreSQL (metadata)
iOS App  ──[Presigned URL]──────►  S3 (raw clip upload)
Go API   ──►  Redis Queue  ──►  Go Worker  ──►  FFmpeg  ──►  S3 / CloudFront
Go Worker  ──►  APNs  ──►  iOS App (reel ready notification)
```

### 5.3 Core Backend Services

#### Upload Service
- Client requests a presigned S3 URL from the API
- Client uploads directly to S3 — no video data passes through the Go API server
- On upload completion, client calls `POST /clips` to confirm
- At confirmation, API calls S3 HeadObject to get the object's `LastModified` → stored as `arrived_at` (actual upload time, not confirmation time)
- Go API records clip metadata: `user_id`, `session_id`, `recorded_at`, `arrived_at`, `s3_key`, `duration_ms`, `slot_id` (if named_slots mode)
- **Client-side retry queue:** app persists unconfirmed clips locally (CoreData). On next launch or connectivity change, retries `POST /clips` confirmation. No S3 event notification infra needed for MVP.

#### Timestamp Validation
- `recorded_at` is sent by the client at the moment of recording (captured at `captureOutput` callback)
- On clip confirmation, the API checks: `|recorded_at − arrived_at| ≤ 30 minutes` (where `arrived_at` = S3 HeadObject `LastModified`)
- If within tolerance, `recorded_at` is trusted and used for alignment
- If outside tolerance, `recorded_at` is clamped to `arrived_at`, flagged via `recorded_at_clamped = true`, and the clip is still included in the reel

#### Section-Based Alignment & Reel Composition Engine
This is the core technical challenge. The algorithm:

1. At deadline, fetch all clips and join events grouped by participant
2. Exclude any participant with zero submitted clips or a deleted account
3. Order remaining participants: creator first, then by `joined_at`
4. **Build sections:**
   - **Named Slots mode:** each slot (in `slot_order`) = one section. Clips are assigned to the slot whose time window contains their `recorded_at`. Participants who marked a slot as "skip" get a black panel.
   - **Auto-Slot mode:** cluster all clips across all participants by `recorded_at` temporal proximity into buckets. Each bucket = one section. Order sections chronologically.
5. For each section:
   - Each participant's clips for that section play sequentially in their panel, totaling ≤ max section duration
   - If total content < max section duration, remainder is black + participant name
   - If a participant has no clips for the section, their entire panel is black + name
6. Insert "joined session" marker events at each non-creator participant's `joined_at` timestamp as brief full-width interstitial panels
7. **Audio:** one panel's audio plays at a time; rotate active audio source across sections
8. **Timestamps:** small individual `recorded_at` per panel in corner; large section label or averaged time centered on the segment
9. Stack panels vertically in consistent participant order
10. Compose with FFmpeg using `filter_complex` (vstack + drawtext + concat)
11. Output: H.264 MP4, 720p, optimized for mobile playback

> **FFmpeg note:** All processing runs on a dedicated worker instance — not Lambda. Clip file sizes and processing time exceed Lambda constraints.

#### Session Scheduler
- A Go cron scheduler polls every 30 seconds for sessions past their deadline
- Qualifying sessions are enqueued as reel generation jobs in Redis
- Workers pick up jobs and execute the composition engine
- Job status is tracked: `queued → processing → complete → failed`
- On failure: retry up to 3 times with exponential backoff, then mark failed and notify the session creator

### 5.4 Data Models

#### Users
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `apple_sub` | TEXT UNIQUE | Apple identity token subject |
| `display_name` | TEXT | User-set name |
| `avatar_url` | TEXT | S3 URL, nullable |
| `apns_token` | TEXT | Device push token |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Profile change tracking, cache invalidation |

#### Sessions
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `creator_id` | UUID FK | References users; nullable (ON DELETE SET NULL — session continues if creator deletes account) |
| `name` | TEXT | Optional, max 40 chars |
| `mode` | ENUM | `named_slots` / `auto_slot` |
| `section_count` | INT | 1–6 (intent) |
| `max_section_duration_s` | INT | Max duration per section in seconds (10, 15, 20, or 30) |
| `deadline` | TIMESTAMPTZ | Creator-set cutoff |
| `invite_token` | TEXT UNIQUE | Deep link token, valid until deadline |
| `status` | ENUM | `active` / `generating` / `complete` / `failed` |
| `reel_url` | TEXT | CDN URL, populated on completion |
| `retry_count` | INT | Default 0; tracks reel generation retry attempts |
| `reminder_2h_sent` | BOOLEAN | Default false |
| `reminder_30m_sent` | BOOLEAN | Default false |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | Status transition tracking |
| `completed_at` | TIMESTAMPTZ | Nullable; set when reel generation finishes. Used for expiry calculation and latency monitoring |

#### Session Slots
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `session_id` | UUID FK | References sessions |
| `name` | TEXT | e.g., "Morning", "Midday", or custom |
| `starts_at` | TIME | Slot start time |
| `ends_at` | TIME | Slot end time |
| `slot_order` | INT | Order in the reel |

#### Slot Participations
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `slot_id` | UUID FK | References session_slots |
| `user_id` | UUID FK | References users |
| `status` | ENUM | `recording` / `skipped` |

#### Session Participants
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `session_id` | UUID FK | References sessions |
| `user_id` | UUID FK | References users; nullable if account deleted |
| `display_name_snapshot` | TEXT | Name at join time — preserved if account is later deleted |
| `joined_at` | TIMESTAMPTZ | Used for late joiner timeline marker |
| `status` | ENUM | `active` / `excluded` (set on account deletion mid-session) |

#### Clips
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `session_id` | UUID FK | References sessions |
| `user_id` | UUID FK | References users |
| `slot_id` | UUID FK | Nullable; references session_slots (populated for named_slots mode, null for auto_slot) |
| `s3_key` | TEXT UNIQUE | Raw clip location in S3; unique for client-side retry deduplication |
| `recorded_at` | TIMESTAMPTZ | Device capture time — used for alignment |
| `arrived_at` | TIMESTAMPTZ | S3 HeadObject `LastModified` — actual upload time |
| `recorded_at_clamped` | BOOLEAN | True if `recorded_at` was outside ± 30 min tolerance |
| `duration_ms` | INT | Clip duration in milliseconds |
| `created_at` | TIMESTAMPTZ | |

#### Refresh Tokens
| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `user_id` | UUID FK | References users |
| `token_hash` | TEXT | SHA-256 hash of opaque refresh token |
| `expires_at` | TIMESTAMPTZ | 90 days from issue |
| `created_at` | TIMESTAMPTZ | |

---

## 6. iOS App Architecture

### 6.1 Screen Map

| Screen | Purpose |
|---|---|
| `WelcomeView` | App launch, Sign in with Apple CTA |
| `OnboardingView` | Display name + avatar setup (first launch only) |
| `HomeView` | Monthly calendar, session dot indicators, session cards. **Active session intercept:** if user has an active session, app opens directly to `SessionView` (1 tap to record). If no active session, opens to calendar. |
| `CreateSessionView` | Configure name, mode (named slots / auto-slot), section count, max section duration, deadline; generate invite link |
| `SessionView` | Active session: participant list, section/slot cards, record button, upload progress |
| `CameraView` | AVFoundation hold-to-record, preview, retake |
| `ReelPlayerView` | Full-screen AVPlayer for completed reel; save-to-camera-roll button; expiry warning banner at 75 days |
| `SettingsView` | Display name, avatar, notifications, account deletion |

### 6.2 Key Technical Notes
- **AVFoundation** for camera access and video capture; `recorded_at` captured at `captureOutput` callback time
- **AVAssetExportSession** for H.264 compression before upload
- **Background URLSession** for reliable uploads when the app is backgrounded
- **UserNotifications** for push handling and deep link routing into active or past sessions
- **Keychain** for secure JWT storage
- **async/await** for networking and state management in ViewModels

---

## 7. API Contract

| Endpoint | Method | Description |
|---|---|---|
| `/auth/apple` | POST | Exchange Apple identity token for JWT |
| `/users/me` | GET / PATCH | Get or update current user profile |
| `/users/me` | DELETE | Delete account; excludes user from active sessions |
| `/sessions` | POST | Create a new session (`{ name?, mode, section_count, max_section_duration_s, deadline, slots? }`) |
| `/sessions/:id` | GET | Get session details, participant list, section/slot status |
| `/sessions/:id/invite` | GET | Get the session invite link (Universal Link format) |
| `/sessions/join/:token` | POST | Join a session via invite token |
| `/sessions/:id/slots` | GET | Get slots for a named_slots session |
| `/sessions/:id/slots/:slot_id/skip` | POST | Mark a slot as "skip" for the current user |
| `/sessions/:id/slots/:slot_id/unskip` | POST | Un-skip a previously skipped slot |
| `/sessions/:id/clips/upload-url` | POST | Request presigned S3 upload URL |
| `/sessions/:id/clips` | POST | Confirm clip upload (`{ s3_key, recorded_at, duration_ms, slot_id? }`) |
| `/sessions/:id/reel` | GET | Get reel CDN URL once session is complete |

---

## 8. Non-Functional Requirements

| Requirement | Target |
|---|---|
| API response time (P95) | < 300ms for metadata endpoints |
| Concurrent uploads | Support 4 simultaneous uploads per session |
| Reel generation time | < 5 minutes from deadline to push notification |
| App cold start | < 2 seconds on iPhone 12 or newer |
| Uptime | 99.5% for API (MVP) |
| Raw clip retention | 7 days post-reel generation, then deleted from S3 |
| Reel retention | 90 days on CDN |
| Security | All traffic HTTPS/TLS 1.3; JWTs expire in 24h; S3 clips private by default |
| Privacy | No clip data shared outside the session; no analytics on clip content |

---

## 9. Milestones

### Phase 1 — Foundation
- Go API skeleton with project structure and routing
- PostgreSQL schema (all tables, indexes, migrations)
- Sign in with Apple + JWT auth flow
- AWS S3 bucket setup and presigned URL generation

### Phase 2 — Core iOS
- SwiftUI app shell and navigation structure
- Auth flow (Sign in with Apple, JWT storage in Keychain)
- Home / Calendar view (empty state)
- Session creation flow (mode, section count, max duration) with Universal Link invite generation

### Phase 3 — Recording
- Camera view with hold-to-record
- `recorded_at` captured at `captureOutput` callback
- Clip length enforcement per max section duration
- Client-side H.264 compression (AVAssetExportSession)
- Background upload via presigned URL with progress tracking

### Phase 4 — Reel Engine
- Timestamp validation (`recorded_at` vs `arrived_at`, ± 30 min tolerance + clamping)
- Section-based alignment algorithm (named slots path + auto-slot clustering path)
- Single-panel reel path for solo sessions
- FFmpeg split-screen composition (vstack, drawtext, concat)
- Redis job queue and Go worker pool
- Session scheduler (cron, deadline detection, job enqueue)
- APNs push delivery on reel completion

### Phase 5 — Polish & Integration
- Reel player (full-screen AVPlayer, scrubbing, timestamp overlay)
- Reminder push notifications (2h and 30min before deadline)
- Account deletion flow with in-session exclusion handling
- Error states across all flows (upload failure, reel failure)
- Edge case handling (0 submitters → soft push, deadline during processing)
- Save-to-camera-roll + expiry warning
- Upload retry queue (CoreData persistence for unconfirmed clips)
- End-to-end internal testing

### Phase 6 — Beta
- TestFlight release to a small group of real users (10–20)
- Bug fixes and crash triage
- Performance tuning (upload speed, reel generation time)
- Collect qualitative feedback on the core reel experience

---

## 10. Open Questions

### Resolved (v1)
- ~~**Zero submitters:**~~ → Push notification: "Session ended — start a new one?" No guilt, forward-looking tone.
- ~~**Invite link behaviour after join:**~~ → Same token stays active until deadline or 4-participant cap is reached.
- ~~**Clips in the same time-slot:**~~ → Resolved by section-based model. Multiple clips play sequentially within a section, totaling ≤ max section duration. Never drop content.

### Remaining Pre-Development Decisions
- **Auto-slot clustering threshold:** What temporal proximity threshold should be used to group clips into buckets? (e.g., 30min, 1h, adaptive based on session duration)
- **Slot time zone handling:** How to handle slots when participants are in different time zones?

### Post-MVP Considerations
- Emoji reactions on reel panels
- Android support
- More than 4 participants per session
- Multiple simultaneous sessions per user
- Advanced audio mixing/layering across panels (MVP: one panel at a time)
- Location or weather overlays on panels