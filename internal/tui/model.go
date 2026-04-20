package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/youtube"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	selectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62"))
	mutedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

const (
	colNameWidth   = 36
	colFormatWidth = 8
	colStatusWidth = 20
	colDateWidth   = 16
)

type Mode int

const (
	modeList Mode = iota
	modeAdd
	modeMenu
	modeInfo
)

type downloadEvent struct {
	SongID    int64
	Name      string
	Format    string
	FilePath  string
	Progress  int
	Status    string
	Completed bool
	Err       error
}

type downloadStartedMsg struct {
	Song   library.Song
	ch     <-chan downloadEvent
	cancel context.CancelFunc
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

type Model struct {
	repo *library.Repository
	cfg  *config.Config

	songs         []library.Song
	selected      int
	mode          Mode
	menuSelection int
	statusMessage string
	statusError   bool

	addInput textinput.Model
	cancels  map[int64]context.CancelFunc
}

func NewModel(repo *library.Repository, cfg *config.Config) Model {
	input := textinput.New()
	input.Placeholder = "Paste YouTube URL and press Enter"
	input.CharLimit = 1024
	input.Width = 80

	return Model{
		repo:          repo,
		cfg:           cfg,
		mode:          modeList,
		statusMessage: "Ready",
		menuSelection: 0,
		addInput:      input,
		statusError:   false,
		songs:         nil,
		selected:      0,
		cancels:       map[int64]context.CancelFunc{},
	}
}

func (m Model) Init() tea.Cmd {
	return refreshSongsCmd(m.repo)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancelAllDownloads()
			return m, tea.Quit
		}
		switch m.mode {
		case modeAdd:
			return m.updateAdd(msg)
		case modeMenu:
			return m.updateMenu(msg)
		case modeInfo:
			if msg.String() == "esc" || msg.String() == "enter" {
				m.mode = modeMenu
			}
			return m, nil
		default:
			return m.updateList(msg)
		}
	case refreshMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.songs = msg.songs
		m.clampSelected()
		return m, nil
	case downloadStartedMsg:
		m.cancels[msg.Song.ID] = msg.cancel
		m.songs = append([]library.Song{msg.Song}, m.songs...)
		m.selected = 0
		m.setStatus("Started download", false)
		return m, waitForDownloadUpdate(msg.ch)
	case downloadUpdateMsg:
		if !msg.ok {
			return m, refreshSongsCmd(m.repo)
		}
		m.applyDownloadEvent(msg.event)
		if msg.event.Completed || msg.event.Err != nil {
			delete(m.cancels, msg.event.SongID)
		}
		return m, waitForDownloadUpdate(msg.ch)
	case actionDoneMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.setStatus(msg.message, false)
		return m, refreshSongsCmd(m.repo)
	}

	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelAllDownloads()
		return m, tea.Quit
	case "down", "j":
		if m.selected < len(m.songs)-1 {
			m.selected++
		}
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "ctrl+a", "A":
		m.mode = modeAdd
		m.addInput.SetValue("")
		m.addInput.Focus()
	case "enter":
		if len(m.songs) > 0 {
			m.mode = modeMenu
			m.menuSelection = 0
		}
	case "r":
		return m, refreshSongsCmd(m.repo)
	}

	return m, nil
}

