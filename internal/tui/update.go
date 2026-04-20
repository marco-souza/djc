package tui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/youtube"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ── update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.helpModel.Width = m.width
		// 14 = modal border (2) + padding (2*3=6) + prompt label + margins
		m.addInput.Width = max(20, m.width-14)
		cfgW := max(20, max(60, m.width*2/3)-14)
		for i := range m.configInputs {
			m.configInputs[i].Width = cfgW
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd) // keep ticking always

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancelAllDownloads()
			return m, tea.Quit
		}
		switch m.mode {
		case modeAdd:
			return m.updateAdd(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeDelete:
			return m.updateDelete(msg)
		case modeConfig:
			return m.updateConfig(msg)
		default:
			return m.updateList(msg)
		}

	case refreshMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
		} else {
			m.songs = msg.songs
			m.clampCursor()
			if !m.startupDone {
				m.startupDone = true
				cmds = append(cmds, m.resumeQueuedDownloads()...)
			}
		}

	case metadataFetchedMsg:
		// Only act if we're still in the confirm modal (user may have pressed Esc).
		if m.mode != modeConfirm {
			break
		}
		if msg.err != nil {
			m.mode = modeAdd
			m.setError("Could not fetch info: " + msg.err.Error())
			return m, m.addInput.Focus()
		}
		m.confirmItems = msg.items
		m.confirmSel = make([]bool, len(msg.items))
		for i := range m.confirmSel {
			m.confirmSel[i] = true
		}
		m.confirmCursor = 0
		m.confirmOffset = 0

	case downloadsQueuedMsg:
		// Prepend songs to the list in reverse order so the first track ends up at top.
		for i := len(msg.songs) - 1; i >= 0; i-- {
			m.songs = append([]library.Song{msg.songs[i]}, m.songs...)
		}
		m.cursor, m.offset = 0, 0
		for i, song := range msg.songs {
			m.downloadQueue = append(m.downloadQueue, queuedDownload{Song: song, URL: msg.urls[i]})
		}
		// Start the first download immediately if nothing is already downloading.
		if len(m.cancels) == 0 && len(m.downloadQueue) > 0 {
			next := m.downloadQueue[0]
			m.downloadQueue = m.downloadQueue[1:]
			cmds = append(cmds, startQueuedDownloadCmd(m.repo, m.cfg, next))
		}
		n := len(msg.songs)
		word := "song"
		if n != 1 {
			word = "songs"
		}
		m.setStatus(fmt.Sprintf("%d %s added to queue", n, word), false)

	case downloadStartedMsg:
		m.cancels[msg.Song.ID] = msg.cancel
		if msg.reDownload {
			// Update the existing row in-place instead of prepending a duplicate.
			if idx := slices.IndexFunc(m.songs, func(s library.Song) bool { return s.ID == msg.Song.ID }); idx >= 0 {
				m.songs[idx].Status = "downloading"
				m.songs[idx].Progress = 0
			}
		} else {
			m.songs = append([]library.Song{msg.Song}, m.songs...)
			m.cursor, m.offset = 0, 0
		}
		m.setStatus("Downloading…", false)
		cmds = append(cmds, waitForDownloadUpdate(msg.ch))

	case downloadUpdateMsg:
		if !msg.ok {
			cmds = append(cmds, refreshSongsCmd(m.repo))
			// Start the next queued download if the download pool is now idle.
			if len(m.cancels) == 0 && len(m.downloadQueue) > 0 {
				next := m.downloadQueue[0]
				m.downloadQueue = m.downloadQueue[1:]
				cmds = append(cmds, startQueuedDownloadCmd(m.repo, m.cfg, next))
			}
			break
		}
		m.applyDownloadEvent(msg.event)
		if msg.event.Completed || msg.event.Err != nil {
			delete(m.cancels, msg.event.SongID)
		}
		cmds = append(cmds, waitForDownloadUpdate(msg.ch))

	case actionDoneMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
		} else {
			m.setStatus(msg.message, false)
			cmds = append(cmds, refreshSongsCmd(m.repo))
		}

	case playbackStartedMsg:
		m.playerProc = msg.proc
		m.playerSongID = msg.songID
		m.playerPaused = false
		m.playerIPC = msg.ipc
		m.playerElapsed = 0
		m.playerDuration = 0
		m.setStatus("▶ "+msg.name, false)
		cmds = append(cmds, waitForPlaybackEndCmd(msg.songID, msg.proc), playerTickCmd())
		if msg.ipc != "" {
			cmds = append(cmds, getMpvDurationCmd(msg.ipc))
		}

	case playbackEndedMsg:
		if m.playerSongID == msg.songID {
			if m.playerIPC != "" {
				_ = os.Remove(m.playerIPC)
			}
			m.playerProc = nil
			m.playerSongID = 0
			m.playerPaused = false
			m.playerIPC = ""
			m.playerElapsed = 0
			m.playerDuration = 0
			m.setStatus("", false)
		}

	case playerTickMsg:
		if m.playerSongID != 0 {
			if !m.playerPaused {
				m.playerElapsed += time.Second
				if m.playerDuration > 0 && m.playerElapsed > m.playerDuration {
					m.playerElapsed = m.playerDuration
				}
			}
			cmds = append(cmds, playerTickCmd())
		}

	case mpvDurationMsg:
		if msg.duration > 0 {
			m.playerDuration = time.Duration(msg.duration * float64(time.Second))
		}
	}

	return m, tea.Batch(cmds...)
}

