package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"marco-souza/djc/internal/config"
	"marco-souza/djc/internal/library"
	"marco-souza/djc/internal/youtube"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── colour palette ──────────────────────────────────────────────────────────

const (
	clrAccent   = lipgloss.Color("205") // hot-pink
	clrBlue     = lipgloss.Color("69")  // cornflower
	clrGreen    = lipgloss.Color("42")  // emerald
	clrRed      = lipgloss.Color("196") // red
	clrYellow   = lipgloss.Color("220") // amber
	clrMuted    = lipgloss.Color("240") // dark-grey
	clrSubtle   = lipgloss.Color("250") // light-grey
	clrBright   = lipgloss.Color("255") // white
	clrHiliteBg = lipgloss.Color("62")  // blue selection bg
	clrRedBg    = lipgloss.Color("88")  // dark-red for delete confirm
)

// ── styles ──────────────────────────────────────────────────────────────────

var (
	sTitle = lipgloss.NewStyle().Bold(true).Foreground(clrAccent)

	sSep = lipgloss.NewStyle().Foreground(clrMuted)

	sColHdr = lipgloss.NewStyle().Bold(true).Foreground(clrBlue)

	sSel = lipgloss.NewStyle().
		Foreground(clrBright).
		Background(clrHiliteBg).
		Bold(true)

	sNormal = lipgloss.NewStyle().Foreground(clrSubtle)
	sDling  = lipgloss.NewStyle().Foreground(clrYellow)
	sDone   = lipgloss.NewStyle().Foreground(clrGreen)
	sFailed = lipgloss.NewStyle().Foreground(clrRed)
	sMuted  = lipgloss.NewStyle().Foreground(clrMuted)
	sBright = lipgloss.NewStyle().Foreground(clrBright).Bold(true)
	sGreen  = lipgloss.NewStyle().Foreground(clrGreen)
	sRed    = lipgloss.NewStyle().Foreground(clrRed)
	sKey    = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)

	sDLabelSt = lipgloss.NewStyle().Bold(true).Foreground(clrBlue)
	sDValueSt = lipgloss.NewStyle().Foreground(clrSubtle)

	sModal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrAccent).
		Padding(1, 3)

	sDelModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrRed).
			Padding(1, 3)

	sBtnActive = lipgloss.NewStyle().Bold(true).Padding(0, 2)
	sBtnNormal = lipgloss.NewStyle().Foreground(clrMuted).Padding(0, 2)
)

// ── fixed column widths (name is dynamic) ───────────────────────────────────

const (
	colFmt    = 6
	colStatus = 22
	colDate   = 12
	// row = cursor(2) + name(dynamic) + sp(1) + fmt + sp(1) + status + sp(1) + date
	fixedWidth = 2 + 1 + colFmt + 1 + colStatus + 1 + colDate // = 44
	detailRows = 5                                            // content lines in the details panel
)

// ── mode ────────────────────────────────────────────────────────────────────

type Mode int

const (
	modeList Mode = iota
	modeAdd
	modeDelete
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

// ── model ───────────────────────────────────────────────────────────────────

type Model struct {
	repo *library.Repository
	cfg  *config.Config

	songs  []library.Song
	cursor int // active row index
	offset int // first rendered row index (scroll)
	mode   Mode

	// vim-style dd
	pendingD bool

	// add modal
	addInput textinput.Model

	// delete confirmation: true = "Delete" button selected, false = "Cancel"
	deleteConf bool

	// cancels for in-progress downloads
	cancels map[int64]context.CancelFunc

	// UI components
	spinner spinner.Model

	// terminal dimensions
	width, height int
	ready         bool

	// status bar
	statusMsg   string
	statusError bool
}

// ── constructor ─────────────────────────────────────────────────────────────

func NewModel(repo *library.Repository, cfg *config.Config) Model {
	inp := textinput.New()
	inp.Placeholder = "https://www.youtube.com/watch?v=..."
	inp.CharLimit = 1024
	inp.PromptStyle = lipgloss.NewStyle().Foreground(clrAccent)
	inp.TextStyle = lipgloss.NewStyle().Foreground(clrBright)

	sp := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(clrYellow)),
	)

	return Model{
		repo:     repo,
		cfg:      cfg,
		addInput: inp,
		spinner:  sp,
		cancels:  map[int64]context.CancelFunc{},
	}
}

// ── init ────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(refreshSongsCmd(m.repo), m.spinner.Tick)
}

