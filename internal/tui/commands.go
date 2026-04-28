package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/youtube"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ── commands ────────────────────────────────────────────────────────────────

func refreshSongsCmd(repo *library.Repository) tea.Cmd {
	return func() tea.Msg {
		songs, err := repo.ListSongs()
		return refreshMsg{songs: songs, err: err}
	}
}

// loadQueuedSongsCmd loads all songs with status "queued" from the database
// so they can be resumed on startup. It first resets any songs stuck in
// "downloading" (e.g., from a crash) back to "queued".
func loadQueuedSongsCmd(repo *library.Repository) tea.Cmd {
	return func() tea.Msg {
		// Recover from potential crash: reset stuck "downloading" songs → "queued"
		if _, err := repo.ResetDownloadingToQueued(); err != nil {
			return queuedSongsLoadedMsg{err: fmt.Errorf("reset stuck downloads: %w", err)}
		}

		songs, err := repo.GetQueuedSongs()
		return queuedSongsLoadedMsg{songs: songs, err: err}
	}
}

// fetchMetadataCmd queries yt-dlp for video/playlist info without downloading.
func fetchMetadataCmd(url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		items, err := youtube.FetchMetadata(ctx, url)
		return metadataFetchedMsg{items: items, url: url, err: err}
	}
}

// createQueuedDownloadsCmd creates Song rows in the DB for each item (status "queued")
// and returns a downloadsQueuedMsg so the TUI can enqueue and display them.
func createQueuedDownloadsCmd(repo *library.Repository, cfg *config.Config, items []youtube.VideoInfo) tea.Cmd {
	return func() tea.Msg {
		songs := make([]library.Song, 0, len(items))
		urls := make([]string, 0, len(items))
		for _, item := range items {
			song, err := repo.CreateSong(item.URL, cfg.AudioFormat, "queued")
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("create queued song: %w", err)}
			}
			songs = append(songs, song)
			urls = append(urls, item.URL)
		}
		return downloadsQueuedMsg{songs: songs, urls: urls}
	}
}

// startQueuedDownloadCmd starts downloading a song that is already in the DB
// (created with status "queued"). It sets reDownload=true so the TUI updates
// the existing row instead of prepending a duplicate.
func startQueuedDownloadCmd(repo *library.Repository, cfg *config.Config, queued queuedDownload) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan downloadEvent)
		ctx, cancel := context.WithCancel(context.Background())
		go downloadSong(ctx, repo, cfg, queued.Song, queued.URL, ch)
		return downloadStartedMsg{Song: queued.Song, ch: ch, cancel: cancel, reDownload: true}
	}
}

func waitForDownloadUpdate(ch <-chan downloadEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		return downloadUpdateMsg{event: event, ch: ch, ok: ok}
	}
}

