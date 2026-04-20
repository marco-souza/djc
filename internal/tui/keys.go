package tui

import (
"github.com/charmbracelet/bubbles/key"
)

// keyMap defines all key bindings and implements help.KeyMap so the bubbles
// help component can render them automatically.
type keyMap struct {
Move    key.Binding
Ends    key.Binding
Add     key.Binding
Delete  key.Binding
Export  key.Binding
Config  key.Binding
Refresh key.Binding
Play    key.Binding
Quit    key.Binding
}

// defaultKeys is the singleton key map used throughout the TUI.
var defaultKeys = keyMap{
Move: key.NewBinding(
key.WithKeys("j", "k", "up", "down"),
key.WithHelp("j/k", "move"),
),
Ends: key.NewBinding(
key.WithKeys("g", "G"),
key.WithHelp("g/G", "top/bottom"),
),
Add: key.NewBinding(
key.WithKeys("a"),
key.WithHelp("a", "add"),
),
Delete: key.NewBinding(
key.WithKeys("d"),
key.WithHelp("dd", "delete"),
),
Export: key.NewBinding(
key.WithKeys("e"),
key.WithHelp("e", "→mp3"),
),
Config: key.NewBinding(
key.WithKeys("c"),
key.WithHelp("c", "config"),
),
Refresh: key.NewBinding(
key.WithKeys("r"),
key.WithHelp("r", "refresh"),
),
Play: key.NewBinding(
key.WithKeys(" "),
key.WithHelp("spc", "play/pause"),
),
Quit: key.NewBinding(
key.WithKeys("q", "ctrl+c"),
key.WithHelp("q", "quit"),
),
}

// ShortHelp returns the compact one-line help bar bindings.
func (k keyMap) ShortHelp() []key.Binding {
return []key.Binding{k.Move, k.Ends, k.Add, k.Delete, k.Export, k.Config, k.Refresh, k.Play, k.Quit}
}

// FullHelp returns the expanded multi-row help view bindings.
func (k keyMap) FullHelp() [][]key.Binding {
return [][]key.Binding{k.ShortHelp()}
}
