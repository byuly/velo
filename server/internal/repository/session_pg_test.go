package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byuly/velo/server/internal/domain"
	"github.com/byuly/velo/server/internal/repository"
	"github.com/byuly/velo/server/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newSessionInput(creatorID uuid.UUID) domain.Session {
	return domain.Session{
		CreatorID:           &creatorID,
		Mode:                domain.SessionModeNamedSlots,
		SectionCount:        3,
		MaxSectionDurationS: 15,
		Deadline:            time.Now().Add(24 * time.Hour),
		InviteToken:         uuid.New().String(),
		Status:              domain.SessionStatusActive,
	}
}

func newSlots(n int) []domain.Slot {
	slots := make([]domain.Slot, n)
	for i := 0; i < n; i++ {
		slots[i] = domain.Slot{
			Name:      "Slot " + uuid.New().String()[:4],
			StartsAt:  domain.TimeOfDay{Hour: 9 + i, Minute: 0},
			EndsAt:    domain.TimeOfDay{Hour: 10 + i, Minute: 0},
			SlotOrder: i,
		}
	}
	return slots
}

// --- Create ---

func TestSessionCreate_WithSlots(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	input := newSessionInput(creator.ID)
	slots := newSlots(3)

	created, err := repo.Create(ctx, input, slots)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, creator.ID, *created.CreatorID)
	require.Equal(t, domain.SessionStatusActive, created.Status)
	require.Equal(t, input.InviteToken, created.InviteToken)

	got, err := repo.GetSlots(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, 0, got[0].SlotOrder)
	require.Equal(t, domain.TimeOfDay{Hour: 9}, got[0].StartsAt)
	require.Equal(t, domain.TimeOfDay{Hour: 10}, got[0].EndsAt)
}

// --- GetByID / GetByInviteToken ---

func TestSessionGetByID_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	creator := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateSession(t, pool, creator.ID, nil)

	got, err := repo.GetByID(context.Background(), fixture.ID)
	require.NoError(t, err)
	require.Equal(t, fixture.ID, got.ID)
	require.Equal(t, fixture.InviteToken, got.InviteToken)
}

func TestSessionGetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestSessionGetByInviteToken_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	creator := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		InviteToken: testutil.Ptr("token-xyz"),
	})

	got, err := repo.GetByInviteToken(context.Background(), "token-xyz")
	require.NoError(t, err)
	require.Equal(t, fixture.ID, got.ID)
}

func TestSessionGetByInviteToken_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	_, err := repo.GetByInviteToken(context.Background(), "nope")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- UpdateStatus / Cancel ---

func TestSessionUpdateStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	creator := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateSession(t, pool, creator.ID, nil)

	err := repo.UpdateStatus(context.Background(), fixture.ID, domain.SessionStatusGenerating)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), fixture.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SessionStatusGenerating, got.Status)
}

func TestSessionUpdateStatus_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	err := repo.UpdateStatus(context.Background(), uuid.New(), domain.SessionStatusComplete)
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestSessionCancel(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	creator := testutil.CreateUser(t, pool, nil)
	fixture := testutil.CreateSession(t, pool, creator.ID, nil)

	err := repo.Cancel(context.Background(), fixture.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), fixture.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SessionStatusCancelled, got.Status)
}

func TestSessionCancel_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	err := repo.Cancel(context.Background(), uuid.New())
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- AddParticipant ---

func TestAddParticipant_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, nil)
	user := testutil.CreateUser(t, pool, nil)

	p, err := repo.AddParticipant(ctx, session.ID, user.ID, "Alice")
	require.NoError(t, err)
	require.Equal(t, session.ID, p.SessionID)
	require.NotNil(t, p.UserID)
	require.Equal(t, user.ID, *p.UserID)
	require.Equal(t, "Alice", p.DisplayNameSnapshot)
	require.Equal(t, domain.ParticipantStatusActive, p.Status)
}

func TestAddParticipant_SessionFull(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, nil)

	for i := 0; i < domain.MaxParticipants; i++ {
		u := testutil.CreateUser(t, pool, nil)
		testutil.CreateParticipant(t, pool, session.ID, u.ID, nil)
	}

	extra := testutil.CreateUser(t, pool, nil)
	_, err := repo.AddParticipant(ctx, session.ID, extra.ID, "Late")
	require.True(t, errors.Is(err, domain.ErrSessionFull))
}

func TestAddParticipant_Duplicate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, nil)
	user := testutil.CreateUser(t, pool, nil)

	_, err := repo.AddParticipant(ctx, session.ID, user.ID, "Alice")
	require.NoError(t, err)

	_, err = repo.AddParticipant(ctx, session.ID, user.ID, "Alice")
	require.True(t, errors.Is(err, domain.ErrAlreadyInSession))
}

func TestAddParticipant_AlreadyInOtherSession(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	other := testutil.CreateSession(t, pool, creator.ID, nil)
	user := testutil.CreateUser(t, pool, nil)
	testutil.CreateParticipant(t, pool, other.ID, user.ID, nil)

	target := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		InviteToken: testutil.Ptr(uuid.New().String()),
	})

	_, err := repo.AddParticipant(ctx, target.ID, user.ID, "Alice")
	require.True(t, errors.Is(err, domain.ErrAlreadyInSession))
}