func downloadSong(ctx context.Context, repo *library.Repository, cfg *config.Config,
	song library.Song, url string, ch chan<- downloadEvent) {

	defer close(ch)

	// For playlist downloads, each track is identified by its PlaylistIndex.
	// The pre-created placeholder `song` row is reused for the first track.
	// Subsequent tracks get new Song rows created on the fly.
	type trackState struct {
		id          int64
		latestPath  string
		latestName  string
		errReported bool
	}
	var (
		tracks     = make(map[int]*trackState) // PlaylistIndex → state
		firstIndex = -1                        // -1 until we see the first playlist track
	)

	var latestFilePath, latestName string
	var updateErrReported bool

	_, err := youtube.DownloadAudioWithProgress(ctx, url, cfg.AudioFormat, nil, cfg,
		func(progress youtube.DownloadProgress) {
			if progress.PlaylistIndex > 0 {
				// ── Playlist track ──────────────────────────────────────
				pIdx := progress.PlaylistIndex
				track, exists := tracks[pIdx]
				if !exists {
					if firstIndex == -1 {
						// Reuse the pre-created placeholder for the first track.
						firstIndex = pIdx
						track = &trackState{id: song.ID}
					} else {
						// Create a new Song row for this track.
						newSong, createErr := repo.CreateSong(url, cfg.AudioFormat, "downloading")
						if createErr != nil {
							ch <- downloadEvent{SongID: song.ID, Err: fmt.Errorf("create playlist song: %w", createErr)}
							return
						}
						track = &trackState{id: newSong.ID}
						// Notify TUI to add this row to the list.
						ch <- downloadEvent{IsNew: true, SongID: newSong.ID, NewSong: newSong}
					}
					tracks[pIdx] = track
				}

				if progress.FilePath != "" {
					track.latestPath = progress.FilePath
				}
				if progress.Name != "" {
					track.latestName = progress.Name
				}

				status := "downloading"
				if progress.Completed {
					status = "downloaded"
				}

				repoErr := repo.UpdateDownload(track.id, pick(progress.Name, song.Name), track.latestPath, status, progress.Percent)
				if repoErr != nil && !track.errReported {
					track.errReported = true
					ch <- downloadEvent{SongID: track.id, Err: fmt.Errorf("update playlist song progress: %w", repoErr)}
					return
				}
				ch <- downloadEvent{
					SongID:    track.id,
					Name:      pick(progress.Name, song.Name),
					Format:    pick(progress.Format, song.Format),
					FilePath:  track.latestPath,
					Progress:  progress.Percent,
					Status:    status,
					Completed: progress.Completed,
				}
				return
			}

			// ── Single-song (non-playlist) track ─────────────────────────
			if progress.FilePath != "" {
				latestFilePath = progress.FilePath
			}
			if progress.Name != "" {
				latestName = progress.Name
			}

			status := "downloading"
			if progress.Completed {
				status = "downloaded"
			}

			repoErr := repo.UpdateDownload(song.ID, pick(progress.Name, song.Name), latestFilePath, status, progress.Percent)
			if repoErr != nil && !updateErrReported {
				updateErrReported = true
				ch <- downloadEvent{SongID: song.ID, Err: fmt.Errorf("update song progress: %w", repoErr)}
				return // skip the progress event this iteration; the UI error message is sufficient
			}
			ch <- downloadEvent{
				SongID:    song.ID,
				Name:      pick(progress.Name, song.Name),
				Format:    pick(progress.Format, song.Format),
				FilePath:  latestFilePath,
				Progress:  progress.Percent,
				Status:    status,
				Completed: progress.Completed,
			}
		})

	if err != nil {
		if len(tracks) > 0 {
			// Mark all incomplete playlist tracks as failed.
			errStatus := fmt.Sprintf("failed: %s", err.Error())
			for _, track := range tracks {
				if track.latestName == "" {
					track.latestName = song.Name
				}
				_ = repo.UpdateDownload(track.id, track.latestName, track.latestPath, errStatus, 0)
				ch <- downloadEvent{SongID: track.id, Status: errStatus, Err: err}
			}
			return
		}
		// Single-song error path.
		errStatus := fmt.Sprintf("failed: %s", err.Error())
		if repoErr := repo.UpdateDownload(song.ID, pick(latestName, song.Name), latestFilePath, errStatus, 0); repoErr != nil {
			err = fmt.Errorf("%w (status save failed: %v)", err, repoErr)
		}
		ch <- downloadEvent{SongID: song.ID, Status: errStatus, Err: err}
		return
	}

	// Finalize single-song downloads (playlist tracks are already finalized per-track).
	if len(tracks) == 0 {
		if latestName == "" {
			latestName = song.Name
		}

		// Verify the file exists - yt-dlp may report intermediate filename (.mp4)
		// but actual file after audio extraction has different extension (.flac, etc.)
		actualFilePath := latestFilePath
		if _, statErr := os.Stat(latestFilePath); statErr != nil {
			// File not found at reported path, search for it with correct extension
			dir := filepath.Dir(latestFilePath)
			base := strings.TrimSuffix(filepath.Base(latestFilePath), filepath.Ext(filepath.Base(latestFilePath)))
			// Look for file with expected audio extension
			expectedExt := "." + song.Format
			candidatePath := filepath.Join(dir, base+expectedExt)
			if _, checkErr := os.Stat(candidatePath); checkErr == nil {
				actualFilePath = candidatePath
			}
		}

		if repoErr := repo.UpdateDownload(song.ID, latestName, actualFilePath, "downloaded", 100); repoErr != nil {
			ch <- downloadEvent{SongID: song.ID, Err: fmt.Errorf("mark song as downloaded: %w", repoErr)}
			return
		}
		ch <- downloadEvent{
			SongID:    song.ID,
			Name:      latestName,
			Format:    extensionOr(song.Format, actualFilePath),
			FilePath:  actualFilePath,
			Progress:  100,
			Status:    "downloaded",
			Completed: true,
		}
	}
}

