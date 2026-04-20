package tui

import (
	"context"
	"os/exec"

	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/youtube"
)

// ── mode ────────────────────────────────────────────────────────────────────

type Mode int

const (
	modeList Mode = iota
	modeAdd
	modeConfirm // confirmation / playlist-select modal
	modeDelete
	modeConfig
)

// ── messages ────────────────────────────────────────────────────────────────

type downloadEvent struct {
	SongID    int64
	Name      string
	Format    string
	FilePath  string
	Progress  int
	Status    string
	Completed bool
	Err       error
	// IsNew is set when this event introduces a brand-new Song row (playlist track).
	// The TUI should prepend NewSong to its song list.
	IsNew   bool
	NewSong library.Song
}

type downloadStartedMsg struct {
	Song       library.Song
	ch         <-chan downloadEvent
	cancel     context.CancelFunc
	reDownload bool // true when refreshing an existing song (don't prepend to list)
}

type downloadUpdateMsg struct {
	event downloadEvent
	ch    <-chan downloadEvent
	ok    bool
}

type actionDoneMsg struct {
	message string
	err     error
}

type refreshMsg struct {
	songs []library.Song
	err   error
}

type playbackStartedMsg struct {
	songID int64
	name   string
	proc   *exec.Cmd
	ipc    string // mpv IPC socket path, empty for other players
}

type playbackEndedMsg struct {
	songID int64
}

// playerTickMsg is sent every second while a track is playing to update the elapsed time.
type playerTickMsg struct{}

// mpvDurationMsg carries the total duration of the current track, queried via mpv IPC.
type mpvDurationMsg struct{ duration float64 }

// metadataFetchedMsg carries the result of a metadata-fetch operation.
type metadataFetchedMsg struct {
	items []youtube.VideoInfo
	url   string
	err   error
}

// downloadsQueuedMsg is sent after Song rows have been created for queued downloads.
type downloadsQueuedMsg struct {
	songs []library.Song
	urls  []string
}

// queuedDownload holds a song that is waiting its turn in the download queue.
type queuedDownload struct {
	Song library.Song
	URL  string
}
