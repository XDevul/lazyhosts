# lazyhosts

A terminal TUI for managing `/etc/hosts` profiles via [hostctl](https://github.com/guumaster/hostctl). Keyboard-driven, fast, inspired by lazygit.

![Go](https://img.shields.io/badge/Go-1.21+-blue)

## Features

- Two-pane layout: profile list + detail/preview panel
- Enable/disable hostctl profiles with confirmation dialog
- Fuzzy search/filter profiles with `/`
- Live `/etc/hosts` preview
- Sudo status detection
- Keyboard-first UX (vim-style navigation)

## Prerequisites

```bash
# Install hostctl
brew install guumaster/tap/hostctl

# Ensure sudo is available for profile switching
sudo -v
```

## Build & Run

```bash
git clone https://github.com/jr/lazyhosts.git
cd lazyhosts
go build -o lazyhosts .
./lazyhosts
```

Or run directly:

```bash
go run .
```

## Keybindings

| Key       | Action                  |
|-----------|-------------------------|
| `↑` / `k` | Move up                |
| `↓` / `j` | Move down              |
| `Enter`   | Enable selected profile |
| `d`       | Disable selected profile|
| `r`       | Reload profile list     |
| `/`       | Search / filter         |
| `Esc`     | Cancel search / dialog  |
| `?`       | Toggle help popup       |
| `q`       | Quit                    |

## Project Structure

```
lazyhosts/
├── main.go                    # Entry point
├── internal/
│   ├── hostctl/hostctl.go     # hostctl command execution
│   ├── model/model.go         # Bubble Tea model (Update/View)
│   ├── state/state.go         # Application state
│   └── ui/
│       ├── keys.go            # Key bindings
│       ├── render.go          # UI rendering
│       └── styles.go          # Lipgloss styles
```

## License

MIT
