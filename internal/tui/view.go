package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"marco-souza/djc/internal/library"

	"github.com/charmbracelet/lipgloss"
)

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
	case modeConfig:
		return m.viewConfig()
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

// viewConfig renders a centred config editor modal.
func (m Model) viewConfig() string {
	title := sTitle.Render("⚙  Configuration")

	var rows []string
	rows = append(rows, title, "")
	for i, inp := range m.configInputs {
		if i == m.configFocus {
			rows = append(rows, "  "+sDLabelSt.Render(configLabels[i]))
		} else {
			rows = append(rows, "  "+sMuted.Render(configLabels[i]))
		}
		rows = append(rows, "  "+inp.View(), "")
	}

	hint := sMuted.Render("  tab/j/k  navigate  •  ctrl+s / enter  save  •  esc  cancel")
	rows = append(rows, hint)

	content := strings.Join(rows, "\n")
	box := sCfgModal.Width(max(60, m.width*2/3)).Render(content)

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
	if n > maxNameWidth {
		return maxNameWidth
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
		return sFailed.Width(w).Render(line)
	case song.Status == "downloading":
		return sDling.Width(w).Render(line)
	default:
		return sNormal.Width(w).Render(line)
	}
}

func (m Model) rowStatus(song library.Song, width int) string {
	// Show playback state for the currently playing/paused song.
	if m.playerSongID == song.ID {
		if m.playerPaused {
			return truncate("⏸ paused", width)
		}
		return truncate("▶ playing", width)
	}
	switch {
	case song.Status == "downloading":
		bar := m.downloadProgress.ViewAs(float64(song.Progress) / 100)
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
	pad := lipgloss.NewStyle().Width(w)

	if len(m.songs) == 0 {
		lines := make([]string, detailRows)
		lines[0] = pad.Render(sMuted.Render("  Select a song to see details"))
		for i := 1; i < detailRows; i++ {
			lines[i] = pad.Render("")
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

	titleLine := pad.Render("  " + sBright.Render(truncate(song.Name, w-4)))
	fmtDate := pad.Render("  " + label("Format") + val(strings.ToUpper(song.Format)) +
		"   " + label("Added") + val(song.CreatedAt.Local().Format("Mon Jan 02, 2006 15:04")))
	statusLine := pad.Render("  " + label("Status") + val(formatSongStatus(song)))
	pathLine := pad.Render("  " + label("Path") + val(truncate(song.FilePath, nw)))
	srcLine := pad.Render("  " + label("Source") + val(truncate(song.SourceURL, nw)))

	return strings.Join([]string{titleLine, fmtDate, statusLine, pathLine, srcLine}, "\n")
}

func (m Model) renderStatus(w int) string {
	pad := lipgloss.NewStyle().Width(w)
	if m.statusMsg == "" {
		return pad.Render("")
	}
	if m.statusError {
		return pad.Render(sRed.Render(" " + m.statusMsg))
	}
	return pad.Render(sGreen.Render(" " + m.statusMsg))
}

func (m Model) renderHelp() string {
	return " " + m.helpModel.View(defaultKeys)
}

// ── pure helpers ─────────────────────────────────────────────────────────────

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
