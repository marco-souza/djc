package tui

import (
	"testing"

	"marco-souza/djc/internal/library"

	"github.com/stretchr/testify/assert"
)

func TestFormatSongStatus(t *testing.T) {
	tests := []struct {
		name string
		song library.Song
		want string
	}{
		{
			name: "downloading includes percentage",
			song: library.Song{Status: "downloading", Progress: 42},
			want: "Downloading… 42%",
		},
		{
			name: "downloaded status",
			song: library.Song{Status: "downloaded", Progress: 100},
			want: "Downloaded",
		},
		{
			name: "arbitrary status passthrough",
			song: library.Song{Status: "some other status"},
			want: "some other status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatSongStatus(tt.song))
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name  string
		pct   int
		width int
		want  string
	}{
		{name: "0%", pct: 0, width: 4, want: "[░░░░]"},
		{name: "50%", pct: 50, width: 4, want: "[██░░]"},
		{name: "100%", pct: 100, width: 4, want: "[████]"},
		{name: "clamped below 0", pct: -10, width: 4, want: "[░░░░]"},
		{name: "clamped above 100", pct: 200, width: 4, want: "[████]"},
		{name: "zero width", pct: 50, width: 0, want: "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, progressBar(tt.pct, tt.width))
		})
	}
}
