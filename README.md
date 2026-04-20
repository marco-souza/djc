# DJ Companion 🎧

Welcome to my DJ Companion! This is a CLI tool for helping with downloading music, managing playlists, and other DJ-related tasks. Whether you're a fellow DJ or just someone who loves music and coding, I hope you find something useful here.

## About the Project

This project is a harmonious blend of my passion for music and programming. As a DJ and a developer, I thrive on creating innovative tools that not only help manage and expand my music library but also enhance my playlists and elevate my performances.

With this CLI tool, I've combined utility and creativity to assist in downloading music, managing playlists, and exploring new avenues for DJ-related tasks. Here, you'll find a curated mix of scripts, tools, and experimental projects—all designed to make the DJ experience more seamless, enjoyable, and inspiring.

## Features

- **Custom Music Tools**: Scripts and applications to help organize, analyze, and manage my music collection.
- **Setup Enhancements**: Tools to optimize my DJ setup for seamless and high-quality performances.
- **Workflow Improvements**: Automations and utilities to streamline my DJ workflow and save time.
- **Experimental Projects**: Creative and innovative projects where I explore new ideas, technologies, and concepts.

## Getting Started

Feel free to explore the repository and see what I've been working on. If you're a fellow DJ or developer, you might find some of the tools useful or inspiring for your own projects.

1. **Clone the Repository**: 
   ```bash
   git clone https://github.com/marco-souza/djc.git
   ```
2. **Explore the Code**: Dive into the scripts and tools to see how they work.
3. **Contribute**: If you have ideas or improvements, feel free to fork the repo and submit a pull request!

## TUI

Run the terminal UI with:

```bash
djc tui
```

The TUI uses a **full-page split layout**: the top panel lists all downloaded songs and the
bottom panel shows the details of the currently selected song.

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `a` | Open **Add Song** modal (paste a YouTube URL) |
| `c` | Open **Config** modal (edit download directory, format, quality, output template) |
| `dd` | Open **Delete** confirmation modal (vim-style double key) |
| `e` | Export selected song to MP3 (requires `ffmpeg`) |
| `r` | Refresh/reconcile the selected song (locate/move the file or re-download it if needed) |
| `Space` / `F8` | Play / pause the selected song — F8 acts as a media key in most terminals |
| `[` / `F7` | Seek back 10 s (requires `mpv`) |
| `]` / `F9` | Seek forward 10 s (requires `mpv`) |
| `-` | Volume down 10 % (requires `mpv`) |
| `=` | Volume up 10 % (requires `mpv`) |
| `q` / `Ctrl+C` | Quit |

Inside the **Add Song** modal: `Enter` starts the download, `Esc` cancels.

Inside the **Delete** modal: `h`/`l` or `Tab` toggles the button, `Enter` confirms,
`y` deletes immediately, `n` or `Esc` cancels.

### Player bar

A persistent **player bar** sits just below the title and is always visible.  When a
track is playing it shows the song name, play/pause state, a live progress bar,
elapsed/total time (`m:ss / m:ss`), and the current volume level.

Seek and volume controls require **`mpv`** (preferred player when available).  If only
`ffplay`, `afplay`, or `aplay` is installed, play/pause still works but seek and volume
keys are no-ops.

**Media key integration** — `F7` / `F8` / `F9` are bound to seek-back / play-pause /
seek-forward.  Whether your physical media keys send these codes depends on your
terminal and OS key mapping (e.g. `xdotool key F8` on X11, or a terminal binding
like Kitty's `map XF86AudioPlay send_key F8`).

## Contact

I'm always open to feedback, collaboration, or just a chat about music and tech. Feel free to reach out to me at [marco@tremtec.com](mailto:marco@tremtec.com).

## License

This project is open-source and available under the [MIT License](LICENSE).

---

Thanks for stopping by! Keep the beats rolling and the code flowing! 🎶💻