// updateList handles keys in normal list mode.
func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// vim dd: second 'd' press triggers delete
	if m.pendingD {
		m.pendingD = false
		m.setStatus("", false)
		if msg.String() == "d" && len(m.songs) > 0 {
			m.mode = modeDelete
			m.deleteConf = false
			return m, nil
		}
		// not a second 'd' — fall through and process the key normally
	}

	switch msg.String() {
	case "q":
		m.cancelAllDownloads()
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.songs)-1 {
			m.cursor++
			m.ensureVisible()
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}

	case "g":
		m.cursor, m.offset = 0, 0

	case "G":
		if n := len(m.songs); n > 0 {
			m.cursor = n - 1
			lh := m.listHeight()
			if n > lh {
				m.offset = n - lh
			} else {
				m.offset = 0
			}
		}

	case "a":
		m.mode = modeAdd
		m.addInput.SetValue("")
		return m, m.addInput.Focus()

	case "c":
		vals := [4]string{m.cfg.DownloadDir, m.cfg.AudioFormat, m.cfg.AudioQuality, m.cfg.OutputTemplate}
		for i := range m.configInputs {
			m.configInputs[i].SetValue(vals[i])
			m.configInputs[i].Blur()
		}
		m.configFocus = 0
		m.mode = modeConfig
		return m, m.configInputs[0].Focus()

	case "d":
		if len(m.songs) > 0 {
			m.pendingD = true
			m.setStatus("dd  →  delete selected song", false)
		}

	case "e":
		if len(m.songs) > 0 {
			return m, exportSongCmd(m.songs[m.cursor])
		}

	case "r":
		if len(m.songs) > 0 {
			song := m.songs[m.cursor]
			if _, downloading := m.cancels[song.ID]; downloading {
				m.setStatus("Download already in progress", false)
				break
			}
			m.setStatus("Refreshing…", false)
			return m, refreshSelectedSongCmd(m.repo, m.cfg, song)
		}

	case " ", "f8":
		if len(m.songs) == 0 {
			break
		}
		song := m.songs[m.cursor]
		if song.FilePath == "" || song.Status != "downloaded" {
			m.setError("Song not downloaded yet")
			break
		}
		// Same song playing: toggle pause/resume.
		if m.playerProc != nil && m.playerSongID == song.ID {
			if m.playerPaused {
				_ = m.playerProc.Process.Signal(syscall.SIGCONT)
				m.playerPaused = false
				m.setStatus("▶ "+song.Name, false)
			} else {
				_ = m.playerProc.Process.Signal(syscall.SIGSTOP)
				m.playerPaused = true
				m.setStatus("⏸ "+song.Name, false)
			}
			break
		}
		// Different or no song: stop current and start the new one.
		m.stopPlayer()
		return m, playSongCmd(song)

	case "[", "f7":
		if m.playerSongID != 0 {
			if m.playerIPC != "" {
				return m, mpvSeekCmd(m.playerIPC, -10)
			}
			if m.playerElapsed > 10*time.Second {
				m.playerElapsed -= 10 * time.Second
			} else {
				m.playerElapsed = 0
			}
		}

	case "]", "f9":
		if m.playerSongID != 0 {
			if m.playerIPC != "" {
				return m, mpvSeekCmd(m.playerIPC, 10)
			}
			m.playerElapsed += 10 * time.Second
			if m.playerDuration > 0 && m.playerElapsed > m.playerDuration {
				m.playerElapsed = m.playerDuration
			}
		}

	case "-":
		if m.playerSongID != 0 {
			if m.playerVolume > 0 {
				m.playerVolume -= 10
			}
			if m.playerIPC != "" {
				return m, mpvVolumeCmd(m.playerIPC, m.playerVolume)
			}
		}

	case "=":
		if m.playerSongID != 0 {
			if m.playerVolume < 100 {
				m.playerVolume += 10
			}
			if m.playerIPC != "" {
				return m, mpvVolumeCmd(m.playerIPC, m.playerVolume)
			}
		}

	case "esc":
		m.setStatus("", false)
	}

	return m, nil
}

