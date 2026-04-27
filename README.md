# 🧭 devcfg — Linux/macOS Environment Configuration TUI

[![Release](https://img.shields.io/github/v/release/I3-rett/devcfg)](https://github.com/I3-rett/devcfg/releases)
[![Tests](https://github.com/I3-rett/devcfg/actions/workflows/test.yml/badge.svg)](https://github.com/I3-rett/devcfg/actions/workflows/test.yml)
[![Pipeline](https://github.com/I3-rett/devcfg/actions/workflows/release.yml/badge.svg)](https://github.com/I3-rett/devcfg/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go)](https://go.dev/dl/)
[![Go Report Card](https://goreportcard.com/badge/github.com/I3-rett/devcfg)](https://goreportcard.com/report/github.com/I3-rett/devcfg)

`devcfg` is a CLI TUI written in Go that lets you **configure a Linux/macOS machine after an SSH connection**, without any built-in SSH logic.

Workflow:
1. Connect manually via SSH
2. Download and run `devcfg`
3. Follow the interactive TUI workflow
4. Configure your environment (tools, git, docker…)
5. Everything runs locally on the remote machine

---

## 🚀 Quick Start

### Recommended: One-line Install Script

The installation script automatically downloads a pre-built binary if available, or builds from source if not. It installs the binary to `~/.local/bin` and updates your PATH:

```bash
curl -fsSL https://raw.githubusercontent.com/I3-rett/devcfg/main/install.sh | bash
```

The script will:
- Download or build the `devcfg` binary
- Install it to `~/.local/bin` (creating the directory if needed)
- Add `~/.local/bin` to your PATH in your shell configuration file (`.bashrc`, `.zshrc`, etc.)
- Provide instructions for making `devcfg` immediately available

After installation, either:
1. Open a new terminal, or
2. Run `source ~/.bashrc` (or `~/.zshrc` for zsh)

Then you can run `devcfg` from anywhere.

### Option 1: Download Pre-built Binary (if available)

```bash
# Linux (amd64)
curl -fsSL https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-linux-amd64 -o devcfg \
  && chmod +x devcfg && ./devcfg

# macOS (Apple Silicon)
curl -fsSL https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-darwin-arm64 -o devcfg \
  && chmod +x devcfg && ./devcfg
```

> **Note:** If you get a "Not Found" error, releases may not be published yet. Use the install script or Option 2 below.

### Option 2: Build from Source

**Requirements:** [Go 1.24+](https://go.dev/dl/)

```bash
git clone https://github.com/I3-rett/devcfg.git
cd devcfg
go build -o devcfg .
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

Available tools: `git`, `neovim`, `docker`, `nodejs`, `python3`, `curl`, `tmux`, `htop`, `ripgrep`, `fzf`, `zsh`, `starship`, `bat`

```
Step 1/3 — Tools Installation

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

---

## 🏗️ Architecture

```
devcfg/
├── main.go                         Entry point
├── internal/
│   ├── system/detect.go            OS + package manager detection
│   ├── registry/
│   │   ├── registry.go             Tool registry (go:embed)
│   │   └── tools.json              Tool definitions (13 tools)
│   ├── resolver/resolver.go        brew / apt / fallback selection
│   ├── executor/executor.go        Command runner (stdout+stderr capture)
│   └── tui/
│       ├── app.go                  Root Bubble Tea model (step orchestrator)
│       ├── tuistyles/styles.go     Lipgloss theme (purple/teal)
│       └── steps/
│           ├── tools.go            Step 1 — Tools checklist
│           ├── git.go              Step 2 — Git config form
│           └── docker.go           Step 3 — Docker checks
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

## 🧪 Testing

### Guidelines

Tests in this project follow standard Go conventions and are co-located with the packages they cover (`*_test.go` in the same directory).

#### Principles

- **Table-driven tests** — use `[]struct{ name, input, want }` slices and iterate with `t.Run(tc.name, ...)` to keep cases readable and easy to extend.
- **No external test framework** — only the standard `testing` package. Helpers from `testing/iotest` or `os/exec` stubs where relevant.
- **Isolation** — unit tests must not make real network calls or perform persistent filesystem mutations. Use `t.TempDir()` for temporary files, keep filesystem writes confined there, and restore env variables with `t.Setenv()`.
- **One assertion per sub-test** — keep each `t.Run` focused; avoid asserting unrelated things together.
- **Error paths covered** — every function that returns an `error` must have at least one test case that triggers the error branch.
- **Deterministic** — tests must not rely on timing, random values, or test-execution order.

#### Coverage targets

| Package | Priority | Notes |
|---------|----------|-------|
| `internal/system` | High | Mock filesystem for `/etc/os-release`; mock `PATH` for binary detection |
| `internal/registry` | High | Validate `List()` length + fields; `Find()` hit and miss |
| `internal/resolver` | High | All package-manager × tool combinations; fallback; error case |
| `internal/executor` | Medium | Real subprocess (`echo`); empty args; failing command |
| `internal/tui` | Low | Pure View/Init smoke tests; no interactive input required |

#### Running tests

```bash
# All packages
go test ./...

# With race detector (recommended in CI)
go test -race ./...

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

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
            └── docker.go          Step 3 — Docker checks
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

Pre-built binaries are available in [GitHub Releases](https://github.com/I3-rett/devcfg/releases) once a version tag is pushed.

```bash
# Linux (amd64)
curl -fsSL https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-linux-amd64 -o devcfg \
  && chmod +x devcfg && ./devcfg

# macOS (Apple Silicon)
curl -fsSL https://github.com/I3-rett/devcfg/releases/latest/download/devcfg-darwin-arm64 -o devcfg \
  && chmod +x devcfg && ./devcfg
```

> **Note:** If no releases have been published yet, build from source following the instructions in the Quick Start section.

---

## 🧩 Philosophy

- **SSH-external** — no internal SSH logic; runs where you are
- **Local execution only** — all commands run on the target machine
- **Structured workflow** — not just a package installer
- **Keyboard-first UX** — inspired by `tssh` style
- **Deterministic + minimal** — explicit steps, clear feedback
- **Extensible via registry** — the bundled tool registry is defined in `tools.json`; changes require rebuilding the binary