// ── update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.addInput.Width = max(20, m.width-14)

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
		case modeDelete:
			return m.updateDelete(msg)
		default:
			return m.updateList(msg)
		}

	case refreshMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
		} else {
			m.songs = msg.songs
			m.clampCursor()
		}

	case downloadStartedMsg:
		m.cancels[msg.Song.ID] = msg.cancel
		m.songs = append([]library.Song{msg.Song}, m.songs...)
		m.cursor, m.offset = 0, 0
		m.setStatus("Downloading…", false)
		cmds = append(cmds, waitForDownloadUpdate(msg.ch))

	case downloadUpdateMsg:
		if !msg.ok {
			cmds = append(cmds, refreshSongsCmd(m.repo))
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
		return m, refreshSongsCmd(m.repo)

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

// ── view ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "Loading…\n"
	}
	switch m.mode {
	case modeAdd:
		return m.viewAdd()
	case modeDelete:
		return m.viewDelete()
	default:
		return m.viewMain()
	}
}

// viewMain renders the full-page list + details split.
//
// Layout (each line):
//
//	1        title bar
//	2        ─── separator
//	3        table column headers
//	4..4+lh  song rows  (lh = listHeight())
//	+1       ─── separator
//	+5       detail panel  (detailRows content lines)
//	+1       ─── separator
//	+1       status line
//	+1       help bar
func (m Model) viewMain() string {
	w := m.width

	sep := sSep.Render(strings.Repeat("─", w))

	// 1. Title
	title := sTitle.Render("🎧  DJ Companion")

	// 2. Table header
	header := m.renderHeader(w)

	// 3. Song list rows (exactly listHeight lines)
	list := m.renderList(w)

	// 4. Detail panel
	details := m.renderDetails(w)

	// 5. Status
	status := m.renderStatus(w)

	// 6. Help bar
	help := m.renderHelp()

	parts := []string{
		title,
		sep,
		header,
		list,
		sep,
		details,
		sep,
		status,
		help,
	}
	return strings.Join(parts, "\n")
}

// viewAdd renders the full screen with a centered "Add Song" modal.
func (m Model) viewAdd() string {
	title := sTitle.Render("  Add New Song")
	prompt := sMuted.Render("  YouTube URL")
	input := "  " + m.addInput.View()
	hint := sMuted.Render("  enter  confirm  •  esc  cancel")

	content := strings.Join([]string{title, "", prompt, input, "", hint}, "\n")
	box := sModal.Width(max(60, m.width/2)).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}

// viewDelete renders a centered delete confirmation modal.
func (m Model) viewDelete() string {
	songName := "unknown"
	if len(m.songs) > 0 {
		songName = m.songs[m.cursor].Name
	}
	name := truncate(songName, 42)

	title := sRed.Bold(true).Render("  Delete Song?")
	nameStr := lipgloss.NewStyle().Italic(true).Foreground(clrSubtle).Render(`  "` + name + `"`)
	warning := sMuted.Render("  This will also remove the file from disk.")

	var btnDel, btnCancel string
	if m.deleteConf {
		btnDel = sBtnActive.Copy().Background(clrRedBg).Foreground(clrBright).Render("  Delete  ")
		btnCancel = sBtnNormal.Render("  Cancel  ")
	} else {
		btnDel = sBtnNormal.Render("  Delete  ")
		btnCancel = sBtnActive.Copy().Background(clrHiliteBg).Foreground(clrBright).Render("  Cancel  ")
	}
	buttons := "  " + btnDel + "   " + btnCancel

	hint := sMuted.Render("  h/l  toggle  •  enter  confirm  •  esc / n  cancel")
	yHint := sMuted.Render("  y  delete immediately")

	content := strings.Join([]string{title, "", nameStr, warning, "", buttons, "", hint, yHint}, "\n")
	box := sDelModal.Width(max(60, m.width/2)).Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(clrMuted),
	)
}

// ── rendering helpers ───────────────────────────────────────────────────────

func (m Model) nameWidth() int {
	n := m.width - fixedWidth
	if n < 8 {
		return 8
	}
	return n
}

func (m Model) renderHeader(w int) string {
	nw := m.nameWidth()
	h := fmt.Sprintf("  %-*s %-*s %-*s %-*s",
		nw, "SONG",
		colFmt, "FMT",
		colStatus, "STATUS",
		colDate, "DATE",
	)
	return sColHdr.Render(truncatePlain(h, w))
}

