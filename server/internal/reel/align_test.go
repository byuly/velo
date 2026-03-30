package reel

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func makeSessionData(creatorID uuid.UUID, maxDur int) *SessionData {
	return &SessionData{
		Session: SessionRow{
			ID:                  uuid.New(),
			CreatorID:           &creatorID,
			MaxSectionDurationS: maxDur,
		},
	}
}

func TestAlign_TwoParticipantsTwoSlots(t *testing.T) {
	creator := uuid.New()
	other := uuid.New()
	slot1 := uuid.New()
	slot2 := uuid.New()
	clip1 := uuid.New()
	clip2 := uuid.New()
	clip3 := uuid.New()

	data := makeSessionData(creator, 15)
	data.Slots = []SlotRow{
		{ID: slot1, SlotOrder: 0, Name: "Morning"},
		{ID: slot2, SlotOrder: 1, Name: "Evening"},
	}
	data.Participants = []ParticipantRow{
		{UserID: creator, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
		{UserID: other, DisplayNameSnapshot: "Bob", JoinedAt: time.Now().Add(time.Minute), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: clip1, UserID: creator, SlotID: slot1, S3Key: "clips/c1.mp4", DurationMs: 5000},
		{ID: clip2, UserID: other, SlotID: slot1, S3Key: "clips/c2.mp4", DurationMs: 5000},
		{ID: clip3, UserID: creator, SlotID: slot2, S3Key: "clips/c3.mp4", DurationMs: 5000},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Len(t, result.Request.Sections, 2)
	require.Len(t, result.ClipsToFetch, 3)

	// Section 0: Alice (creator first) + Bob.
	s0 := result.Request.Sections[0]
	require.Len(t, s0.Participants, 2)
	require.Equal(t, "Alice", s0.Participants[0].Name)
	require.Equal(t, "Bob", s0.Participants[1].Name)
	require.Equal(t, float64(15), s0.Participants[0].Duration)
	require.Len(t, s0.Participants[0].LocalPaths, 1)
	require.Len(t, s0.Participants[1].LocalPaths, 1)
	require.Equal(t, 0, s0.AudioIdx) // 0 % 2

	// Section 1: Alice has clip, Bob has no clips → nil LocalPaths.
	s1 := result.Request.Sections[1]
	require.Len(t, s1.Participants, 2)
	require.Len(t, s1.Participants[0].LocalPaths, 1) // Alice
	require.Nil(t, s1.Participants[1].LocalPaths)      // Bob — no clip in slot2
	require.Equal(t, 1, s1.AudioIdx)                   // 1 % 2
}

func TestAlign_CreatorOrderedFirst(t *testing.T) {
	creator := uuid.New()
	earlier := uuid.New()

	data := makeSessionData(creator, 10)
	data.Slots = []SlotRow{{ID: uuid.New(), SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		// Earlier joined user listed first, but creator should sort first.
		{UserID: earlier, DisplayNameSnapshot: "EarlyUser", JoinedAt: time.Now().Add(-time.Hour), Status: "active"},
		{UserID: creator, DisplayNameSnapshot: "Creator", JoinedAt: time.Now(), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: earlier, SlotID: data.Slots[0].ID, S3Key: "c1.mp4"},
		{ID: uuid.New(), UserID: creator, SlotID: data.Slots[0].ID, S3Key: "c2.mp4"},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Equal(t, "Creator", result.Request.Sections[0].Participants[0].Name)
	require.Equal(t, "EarlyUser", result.Request.Sections[0].Participants[1].Name)
}

func TestAlign_SkippedSlot(t *testing.T) {
	user1 := uuid.New()
	user2 := uuid.New()
	slotID := uuid.New()

	data := makeSessionData(user1, 10)
	data.Slots = []SlotRow{{ID: slotID, SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		{UserID: user1, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
		{UserID: user2, DisplayNameSnapshot: "Bob", JoinedAt: time.Now().Add(time.Minute), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user1, SlotID: slotID, S3Key: "c1.mp4"},
		// Bob has a clip in a different slot (not this one), so he's "active with clips"
		// but for this particular slot he skipped.
	}
	// Give Bob a clip in a different slot so he passes the "has clips" filter.
	otherSlot := uuid.New()
	data.Slots = append(data.Slots, SlotRow{ID: otherSlot, SlotOrder: 1})
	data.Clips = append(data.Clips, ClipRow{ID: uuid.New(), UserID: user2, SlotID: otherSlot, S3Key: "c2.mp4"})

	data.Participations = []ParticipationRow{
		{SlotID: slotID, UserID: user2, Status: "skipped"},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)

	// Section 0: Alice has clip, Bob is skipped → nil LocalPaths.
	s0 := result.Request.Sections[0]
	require.Len(t, s0.Participants, 2)
	require.Len(t, s0.Participants[0].LocalPaths, 1) // Alice
	require.Nil(t, s0.Participants[1].LocalPaths)      // Bob skipped
}

func TestAlign_ZeroSubmitters(t *testing.T) {
	data := makeSessionData(uuid.New(), 10)
	data.Slots = []SlotRow{{ID: uuid.New(), SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		{UserID: uuid.New(), DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
	}
	// No clips at all.
	data.Clips = nil

	result := Align(data, "/tmp/work")
	require.Nil(t, result)
}

func TestAlign_ExcludedParticipant(t *testing.T) {
	user1 := uuid.New()
	user2 := uuid.New()
	slotID := uuid.New()

	data := makeSessionData(user1, 10)
	data.Slots = []SlotRow{{ID: slotID, SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		{UserID: user1, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
		{UserID: user2, DisplayNameSnapshot: "Bob", JoinedAt: time.Now().Add(time.Minute), Status: "excluded"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user1, SlotID: slotID, S3Key: "c1.mp4"},
		{ID: uuid.New(), UserID: user2, SlotID: slotID, S3Key: "c2.mp4"}, // has clips but excluded
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Len(t, result.Request.Sections[0].Participants, 1)
	require.Equal(t, "Alice", result.Request.Sections[0].Participants[0].Name)
}

func TestAlign_TitlePropagation(t *testing.T) {
	user := uuid.New()
	slotID := uuid.New()

	data := makeSessionData(user, 10)
	data.Slots = []SlotRow{{ID: slotID, SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		{UserID: user, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user, SlotID: slotID, S3Key: "c1.mp4"},
	}
	title := "commuting"
	data.Participations = []ParticipationRow{
		{SlotID: slotID, UserID: user, Status: "recording", Title: &title},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Equal(t, "commuting", result.Request.Sections[0].Participants[0].Title)
}

func TestAlign_AudioRotation(t *testing.T) {
	user1 := uuid.New()
	user2 := uuid.New()
	slot1 := uuid.New()
	slot2 := uuid.New()
	slot3 := uuid.New()

	data := makeSessionData(user1, 10)
	data.Slots = []SlotRow{
		{ID: slot1, SlotOrder: 0},
		{ID: slot2, SlotOrder: 1},
		{ID: slot3, SlotOrder: 2},
	}
	data.Participants = []ParticipantRow{
		{UserID: user1, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
		{UserID: user2, DisplayNameSnapshot: "Bob", JoinedAt: time.Now().Add(time.Minute), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user1, SlotID: slot1, S3Key: "c1.mp4"},
		{ID: uuid.New(), UserID: user2, SlotID: slot1, S3Key: "c2.mp4"},
		{ID: uuid.New(), UserID: user1, SlotID: slot2, S3Key: "c3.mp4"},
		{ID: uuid.New(), UserID: user2, SlotID: slot2, S3Key: "c4.mp4"},
		{ID: uuid.New(), UserID: user1, SlotID: slot3, S3Key: "c5.mp4"},
		{ID: uuid.New(), UserID: user2, SlotID: slot3, S3Key: "c6.mp4"},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Equal(t, 0, result.Request.Sections[0].AudioIdx) // 0 % 2
	require.Equal(t, 1, result.Request.Sections[1].AudioIdx) // 1 % 2
	require.Equal(t, 0, result.Request.Sections[2].AudioIdx) // 2 % 2
}

func TestAlign_SingleParticipant(t *testing.T) {
	user := uuid.New()
	slotID := uuid.New()

	data := makeSessionData(user, 10)
	data.Slots = []SlotRow{{ID: slotID, SlotOrder: 0}}
	data.Participants = []ParticipantRow{
		{UserID: user, DisplayNameSnapshot: "Solo", JoinedAt: time.Now(), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user, SlotID: slotID, S3Key: "c1.mp4"},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Len(t, result.Request.Sections, 1)
	require.Len(t, result.Request.Sections[0].Participants, 1)
	require.Equal(t, 0, result.Request.Sections[0].AudioIdx) // always 0 for single
}

func TestAlign_FourParticipants(t *testing.T) {
	users := [4]uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	slotID := uuid.New()

	data := makeSessionData(users[0], 10)
	data.Slots = []SlotRow{{ID: slotID, SlotOrder: 0}}
	for i, u := range users {
		data.Participants = append(data.Participants, ParticipantRow{
			UserID:              u,
			DisplayNameSnapshot: string(rune('A' + i)),
			JoinedAt:            time.Now().Add(time.Duration(i) * time.Minute),
			Status:              "active",
		})
		data.Clips = append(data.Clips, ClipRow{
			ID: uuid.New(), UserID: u, SlotID: slotID, S3Key: "c" + string(rune('1'+i)) + ".mp4",
		})
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)
	require.Len(t, result.Request.Sections[0].Participants, 4)
	require.Len(t, result.ClipsToFetch, 4)
}

func TestAlign_SkippedSlotWithTitle(t *testing.T) {
	user1 := uuid.New()
	user2 := uuid.New()
	slotID := uuid.New()
	otherSlot := uuid.New()

	data := makeSessionData(user1, 10)
	data.Slots = []SlotRow{
		{ID: slotID, SlotOrder: 0},
		{ID: otherSlot, SlotOrder: 1},
	}
	data.Participants = []ParticipantRow{
		{UserID: user1, DisplayNameSnapshot: "Alice", JoinedAt: time.Now(), Status: "active"},
		{UserID: user2, DisplayNameSnapshot: "Bob", JoinedAt: time.Now().Add(time.Minute), Status: "active"},
	}
	data.Clips = []ClipRow{
		{ID: uuid.New(), UserID: user1, SlotID: slotID, S3Key: "c1.mp4"},
		{ID: uuid.New(), UserID: user2, SlotID: otherSlot, S3Key: "c2.mp4"},
	}
	title := "away"
	data.Participations = []ParticipationRow{
		{SlotID: slotID, UserID: user2, Status: "skipped", Title: &title},
	}

	result := Align(data, "/tmp/work")
	require.NotNil(t, result)

	// Bob is skipped in slot 0 with title "away".
	bob := result.Request.Sections[0].Participants[1]
	require.Nil(t, bob.LocalPaths)
	require.Equal(t, "away", bob.Title)
}