// updateAdd handles keys inside the "add song" modal.
func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.addInput.Blur()
		return m, tea.ClearScreen
	case "enter":
		url := strings.TrimSpace(m.addInput.Value())
		if url == "" {
			m.setError("URL cannot be empty")
			return m, nil
		}
		m.addInput.Blur()
		m.mode = modeConfirm
		m.confirmURL = url
		m.confirmItems = nil // nil == loading
		m.confirmSel = nil
		m.confirmCursor = 0
		m.confirmOffset = 0
		return m, fetchMetadataCmd(url)
	}

	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

// updateConfirm handles keys inside the confirmation / playlist-select modal.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.confirmItems)

	switch msg.String() {
	case "esc":
		// Go back to the add modal with the URL still in the input.
		m.mode = modeAdd
		m.confirmItems = nil
		m.confirmSel = nil
		return m, m.addInput.Focus()

	case "j", "down":
		if n > 0 && m.confirmCursor < n-1 {
			m.confirmCursor++
			if m.confirmCursor >= m.confirmOffset+confirmModalMaxItems {
				m.confirmOffset = m.confirmCursor - confirmModalMaxItems + 1
			}
		}

	case "k", "up":
		if n > 0 && m.confirmCursor > 0 {
			m.confirmCursor--
			if m.confirmCursor < m.confirmOffset {
				m.confirmOffset = m.confirmCursor
			}
		}

	case " ":
		if n > 0 && m.confirmCursor < len(m.confirmSel) {
			m.confirmSel[m.confirmCursor] = !m.confirmSel[m.confirmCursor]
		}

	case "a":
		for i := range m.confirmSel {
			m.confirmSel[i] = true
		}

	case "n":
		for i := range m.confirmSel {
			m.confirmSel[i] = false
		}

	case "enter":
		if n == 0 {
			// Still loading — ignore.
			return m, nil
		}
		var selected []youtube.VideoInfo
		for i, item := range m.confirmItems {
			if i >= len(m.confirmSel) || m.confirmSel[i] {
				selected = append(selected, item)
			}
		}
		if len(selected) == 0 {
			m.setError("No songs selected")
			return m, nil
		}
		m.mode = modeList
		m.confirmItems = nil
		m.confirmSel = nil
		return m, tea.Batch(tea.ClearScreen, createQueuedDownloadsCmd(m.repo, m.cfg, selected))
	}

	return m, nil
}

// updateDelete handles keys inside the delete confirmation modal.
func (m Model) updateDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "q":
		m.mode = modeList
		return m, nil

	case "h", "left", "tab":
		m.deleteConf = !m.deleteConf

	case "l", "right":
		m.deleteConf = !m.deleteConf

	case "y":
		// instant yes
		if len(m.songs) > 0 {
			song := m.songs[m.cursor]
			m.mode = modeList
			return m, deleteSongCmd(m.repo, song)
		}
		m.mode = modeList

	case "enter":
		if m.deleteConf {
			if len(m.songs) > 0 {
				song := m.songs[m.cursor]
				m.mode = modeList
				return m, deleteSongCmd(m.repo, song)
			}
		}
		m.mode = modeList
	}

	return m, nil
}

// updateConfig handles keys inside the config editor modal.
func (m Model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.configInputs)
	switch msg.String() {
	case "esc":
		for i := range m.configInputs {
			m.configInputs[i].Blur()
		}
		m.mode = modeList
		return m, nil

	case "tab", "j", "down":
		m.configInputs[m.configFocus].Blur()
		m.configFocus = (m.configFocus + 1) % n
		return m, m.configInputs[m.configFocus].Focus()

	case "shift+tab", "k", "up":
		m.configInputs[m.configFocus].Blur()
		m.configFocus = (m.configFocus + n - 1) % n
		return m, m.configInputs[m.configFocus].Focus()

	case "ctrl+s":
		for i := range m.configInputs {
			m.configInputs[i].Blur()
		}
		m.mode = modeList
		return m, saveConfigCmd(m.cfg, m.configInputs)

	case "enter":
		if m.configFocus == n-1 {
			for i := range m.configInputs {
				m.configInputs[i].Blur()
			}
			m.mode = modeList
			return m, saveConfigCmd(m.cfg, m.configInputs)
		}
		m.configInputs[m.configFocus].Blur()
		m.configFocus++
		return m, m.configInputs[m.configFocus].Focus()
	}

	var cmd tea.Cmd
	m.configInputs[m.configFocus], cmd = m.configInputs[m.configFocus].Update(msg)
	return m, cmd
}

