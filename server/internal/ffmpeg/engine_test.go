package ffmpeg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompose_SingleParticipant_SingleSection(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 5.0},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width, "single participant = full 720×1280")
	require.Equal(t, 1280, pr.Height)
	require.True(t, pr.HasAudio)
	require.InDelta(t, 5.0, pr.Duration, 1.0, "duration ~5s")
}

func TestCompose_TwoParticipants_SingleSection(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 5.0},
					{LocalPaths: []string{normGreen5s}, Name: "Bob", Duration: 5.0},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	// 2-panel vstack: 720×640 each → 720×1280 total
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.True(t, pr.HasAudio)
}

func TestCompose_BlackPanel_MissingParticipant(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 5.0},
					{LocalPaths: nil, Name: "Bob", Duration: 5.0}, // no clips
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.True(t, pr.HasAudio)
	require.InDelta(t, 5.0, pr.Duration, 1.0, "should match section duration")
}

func TestCompose_MultiSection(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 5.0},
					{LocalPaths: []string{normGreen5s}, Name: "Bob", Duration: 5.0},
				},
				AudioIdx: 0,
			},
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 5.0},
					{LocalPaths: []string{normGreen5s}, Name: "Bob", Duration: 5.0},
				},
				AudioIdx: 1, // rotated audio
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.InDelta(t, 10.0, pr.Duration, 2.0, "two 5s sections ≈ 10s")
}

func TestCompose_PaddingToSectionDuration(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	// normGreen5s is ~5s, but section Duration is 10s → should be padded.
	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Duration: 10.0},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.InDelta(t, 10.0, pr.Duration, 1.5, "padded to section duration")
}

func TestCompose_MultiClipsPerParticipant(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	// Two clips for Alice in one section → concatenated.
	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{
						LocalPaths: []string{normGreen5s, normGreen5s},
						Name:       "Alice",
						Duration:   10.0,
					},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.InDelta(t, 10.0, pr.Duration, 2.0, "two 5s clips ≈ 10s")
}

func TestCompose_WithTitle(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Title: "sleeping", Duration: 5.0},
					{LocalPaths: []string{normGreen5s}, Name: "Bob", Title: "", Duration: 5.0},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.True(t, pr.HasAudio)
}

func TestCompose_BlackPanel_WithTitle(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	c, err := New()
	require.NoError(t, err)
	eng := NewEngine(c)

	req := ComposeRequest{
		WorkDir: t.TempDir(),
		Sections: []SectionRequest{
			{
				Participants: []ParticipantPanel{
					{LocalPaths: []string{normGreen5s}, Name: "Alice", Title: "commuting", Duration: 5.0},
					{LocalPaths: nil, Name: "Bob", Title: "away", Duration: 5.0},
				},
				AudioIdx: 0,
			},
		},
	}

	out, err := eng.Compose(ctx, req)
	require.NoError(t, err)

	pr, err := c.Probe(ctx, out)
	require.NoError(t, err)
	require.Equal(t, 720, pr.Width)
	require.Equal(t, 1280, pr.Height)
	require.InDelta(t, 5.0, pr.Duration, 1.0)
}
