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
			want: "downloading (42%)",
		},
		{
			name: "downloaded status remains unchanged",
			song: library.Song{Status: "downloaded", Progress: 100},
			want: "downloaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatSongStatus(tt.song))
		})
	}
}