// ── layout helpers ───────────────────────────────────────────────────────────

// listHeight returns the number of visible song rows.
//
//	total = title(1) + playerBar(1) + sep(1) + header(1) + list(lh) + sep(1) + details(detailRows) + sep(1) + status(1) + help(1)
//	      = 8 + detailRows + lh
//	=> lh = height - 8 - detailRows
//
// fixedRows = title + playerBar + 3×sep + header + status + help = 8
const fixedRows = 8

func (m Model) listHeight() int {
	lh := m.height - fixedRows - detailRows
	if lh < 1 {
		return 1
	}
	return lh
}

func (m *Model) ensureVisible() {
	lh := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+lh {
		m.offset = m.cursor - lh + 1
	}
}

func (m *Model) clampCursor() {
	if len(m.songs) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	if m.cursor >= len(m.songs) {
		m.cursor = len(m.songs) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

func (m *Model) setError(msg string) { m.statusMsg = msg; m.statusError = true }
func (m *Model) setStatus(msg string, isErr bool) {
	m.statusMsg = msg
	m.statusError = isErr
}

func (m *Model) applyDownloadEvent(event downloadEvent) {
	if event.IsNew {
		// Prepend the new playlist-track song to the list so the user can see it.
		m.songs = append([]library.Song{event.NewSong}, m.songs...)
		return
	}

	if idx := slices.IndexFunc(m.songs, func(s library.Song) bool {
		return s.ID == event.SongID
	}); idx >= 0 {
		s := m.songs[idx]
		if event.Name != "" {
			s.Name = event.Name
		}
		if event.Format != "" {
			s.Format = event.Format
		}
		s.Progress = event.Progress
		s.Status = event.Status
		if event.FilePath != "" {
			s.FilePath = event.FilePath
		}
		m.songs[idx] = s
	}

	if event.Err != nil {
		m.setError(event.Err.Error())
		return
	}
	if event.Completed {
		m.setStatus("Download complete!", false)
	}
}

func (m *Model) cancelAllDownloads() {
	for id, cancel := range m.cancels {
		cancel()
		delete(m.cancels, id)
	}
	m.stopPlayer()
}

// resumeQueuedDownloads is called once at startup.  It enqueues any songs that
// were left in "queued" status (or were mid-download when the app was last
// closed) so they are resumed in insertion order (oldest first).
func (m *Model) resumeQueuedDownloads() []tea.Cmd {
// m.songs is ordered newest-first; iterate in reverse to get oldest-first.
var toResume []library.Song
for i := len(m.songs) - 1; i >= 0; i-- {
s := m.songs[i]
if s.Status == "queued" || s.Status == "downloading" {
toResume = append(toResume, s)
}
}
if len(toResume) == 0 {
return nil
}

// Reset any interrupted "downloading" entries to "queued" in-memory so
// the list shows the correct state before the download restarts.
for i := range m.songs {
if m.songs[i].Status == "downloading" {
m.songs[i].Status = "queued"
m.songs[i].Progress = 0
}
}

for _, song := range toResume {
song.Status = "queued"
m.downloadQueue = append(m.downloadQueue, queuedDownload{Song: song, URL: song.SourceURL})
}

var cmds []tea.Cmd
// Kick off the first download if nothing is already in-flight.
if len(m.cancels) == 0 && len(m.downloadQueue) > 0 {
next := m.downloadQueue[0]
m.downloadQueue = m.downloadQueue[1:]
cmds = append(cmds, startQueuedDownloadCmd(m.repo, m.cfg, next))
}
return cmds
}


func (m *Model) stopPlayer() {
	if m.playerProc != nil && m.playerProc.Process != nil {
		// Resume before killing so the player can shut down from a running state.
		if m.playerPaused {
			_ = m.playerProc.Process.Signal(syscall.SIGCONT)
		}
		_ = m.playerProc.Process.Kill()
	}
	if m.playerIPC != "" {
		_ = os.Remove(m.playerIPC)
		m.playerIPC = ""
	}
	m.playerProc = nil
	m.playerSongID = 0
	m.playerPaused = false
	m.playerElapsed = 0
	m.playerDuration = 0
}