func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.addInput.Blur()
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.addInput.Value())
		if url == "" {
			m.setError("URL cannot be empty")
			return m, nil
		}
		m.mode = modeList
		m.addInput.Blur()
		return m, startDownloadCmd(m.repo, m.cfg, url)
	}

	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	actions := menuActions()
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "down", "j":
		if m.menuSelection < len(actions)-1 {
			m.menuSelection++
		}
	case "up", "k":
		if m.menuSelection > 0 {
			m.menuSelection--
		}
	case "enter":
		if len(m.songs) == 0 {
			m.mode = modeList
			return m, nil
		}
		song := m.songs[m.selected]
		switch actions[m.menuSelection] {
		case "Info":
			m.mode = modeInfo
			return m, nil
		case "Export as MP3":
			m.mode = modeList
			return m, exportSongCmd(song)
		case "Delete":
			m.mode = modeList
			return m, deleteSongCmd(m.repo, song)
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🎧 DJ Companion"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Ctrl+A (or A): add song • Enter: actions • q: quit"))
	b.WriteString("\n\n")

	if m.mode == modeAdd {
		b.WriteString(headerStyle.Render("Add a new song"))
		b.WriteString("\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Enter to start download • Esc to cancel"))
		b.WriteString("\n\n")
	}

	if len(m.songs) == 0 {
		b.WriteString("No songs downloaded yet. Press Ctrl+A to add one.\n")
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-*s %-*s %-*s %-*s", colNameWidth, "NAME", colFormatWidth, "FORMAT", colStatusWidth, "STATUS", colDateWidth, "DATE")))
		b.WriteString("\n")
		for i, song := range m.songs {
			name := truncate(song.Name, colNameWidth)
			line := fmt.Sprintf("%-*s %-*s %-*s %-*s", colNameWidth, name, colFormatWidth, strings.ToLower(song.Format), colStatusWidth, formatSongStatus(song), colDateWidth, song.CreatedAt.Local().Format("2006-01-02 15:04"))
			if i == m.selected && m.mode == modeList {
				line = selectedRowStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if m.mode == modeMenu && len(m.songs) > 0 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Actions"))
		b.WriteString("\n")
		for i, action := range menuActions() {
			line := action
			if action == "Delete" {
				line = errorStyle.Render(action)
			}
			if i == m.menuSelection {
				line = selectedRowStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString(mutedStyle.Render("Enter to confirm • Esc to close"))
		b.WriteString("\n")
	}

	if m.mode == modeInfo && len(m.songs) > 0 {
		song := m.songs[m.selected]
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Song info"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Date: %s\n", song.CreatedAt.Local().Format(time.RFC1123)))
		b.WriteString(fmt.Sprintf("Name: %s\n", song.Name))
		b.WriteString(fmt.Sprintf("File path: %s\n", song.FilePath))
		b.WriteString(mutedStyle.Render("Esc/Enter to go back"))
		b.WriteString("\n")
	}

	if m.statusMessage != "" {
		b.WriteString("\n")
		if m.statusError {
			b.WriteString(errorStyle.Render(m.statusMessage))
		} else {
			b.WriteString(successStyle.Render(m.statusMessage))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) setError(message string) {
	m.setStatus(message, true)
}

func (m *Model) setStatus(message string, isError bool) {
	m.statusMessage = message
	m.statusError = isError
}

func (m *Model) clampSelected() {
	if len(m.songs) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(m.songs) {
		m.selected = len(m.songs) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *Model) applyDownloadEvent(event downloadEvent) {
	if idx := slices.IndexFunc(m.songs, func(song library.Song) bool {
		return song.ID == event.SongID
	}); idx >= 0 {
		song := m.songs[idx]
		if event.Name != "" {
			song.Name = event.Name
		}
		if event.Format != "" {
			song.Format = event.Format
		}
		song.Progress = event.Progress
		song.Status = event.Status
		if event.FilePath != "" {
			song.FilePath = event.FilePath
		}
		m.songs[idx] = song
	}

	if event.Err != nil {
		m.setError(event.Err.Error())
		return
	}

	if event.Completed {
		m.setStatus("Download completed", false)
	}
}

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

func downloadSong(ctx context.Context, repo *library.Repository, cfg *config.Config, song library.Song, url string, ch chan<- downloadEvent) {
	defer close(ch)

	var latestFilePath string
	var latestName string

	_, err := youtube.DownloadAudioWithProgress(ctx, url, cfg.AudioFormat, nil, cfg, func(progress youtube.DownloadProgress) {
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

		_ = repo.UpdateDownload(song.ID, pick(progress.Name, song.Name), pick(progress.FilePath, latestFilePath), status, progress.Percent)
		ch <- downloadEvent{
			SongID:    song.ID,
			Name:      pick(progress.Name, song.Name),
			Format:    pick(progress.Format, song.Format),
			FilePath:  pick(progress.FilePath, latestFilePath),
			Progress:  progress.Percent,
			Status:    status,
			Completed: progress.Completed,
		}
	})
	if err != nil {
		errStatus := fmt.Sprintf("failed: %s", err.Error())
		_ = repo.UpdateDownload(song.ID, pick(latestName, song.Name), latestFilePath, errStatus, 0)
		ch <- downloadEvent{SongID: song.ID, Status: errStatus, Err: err}
		return
	}

	if latestName == "" {
		latestName = song.Name
	}

	_ = repo.UpdateDownload(song.ID, latestName, latestFilePath, "downloaded", 100)
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

func exportSongCmd(song library.Song) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(song.FilePath) == "" {
			return actionDoneMsg{err: fmt.Errorf("song has no file path")}
		}

		ext := strings.ToLower(filepath.Ext(song.FilePath))
		if ext == ".mp3" {
			return actionDoneMsg{message: "Song is already an MP3"}
		}

		outputPath := strings.TrimSuffix(song.FilePath, filepath.Ext(song.FilePath)) + ".mp3"
		cmd := exec.Command("ffmpeg", "-y", "-i", song.FilePath, outputPath)
		cmd.Env = os.Environ()

		if out, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return actionDoneMsg{err: fmt.Errorf("export mp3 failed: %s", msg)}
		}

		return actionDoneMsg{message: fmt.Sprintf("Exported MP3: %s", outputPath)}
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

func formatSongStatus(song library.Song) string {
	if song.Status == "downloading" {
		return fmt.Sprintf("downloading (%d%%)", song.Progress)
	}
	return song.Status
}

func menuActions() []string {
	return []string{"Info", "Export as MP3", "Delete"}
}

func truncate(s string, limit int) string {
	if len([]rune(s)) <= limit {
		return s
	}
	runes := []rune(s)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func pick(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func extensionOr(defaultValue, filePath string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
	if ext != "" {
		return ext
	}
	return defaultValue
}

func (m *Model) cancelAllDownloads() {
	for id, cancel := range m.cancels {
		if cancel != nil {
			cancel()
		}
		delete(m.cancels, id)
	}
}