func saveConfigCmd(cfg *config.Config, inputs [5]textinput.Model) tea.Cmd {
	// Capture the new values before the closure so the mutation happens on the
	// shared Config pointer from the bubbletea Update goroutine, not from the
	// async Cmd goroutine, avoiding a data race.
	downloadDir := strings.TrimSpace(inputs[0].Value())
	audioFormat := strings.TrimSpace(inputs[1].Value())
	audioQuality := strings.TrimSpace(inputs[2].Value())
	outputTemplate := strings.TrimSpace(inputs[3].Value())
	downloadWorkersStr := strings.TrimSpace(inputs[4].Value())
	return func() tea.Msg {
		cfg.DownloadDir = downloadDir
		cfg.AudioFormat = audioFormat
		cfg.AudioQuality = audioQuality
		cfg.OutputTemplate = outputTemplate
		if workers, err := strconv.Atoi(downloadWorkersStr); err == nil {
			if workers < config.MinDownloadWorkers {
				workers = config.MinDownloadWorkers
			}
			if workers > config.MaxDownloadWorkers {
				workers = config.MaxDownloadWorkers
			}
			cfg.DownloadWorkers = workers
		}
		if err := cfg.Save(); err != nil {
			return actionDoneMsg{err: fmt.Errorf("save config: %w", err)}
		}
		return actionDoneMsg{message: "Configuration saved!"}
	}
}

func exportSongCmd(song library.Song) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(song.FilePath) == "" {
			return actionDoneMsg{err: fmt.Errorf("song has no file path")}
		}
		ext := strings.ToLower(filepath.Ext(song.FilePath))
		if ext == ".mp3" {
			return actionDoneMsg{message: "Already an MP3"}
		}
		out := strings.TrimSuffix(song.FilePath, filepath.Ext(song.FilePath)) + ".mp3"
		cmd := exec.Command("ffmpeg", "-y", "-i", song.FilePath, out)
		cmd.Env = os.Environ()
		if o, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(o))
			if msg == "" {
				msg = err.Error()
			}
			return actionDoneMsg{err: fmt.Errorf("ffmpeg: %s", msg)}
		}
		return actionDoneMsg{message: "Exported → " + out}
	}
}

func deleteSongCmd(repo *library.Repository, song library.Song) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(song.FilePath) != "" {
			if err := os.Remove(song.FilePath); err != nil && !os.IsNotExist(err) {
				return actionDoneMsg{err: fmt.Errorf("delete file: %w", err)}
			}
		}
		if err := repo.DeleteSong(song.ID); err != nil {
			return actionDoneMsg{err: fmt.Errorf("delete song: %w", err)}
		}
		return actionDoneMsg{message: "Song deleted"}
	}
}

// dropDBCmd deletes all songs from the database and removes all downloaded files.
func dropDBCmd(repo *library.Repository) tea.Cmd {
	return func() tea.Msg {
		songs, err := repo.ListSongs()
		if err != nil {
			return actionDoneMsg{err: fmt.Errorf("list songs: %w", err)}
		}

		// Delete all downloaded files.
		for _, song := range songs {
			if strings.TrimSpace(song.FilePath) != "" {
				if err := os.Remove(song.FilePath); err != nil && !os.IsNotExist(err) {
					return actionDoneMsg{err: fmt.Errorf("delete file %s: %w", song.FilePath, err)}
				}
			}
		}

		// Delete all songs from database.
		for _, song := range songs {
			if err := repo.DeleteSong(song.ID); err != nil {
				return actionDoneMsg{err: fmt.Errorf("delete song from db: %w", err)}
			}
		}

		return actionDoneMsg{message: "Database dropped"}
	}
}

