package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func startDownloadCmd(repo *library.Repository, cfg *config.Config, url string) tea.Cmd {
	return func() tea.Msg {
		song, err := repo.CreateSong(url, cfg.AudioFormat)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ch := make(chan downloadEvent)
		ctx, cancel := context.WithCancel(context.Background())
		go downloadSong(ctx, repo, cfg, song, url, ch)
		return downloadStartedMsg{Song: song, ch: ch, cancel: cancel}
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

	var latestFilePath, latestName string
	var updateErrReported bool

	_, err := youtube.DownloadAudioWithProgress(ctx, url, cfg.AudioFormat, nil, cfg,
		func(progress youtube.DownloadProgress) {
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
		errStatus := fmt.Sprintf("failed: %s", err.Error())
		if repoErr := repo.UpdateDownload(song.ID, pick(latestName, song.Name), latestFilePath, errStatus, 0); repoErr != nil {
			err = fmt.Errorf("%w (status save failed: %v)", err, repoErr)
		}
		ch <- downloadEvent{SongID: song.ID, Status: errStatus, Err: err}
		return
	}

	if latestName == "" {
		latestName = song.Name
	}

	if repoErr := repo.UpdateDownload(song.ID, latestName, latestFilePath, "downloaded", 100); repoErr != nil {
		ch <- downloadEvent{SongID: song.ID, Err: fmt.Errorf("mark song as downloaded: %w", repoErr)}
		return
	}

	ch <- downloadEvent{
		SongID:    song.ID,
		Name:      latestName,
		Format:    extensionOr(song.Format, latestFilePath),
		FilePath:  latestFilePath,
		Progress:  100,
		Status:    "downloaded",
		Completed: true,
	}
}

func saveConfigCmd(cfg *config.Config, inputs [4]textinput.Model) tea.Cmd {
	return func() tea.Msg {
		cfg.DownloadDir = strings.TrimSpace(inputs[0].Value())
		cfg.AudioFormat = strings.TrimSpace(inputs[1].Value())
		cfg.AudioQuality = strings.TrimSpace(inputs[2].Value())
		cfg.OutputTemplate = strings.TrimSpace(inputs[3].Value())
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
		player, args := findPlayer()
		if player == "" {
			return actionDoneMsg{err: fmt.Errorf("no audio player found (install ffplay or mpv)")}
		}
		args = append(args, song.FilePath)
		cmd := exec.Command(player, args...)
		devNull, err := os.Open(os.DevNull)
		if err == nil {
			cmd.Stdin = devNull
			cmd.Stdout = devNull
			cmd.Stderr = devNull
		}
		if err := cmd.Start(); err != nil {
			return actionDoneMsg{err: fmt.Errorf("start player: %w", err)}
		}
		return playbackStartedMsg{songID: song.ID, name: song.Name, proc: cmd}
	}
}

// waitForPlaybackEndCmd blocks until the player process exits, then notifies the TUI.
func waitForPlaybackEndCmd(songID int64, proc *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		_ = proc.Wait()
		return playbackEndedMsg{songID: songID}
	}
}

// findPlayer returns the first available audio player binary and its arguments.
func findPlayer() (string, []string) {
	if p, err := exec.LookPath("ffplay"); err == nil {
		return p, []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
	}
	if p, err := exec.LookPath("mpv"); err == nil {
		return p, []string{"--no-video", "--really-quiet"}
	}
	if p, err := exec.LookPath("afplay"); err == nil { // macOS
		return p, nil
	}
	if p, err := exec.LookPath("aplay"); err == nil { // Linux ALSA
		return p, nil
	}
	return "", nil
}