func TestAddParticipant_SessionNotActive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Status: testutil.Ptr(domain.SessionStatusCancelled),
	})
	user := testutil.CreateUser(t, pool, nil)

	_, err := repo.AddParticipant(ctx, session.ID, user.ID, "Alice")
	require.True(t, errors.Is(err, domain.ErrSessionNotActive))
}

func TestAddParticipant_SessionNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	user := testutil.CreateUser(t, pool, nil)
	_, err := repo.AddParticipant(context.Background(), uuid.New(), user.ID, "Alice")
	require.True(t, errors.Is(err, domain.ErrNotFound))
}

// --- GetActiveSessionForUser ---

func TestGetActiveSessionForUser_Found(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, nil)
	user := testutil.CreateUser(t, pool, nil)
	testutil.CreateParticipant(t, pool, session.ID, user.ID, nil)

	got, err := repo.GetActiveSessionForUser(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, session.ID, got.ID)
}

func TestGetActiveSessionForUser_None(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	user := testutil.CreateUser(t, pool, nil)
	got, err := repo.GetActiveSessionForUser(context.Background(), user.ID)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestGetActiveSessionForUser_IgnoresCancelledSession(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Status: testutil.Ptr(domain.SessionStatusCancelled),
	})
	user := testutil.CreateUser(t, pool, nil)
	testutil.CreateParticipant(t, pool, session.ID, user.ID, nil)

	got, err := repo.GetActiveSessionForUser(context.Background(), user.ID)
	require.NoError(t, err)
	require.Nil(t, got)
}

// --- UpsertSlotParticipation ---

func TestUpsertSlotParticipation_InsertThenUpdate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()

	creator := testutil.CreateUser(t, pool, nil)
	session := testutil.CreateSession(t, pool, creator.ID, nil)
	slot := testutil.CreateSlot(t, pool, session.ID, nil)
	user := testutil.CreateUser(t, pool, nil)

	err := repo.UpsertSlotParticipation(ctx, slot.ID, user.ID, domain.SlotParticipationStatusSkipped)
	require.NoError(t, err)

	var status domain.SlotParticipationStatus
	err = pool.QueryRow(ctx,
		`SELECT status FROM slot_participations WHERE slot_id = $1 AND user_id = $2`, slot.ID, user.ID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, domain.SlotParticipationStatusSkipped, status)

	err = repo.UpsertSlotParticipation(ctx, slot.ID, user.ID, domain.SlotParticipationStatusRecording)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT status FROM slot_participations WHERE slot_id = $1 AND user_id = $2`, slot.ID, user.ID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, domain.SlotParticipationStatusRecording, status)

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM slot_participations WHERE slot_id = $1 AND user_id = $2`, slot.ID, user.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// --- Scheduler queries ---

func TestGetActiveSessionsPastDeadline(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()
	creator := testutil.CreateUser(t, pool, nil)

	past := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(-time.Hour)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})
	// Future — should be excluded.
	testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(time.Hour)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})
	// Past but cancelled — excluded.
	testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(-time.Hour)),
		Status:      testutil.Ptr(domain.SessionStatusCancelled),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})

	got, err := repo.GetActiveSessionsPastDeadline(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, past.ID, got[0].ID)
}

func TestGetSessionsNeedingReminder_2h(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()
	creator := testutil.CreateUser(t, pool, nil)

	// In-window — included.
	within := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(90 * time.Minute)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})
	// Past deadline — excluded.
	testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(-10 * time.Minute)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})
	// Beyond window — excluded.
	testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(3 * time.Hour)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})

	got, err := repo.GetSessionsNeedingReminder(ctx, 2*time.Hour)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, within.ID, got[0].ID)

	// Flip the 2h flag — should drop out.
	_, err = pool.Exec(ctx, `UPDATE sessions SET reminder_2h_sent = true WHERE id = $1`, within.ID)
	require.NoError(t, err)
	got, err = repo.GetSessionsNeedingReminder(ctx, 2*time.Hour)
	require.NoError(t, err)
	require.Len(t, got, 0)
}

func TestGetSessionsNeedingReminder_30m(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)
	ctx := context.Background()
	creator := testutil.CreateUser(t, pool, nil)

	within := testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(15 * time.Minute)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})
	// 2h window candidate but outside 30m — excluded.
	testutil.CreateSession(t, pool, creator.ID, &testutil.SessionOverrides{
		Deadline:    testutil.Ptr(time.Now().Add(90 * time.Minute)),
		InviteToken: testutil.Ptr(uuid.New().String()),
	})

	got, err := repo.GetSessionsNeedingReminder(ctx, 30*time.Minute)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, within.ID, got[0].ID)
}

func TestGetSessionsNeedingReminder_UnsupportedWindow(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewSessionPg(pool)

	_, err := repo.GetSessionsNeedingReminder(context.Background(), time.Hour)
	require.Error(t, err)
}