// refreshSelectedSongCmd attempts to reconcile the selected song with the
// filesystem, in order:
//  1. File already exists at the recorded path → move to DownloadDir if needed,
//     update the DB entry and report success.
//  2. File not at recorded path → search DownloadDir for a matching file.
//     If found, update the DB entry and report success.
//  3. Not found anywhere → trigger a re-download from the song's SourceURL.
func refreshSelectedSongCmd(repo *library.Repository, cfg *config.Config, song library.Song) tea.Cmd {
	return func() tea.Msg {
		// 1. File exists at the recorded path.
		if song.FilePath != "" {
			if _, err := os.Stat(song.FilePath); err == nil {
				dest := song.FilePath
				// Move to DownloadDir if it lives elsewhere.
				if !strings.HasPrefix(filepath.Clean(song.FilePath)+string(filepath.Separator),
					filepath.Clean(cfg.DownloadDir)+string(filepath.Separator)) {
					if err2 := os.MkdirAll(cfg.DownloadDir, 0o755); err2 == nil {
						target := filepath.Join(cfg.DownloadDir, filepath.Base(song.FilePath))
						if err2 = os.Rename(song.FilePath, target); err2 == nil {
							dest = target
						}
					}
				}
				if err2 := repo.UpdateDownload(song.ID, song.Name, dest, "downloaded", 100); err2 != nil {
					return actionDoneMsg{err: fmt.Errorf("refresh: %w", err2)}
				}
				return actionDoneMsg{message: "Song is up-to-date"}
			}
		}

		// 2. Search DownloadDir for a matching file.
		if found := findSongFile(song, cfg.DownloadDir); found != "" {
			if err := repo.UpdateDownload(song.ID, song.Name, found, "downloaded", 100); err != nil {
				return actionDoneMsg{err: fmt.Errorf("refresh: %w", err)}
			}
			return actionDoneMsg{message: "Song located and refreshed"}
		}

		// 3. Re-download from source.
		if song.SourceURL == "" {
			return actionDoneMsg{err: fmt.Errorf("refresh: file not found and no source URL to re-download from")}
		}
		ch := make(chan downloadEvent)
		ctx, cancel := context.WithCancel(context.Background())
		go downloadSong(ctx, repo, cfg, song, song.SourceURL, ch)
		return downloadStartedMsg{Song: song, ch: ch, cancel: cancel, reDownload: true}
	}
}

// findSongFile walks dir and returns the first file whose base name (with or
// without extension) matches the recorded FilePath's base name or the song Name.
func findSongFile(song library.Song, dir string) string {
	var targets []string
	if song.FilePath != "" {
		base := strings.ToLower(filepath.Base(song.FilePath))
		targets = append(targets, base)
		targets = append(targets, strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base))))
	}
	if song.Name != "" {
		targets = append(targets, strings.ToLower(song.Name))
	}
	if len(targets) == 0 {
		return ""
	}

	var found string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || found != "" {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
		for _, t := range targets {
			if base == t || stem == t {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// ── playback commands ────────────────────────────────────────────────────────

// playSongCmd starts the system audio player for the given song in a subprocess.
// stdin/stdout/stderr are redirected to os.DevNull so the player never interacts
// with the TUI's raw terminal (which would cause it to exit immediately).
func playSongCmd(song library.Song) tea.Cmd {
	return func() tea.Msg {
		// Verify the file exists before attempting to play.
		if _, err := os.Stat(song.FilePath); err != nil {
			if os.IsNotExist(err) {
				return actionDoneMsg{err: fmt.Errorf("file not found: %s", song.FilePath)}
			}
			return actionDoneMsg{err: fmt.Errorf("file not accessible: %w", err)}
		}

		player, args, ipc := findPlayer()
		if player == "" {
			return actionDoneMsg{err: fmt.Errorf("no supported audio player found (install ffplay, mpv, afplay, or aplay)")}
		}
		args = append(args, song.FilePath)
		cmd := exec.Command(player, args...)

		// Open /dev/null for stdin (read) and stdout/stderr (write) separately.
		// Using a single read-only handle for write fds would cause the player to fail.
		stdinNull, err := os.Open(os.DevNull)
		if err == nil {
			stdoutNull, werr := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
			if werr == nil {
				cmd.Stdin = stdinNull
				cmd.Stdout = stdoutNull
				cmd.Stderr = stdoutNull
			} else {
				_ = stdinNull.Close()
			}
		}

		if err := cmd.Start(); err != nil {
			// Close the /dev/null handles if Start fails.
			if f, ok := cmd.Stdin.(*os.File); ok {
				_ = f.Close()
			}
			if f, ok := cmd.Stdout.(*os.File); ok {
				_ = f.Close()
			}
			return actionDoneMsg{err: fmt.Errorf("start player: %w", err)}
		}
		// Close our copies of the /dev/null handles; the child process has its own fds.
		if f, ok := cmd.Stdin.(*os.File); ok {
			_ = f.Close()
		}
		if f, ok := cmd.Stdout.(*os.File); ok {
			_ = f.Close()
		}
		return playbackStartedMsg{songID: song.ID, name: song.Name, proc: cmd, ipc: ipc}
	}
}

// waitForPlaybackEndCmd blocks until the player process exits, then notifies the TUI.
func waitForPlaybackEndCmd(songID int64, proc *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		_ = proc.Wait()
		return playbackEndedMsg{songID: songID}
	}
}

// playerTickCmd fires a tick every second to update the elapsed-time display.
func playerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return playerTickMsg{}
	})
}

