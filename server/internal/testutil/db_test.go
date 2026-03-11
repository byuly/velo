package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTestDB_PingSucceeds(t *testing.T) {
	pool := SetupTestDB(t)
	err := pool.Ping(context.Background())
	require.NoError(t, err)
}

func TestSetupTestDB_AllTablesExist(t *testing.T) {
	pool := SetupTestDB(t)
	ctx := context.Background()

	tables := []string{
		"users", "sessions", "session_slots",
		"session_participants", "slot_participations",
		"clips", "refresh_tokens",
	}

	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", table)
	}
}

func TestSetupTestDB_AllEnumTypesExist(t *testing.T) {
	pool := SetupTestDB(t)
	ctx := context.Background()

	enums := []string{
		"session_mode", "session_status",
		"participant_status", "slot_participation_status",
	}

	for _, enum := range enums {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_type WHERE typname = $1)`,
			enum,
		).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "enum type %s should exist", enum)
	}
}

func TestSetupTestDB_Isolation(t *testing.T) {
	t.Run("insert", func(t *testing.T) {
		pool := SetupTestDB(t)
		ctx := context.Background()

		_, err := pool.Exec(ctx,
			`INSERT INTO users (apple_sub) VALUES ('isolation_test_sub')`)
		require.NoError(t, err)

		var count int
		err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE apple_sub = 'isolation_test_sub'`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("absent", func(t *testing.T) {
		pool := SetupTestDB(t)
		ctx := context.Background()

		var count int
		err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE apple_sub = 'isolation_test_sub'`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "data from other test should not be visible")
	})
}

func TestFixtures(t *testing.T) {
	pool := SetupTestDB(t)

	user := CreateUser(t, pool, nil)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.AppleSub)

	session := CreateSession(t, pool, user.ID, nil)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "active", string(session.Status))

	slot := CreateSlot(t, pool, session.ID, nil)
	assert.NotEmpty(t, slot.ID)
	assert.Equal(t, "Default Slot", slot.Name)

	participant := CreateParticipant(t, pool, session.ID, user.ID, nil)
	assert.NotEmpty(t, participant.ID)

	clip := CreateClip(t, pool, session.ID, user.ID, nil)
	assert.NotEmpty(t, clip.ID)
	assert.Equal(t, 5000, clip.DurationMs)

	rt := CreateRefreshToken(t, pool, user.ID, nil)
	assert.NotEmpty(t, rt.ID)
	assert.NotEmpty(t, rt.TokenHash)
}
