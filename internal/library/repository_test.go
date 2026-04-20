package library

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositorySongLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "library.db")
	repo, err := Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })

	created, err := repo.CreateSong("https://youtube.com/watch?v=abc", "flac", "downloading")
	require.NoError(t, err)
	assert.Equal(t, "downloading", created.Status)
	assert.Equal(t, 0, created.Progress)

	err = repo.UpdateDownload(created.ID, "Song Name", "/music/song.flac", "downloaded", 100)
	require.NoError(t, err)

	song, err := repo.GetSong(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Song Name", song.Name)
	assert.Equal(t, "/music/song.flac", song.FilePath)
	assert.Equal(t, "downloaded", song.Status)
	assert.Equal(t, 100, song.Progress)

	songs, err := repo.ListSongs()
	require.NoError(t, err)
	require.Len(t, songs, 1)
	assert.Equal(t, created.ID, songs[0].ID)

	err = repo.DeleteSong(created.ID)
	require.NoError(t, err)

	songs, err = repo.ListSongs()
	require.NoError(t, err)
	assert.Empty(t, songs)
}