// getMpvDurationCmd queries the total duration of the current track from mpv via IPC.
// It retries for up to ~2 s to allow mpv time to open the file and set up the socket.
const (
	mpvDurationRetries  = 5
	mpvDurationInterval = 400 * time.Millisecond
)

func getMpvDurationCmd(ipc string) tea.Cmd {
	return func() tea.Msg {
		for range mpvDurationRetries {
			time.Sleep(mpvDurationInterval)
			dur, err := mpvGetFloat(ipc, "duration")
			if err == nil && dur > 0 {
				return mpvDurationMsg{duration: dur}
			}
		}
		return mpvDurationMsg{} // duration unknown
	}
}

// mpvSeekCmd sends a relative seek command to mpv (delta in seconds).
func mpvSeekCmd(ipc string, delta float64) tea.Cmd {
	return func() tea.Msg {
		if err := mpvSendCmd(ipc, []interface{}{"seek", delta, "relative"}); err != nil {
			return actionDoneMsg{err: fmt.Errorf("seek: %w", err)}
		}
		return nil
	}
}

// mpvVolumeCmd sets the playback volume (0–100) on mpv.
func mpvVolumeCmd(ipc string, vol int) tea.Cmd {
	return func() tea.Msg {
		_ = mpvSendCmd(ipc, []interface{}{"set_property", "volume", vol})
		return nil
	}
}

// findPlayer returns the first available audio player binary, its arguments, and an
// optional mpv IPC socket path.  mpv is tried first because its JSON IPC socket
// enables seek, volume control, and duration queries.
func findPlayer() (player string, args []string, ipc string) {
	if p, err := exec.LookPath("mpv"); err == nil {
		socket := fmt.Sprintf("/tmp/djc-mpv-%d.sock", os.Getpid())
		return p, []string{
			"--no-video",
			"--really-quiet",
			"--input-ipc-server=" + socket,
		}, socket
	}
	if p, err := exec.LookPath("ffplay"); err == nil {
		return p, []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}, ""
	}
	if p, err := exec.LookPath("afplay"); err == nil { // macOS
		return p, nil, ""
	}
	if p, err := exec.LookPath("aplay"); err == nil { // Linux ALSA
		return p, nil, ""
	}
	return "", nil, ""
}

// ── mpv IPC helpers ──────────────────────────────────────────────────────────

// mpvSendCmd writes a JSON command to the mpv IPC socket (fire-and-forget).
func mpvSendCmd(socket string, command []interface{}) error {
	msg, err := json.Marshal(map[string]interface{}{"command": command})
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(append(msg, '\n'))
	return err
}

// mpvGetFloat queries a numeric property from the mpv IPC socket.
func mpvGetFloat(socket, property string) (float64, error) {
	type request struct {
		Command   []interface{} `json:"command"`
		RequestID int           `json:"request_id"`
	}
	type response struct {
		Data      float64 `json:"data"`
		Error     string  `json:"error"`
		RequestID int     `json:"request_id"`
	}

	req, err := json.Marshal(request{
		Command:   []interface{}{"get_property", property},
		RequestID: 1,
	})
	if err != nil {
		return 0, err
	}

	conn, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err = conn.Write(append(req, '\n')); err != nil {
		return 0, err
	}

	dec := json.NewDecoder(conn)
	for {
		var resp response
		if err = dec.Decode(&resp); err != nil {
			return 0, err
		}
		if resp.RequestID == 1 {
			if resp.Error != "success" {
				return 0, fmt.Errorf("mpv: %s", resp.Error)
			}
			return resp.Data, nil
		}
	}
}
