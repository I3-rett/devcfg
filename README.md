# 🧭 devcfg — Linux/macOS Environment Configuration TUI

[![Release](https://img.shields.io/github/v/release/I3-rett/devcfg)](https://github.com/I3-rett/devcfg/releases)

`devcfg` is a CLI TUI written in Go that lets you **configure a Linux/macOS machine after an SSH connection**, without any built-in SSH logic.

Workflow:
1. Connect manually via SSH
2. Download and run `devcfg`
3. Follow the interactive TUI workflow
4. Configure your environment (tools, git, docker, shell…)
5. Everything runs locally on the remote machine

---

## 🚀 Quick Start

```bash
# Download the binary (replace with the correct asset for your platform)
curl -L https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-linux-amd64 -o devcfg
chmod +x devcfg
./devcfg
```

---

## 🎮 TUI Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `SPACE` | Toggle selection (checkbox / radio) |
| `ENTER` on item | Toggle selection |
| `ENTER` on Continue | Validate step and proceed |
| `Tab` / `Shift+Tab` | Navigate form fields (Git step) |
| `q` / `Ctrl+C` | Quit |

---

## 🪜 Workflow Steps

### Step 1 — Tools Installation
Interactive checklist of tools to install. Uses the system package manager (brew/apt) or a fallback script.

Available tools: `git`, `neovim`, `docker`, `nodejs`, `python3`, `curl`, `tmux`, `htop`, `ripgrep`, `fzf`, `zsh`, `starship`

```
Step 1/4 — Tools Installation

  [ ] git          Version control system
  [✓] neovim       Hyperextensible text editor
▶ [ ] docker       Container platform
  [ ] nodejs       JavaScript runtime
  ...

╭──────────────╮
│  Continue    │
╰──────────────╯
```

### Step 2 — Git Configuration
Form to set `git config --global` identity.

- `user.name`
- `user.email`
- GPG signing toggle (`commit.gpgsign`)

### Step 3 — Docker Setup
Automatic checks:
- Docker installation detected
- Docker daemon status (via `systemctl is-active docker`)
- User membership in the `docker` group (offers `sudo usermod -aG docker $USER`)

### Step 4 — Shell Setup
- View current shell (`$SHELL`)
- Option to switch to `zsh` or `bash` (via `chsh`)
- Option to inject basic aliases into `~/.zshrc` / `~/.bashrc`:
  `ll`, `la`, `gs`, `gp`, `gc`, `gco`, `..`, `...`

---

## 🏗️ Architecture

```
devcfg/
├── main.go                         Entry point
├── internal/
│   ├── system/detect.go            OS + package manager detection
│   ├── registry/
│   │   ├── registry.go             Tool registry (go:embed)
│   │   └── tools.json              Tool definitions (12 tools)
│   ├── resolver/resolver.go        brew / apt / fallback selection
│   ├── executor/executor.go        Command runner (stdout+stderr capture)
│   └── tui/
│       ├── app.go                  Root Bubble Tea model (step orchestrator)
│       ├── tuistyles/styles.go     Lipgloss theme (purple/teal)
│       └── steps/
│           ├── tools.go            Step 1 — Tools checklist
│           ├── git.go              Step 2 — Git config form
│           ├── docker.go           Step 3 — Docker checks
│           └── shell.go            Step 4 — Shell setup
└── .github/workflows/release.yml  CI/CD: build + publish release
```

### Layer Responsibilities

| Layer | Package | Role |
|-------|---------|------|
| **System** | `internal/system` | Detect OS (`macos`, `ubuntu`, `debian`, `linux`) and package manager (`brew`, `apt`, `none`) via `runtime.GOOS` and `/etc/os-release` |
| **Registry** | `internal/registry` | Load tool definitions from embedded `tools.json`; expose `List()` and `Find(name)` |
| **Resolver** | `internal/resolver` | Select install command: brew → apt → fallback script |
| **Executor** | `internal/executor` | Run arbitrary commands with `os/exec`, capture combined stdout+stderr |
| **TUI** | `internal/tui` | Multi-step Bubble Tea workflow with lipgloss styling |

---

## ⚙️ Tool Model (tools.json)

```json
{
  "name": "neovim",
  "description": "Hyperextensible text editor",
  "brew": "neovim",
  "apt":  "neovim",
  "fallback": ""
}
```

Each tool carries its own package name per package manager. No OS coupling in the tool definition itself.

---

## 🧠 Resolver Priority

```
brew available + brew package defined  →  brew install <pkg>
apt available  + apt package defined   →  sudo apt-get install -y <pkg>
fallback script defined                →  sh -c "<script>"
otherwise                              →  error: no install method
```

---

## 📦 Build from Source

```bash
git clone https://github.com/I3-rett/devcfg.git
cd devcfg
go build -o devcfg .
./devcfg
```

**Requirements:** Go 1.24+

**Dependencies:**
- [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) — TUI framework
- [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) — terminal styling
- [`github.com/charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) — text inputs

---

## 🛠️ Dev Setup (Contributing)

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- `git`

### Quickstart

```bash
# 1. Fork then clone the repo
git clone https://github.com/<your-fork>/devcfg.git
cd devcfg

# 2. Download dependencies
go mod download

# 3. Build the binary
go build -o devcfg .

# 4. Run locally
./devcfg
```

### Run without building

```bash
go run .
```

### Project layout

```
devcfg/
├── main.go                        Entry point
├── go.mod / go.sum                Module definition and lock file
└── internal/
    ├── system/detect.go           OS + package manager detection
    ├── registry/
    │   ├── registry.go            Tool registry (go:embed)
    │   └── tools.json             Tool definitions
    ├── resolver/resolver.go       brew / apt / fallback selection
    ├── executor/executor.go       Command runner
    └── tui/
        ├── app.go                 Root Bubble Tea model (step orchestrator)
        ├── tuistyles/styles.go    Lipgloss theme
        └── steps/
            ├── tools.go           Step 1 — Tools checklist
            ├── git.go             Step 2 — Git config form
            ├── docker.go          Step 3 — Docker checks
            └── shell.go           Step 4 — Shell setup
```

### Making changes

- **Add a new tool** — edit `internal/registry/tools.json` (rebuild required to embed the updated file).
- **Add a new TUI step** — create a new file under `internal/tui/steps/`, implement the `tea.Model` interface, and wire it up in `internal/tui/app.go`.
- **Change styling** — update the Lipgloss theme in `internal/tui/tuistyles/styles.go`.

### Submitting a PR

1. Create a feature branch: `git checkout -b feat/my-change`
2. Make and commit your changes
3. Open a pull request against `main`

---

## 📦 CI/CD

GitHub Actions workflow (`.github/workflows/release.yml`) triggers on `v*` tag pushes and:

1. Builds `devcfg-linux-amd64` (cross-compiled on ubuntu-latest)
2. Builds `devcfg-darwin-arm64` (cross-compiled on ubuntu-latest)
3. Creates a GitHub Release with both binaries

---

## 🌍 Distribution

```bash
# Linux (amd64)
curl -L https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-linux-amd64 -o devcfg \
  && chmod +x devcfg && ./devcfg

# macOS (Apple Silicon)
curl -L https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-darwin-arm64 -o devcfg \
  && chmod +x devcfg && ./devcfg
```

---

## 🧩 Philosophy

- **SSH-external** — no internal SSH logic; runs where you are
- **Local execution only** — all commands run on the target machine
- **Structured workflow** — not just a package installer
- **Keyboard-first UX** — inspired by `tssh` style
- **Deterministic + minimal** — explicit steps, clear feedback
- **Extensible via registry** — the bundled tool registry is defined in `tools.json`; changes require rebuilding the binary