func (m Model) renderList(w int) string {
	lh := m.listHeight()

	if len(m.songs) == 0 {
		emptyMsg := sMuted.Render("  No songs yet — press a to add one")
		lines := make([]string, lh)
		lines[0] = emptyMsg
		for i := 1; i < lh; i++ {
			lines[i] = ""
		}
		return strings.Join(lines, "\n")
	}

	rows := make([]string, lh)
	for i := 0; i < lh; i++ {
		idx := m.offset + i
		if idx < len(m.songs) {
			rows[i] = m.renderRow(idx, w)
		} else {
			rows[i] = ""
		}
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderRow(idx, w int) string {
	song := m.songs[idx]
	selected := idx == m.cursor
	nw := m.nameWidth()

	cursor := "  "
	if selected {
		cursor = "▶ "
	}

	name := truncate(song.Name, nw)
	fmtStr := truncate(strings.ToLower(song.Format), colFmt)
	statusStr := m.rowStatus(song, colStatus)
	dateStr := song.CreatedAt.Local().Format("Jan 02 15:04")

	line := fmt.Sprintf("%s%-*s %-*s %-*s %-*s",
		cursor,
		nw, name,
		colFmt, fmtStr,
		colStatus, statusStr,
		colDate, dateStr,
	)
	line = truncatePlain(line, w)

	if selected {
		return sSel.Width(w).Render(line)
	}
	switch {
	case strings.HasPrefix(song.Status, "failed"):
		return sFailed.Render(line)
	case song.Status == "downloading":
		return sDling.Render(line)
	default:
		return sNormal.Render(line)
	}
}

func (m Model) rowStatus(song library.Song, width int) string {
	switch {
	case song.Status == "downloading":
		bar := progressBar(song.Progress, 8)
		pct := fmt.Sprintf("%3d%%", song.Progress)
		s := m.spinner.View() + " " + bar + " " + pct
		return truncate(s, width)
	case song.Status == "downloaded":
		return truncate("✓ downloaded", width)
	case strings.HasPrefix(song.Status, "failed"):
		return truncate("✗ "+strings.TrimPrefix(song.Status, "failed: "), width)
	default:
		return truncate(song.Status, width)
	}
}

func (m Model) renderDetails(w int) string {
	if len(m.songs) == 0 {
		lines := make([]string, detailRows)
		lines[0] = sMuted.Render("  Select a song to see details")
		for i := 1; i < detailRows; i++ {
			lines[i] = ""
		}
		return strings.Join(lines, "\n")
	}

	song := m.songs[m.cursor]
	nw := w - 12

	label := func(s string) string {
		return sDLabelSt.Width(8).Render(s + ":")
	}
	val := func(s string) string {
		return sDValueSt.Render(s)
	}

	titleLine := "  " + sBright.Render(truncate(song.Name, w-4))
	fmtDate := "  " + label("Format") + val(strings.ToUpper(song.Format)) +
		"   " + label("Added") + val(song.CreatedAt.Local().Format("Mon Jan 02, 2006 15:04"))
	statusLine := "  " + label("Status") + val(formatSongStatus(song))
	pathLine := "  " + label("Path") + val(truncate(song.FilePath, nw))
	srcLine := "  " + label("Source") + val(truncate(song.SourceURL, nw))

	return strings.Join([]string{titleLine, fmtDate, statusLine, pathLine, srcLine}, "\n")
}

func (m Model) renderStatus(w int) string {
	if m.statusMsg == "" {
		return ""
	}
	if m.statusError {
		return sRed.Render(" " + m.statusMsg)
	}
	return sGreen.Render(" " + m.statusMsg)
}

func (m Model) renderHelp() string {
	bind := func(k, desc string) string {
		return sKey.Render(k) + sMuted.Render(" "+desc)
	}
	sep := sMuted.Render("  │  ")
	parts := []string{
		bind("j/k", "move"),
		bind("g/G", "top/bottom"),
		bind("a", "add"),
		bind("dd", "delete"),
		bind("e", "→mp3"),
		bind("r", "reload"),
		bind("q", "quit"),
	}
	return " " + strings.Join(parts, sep)
}

// ── layout helpers ───────────────────────────────────────────────────────────

// listHeight returns the number of visible song rows.
//
//	total = title(1) + sep(1) + header(1) + list(lh) + sep(1) + details(detailRows) + sep(1) + status(1) + help(1)
//	      = 7 + detailRows + lh
//	=> lh = height - 7 - detailRows
func (m Model) listHeight() int {
	lh := m.height - 7 - detailRows
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
}

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

// ── pure helpers ─────────────────────────────────────────────────────────────

func progressBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct * width) / 100
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func formatSongStatus(song library.Song) string {
	switch {
	case song.Status == "downloading":
		return fmt.Sprintf("Downloading… %d%%", song.Progress)
	case song.Status == "downloaded":
		return "Downloaded"
	default:
		return song.Status
	}
}

// truncate shortens s to at most limit runes, appending "…" if trimmed.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

// truncatePlain truncates a plain (possibly ANSI-free) byte string by byte position.
// Used only for lines where we know there are no escape codes.
func truncatePlain(s string, limit int) string {
	if len([]rune(s)) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
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
