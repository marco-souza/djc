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


