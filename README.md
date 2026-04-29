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

> **✨ Fully Customizable** — `devcfg` uses a simple JSON tool registry that you can modify to add your own tools, creating custom environment configurations for your team or organization. See [🎨 Customization](#-customization) for details.

## 📑 Table of Contents

- [🚀 Quick Start](#-quick-start)
  - [Recommended: One-line Install Script](#recommended-one-line-install-script)
  - [Option 1: Download Pre-built Binary (if available)](#option-1-download-pre-built-binary-if-available)
  - [Option 2: Build from Source](#option-2-build-from-source)
- [🎮 TUI Navigation](#-tui-navigation)
- [🪜 Workflow Steps](#-workflow-steps)
  - [Step 1 — Tools Installation](#step-1--tools-installation)
  - [Step 2 — Git Configuration](#step-2--git-configuration)
  - [Step 3 — Docker Setup](#step-3--docker-setup)
- [🏗️ Architecture](#-architecture)
  - [Layer Responsibilities](#layer-responsibilities)
- [⚙️ Tool Model (tools.json)](#-tool-model-toolsjson)
- [🧠 Resolver Priority](#-resolver-priority)
- [🎨 Customization](#-customization)
- [📦 Build from Source](#-build-from-source)
- [🧪 Testing](#-testing)
- [🛠️ Dev Setup (Contributing)](#-dev-setup-contributing)
- [📦 CI/CD](#-cicd)
- [🌍 Distribution](#-distribution)
- [🧩 Philosophy](#-philosophy)

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

Available tools: `git`, `neovim`, `docker`, `lazydocker`, `nvm`, `python3`, `curl`, `tmux`, `htop`, `ripgrep`, `fzf`, `zsh`, `bat`, `tssh`, `lazygit`

```
Step 1/3 — Tools Installation

  [ ] git          Version control system
  [✓] neovim       Hyperextensible text editor
▶ [ ] docker       Container platform
  [ ] nvm          Node Version Manager
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
│   │   └── tools.json              Tool definitions (18 tools)
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

## 🎨 Customization

`devcfg` is designed to be **easily customizable** to fit your specific development environment needs. The tool registry is defined in a simple JSON file that you can modify to add, remove, or update tools.

### Adding Custom Tools

To add your own tools to the registry:

1. **Edit the tool registry** — Modify `internal/registry/tools.json` to include your custom tool definitions
2. **Define installation methods** — Specify how the tool should be installed on different systems:
   - `brew`: Homebrew package name (for macOS and Linux with Homebrew)
   - `apt`: APT package name (for Debian/Ubuntu systems)
   - `fallback`: Shell script command for systems without package managers
3. **Rebuild the binary** — Run `go build -o devcfg .` to embed the updated registry
4. **Deploy** — Use your customized binary on your target machines

### Tool Definition Format

Each tool in `tools.json` follows this structure:

```json
{
  "name": "tool-name",
  "description": "Human-readable description",
  "binary": "executable-name",
  "brew": "homebrew-package-name",
  "apt": "apt-package-name",
  "fallback": "curl -fsSL https://example.com/install.sh | bash",
  "requires": ["dependency-tool-1", "dependency-tool-2"]
}
```

**Field Details:**
- `name` — Unique identifier for the tool (required)
- `description` — Shown in the TUI checklist (required)
- `binary` — Executable name to check if installed (leave empty for non-binary tools like fonts or configs)
- `binaryAliases` — Array of alternative binary names to check (e.g., `["bat", "batcat"]`)
- `brew` — Homebrew formula/cask name
- `apt` — APT package name
- `fallback` — Shell command to install when package managers aren't available
- `requires` — Array of tool names that must be installed first (optional)

### Customization Examples

**Add a custom development tool:**
```json
{
  "name": "mycli",
  "description": "My custom CLI tool",
  "binary": "mycli",
  "brew": "myorg/tap/mycli",
  "apt": "",
  "fallback": "curl -fsSL https://mycli.dev/install.sh | bash"
}
```

**Add a tool with dependencies:**
```json
{
  "name": "my-neovim-config",
  "description": "My Neovim configuration",
  "binary": "",
  "brew": "",
  "apt": "",
  "fallback": "git clone https://github.com/me/my-nvim-config ~/.config/nvim",
  "requires": ["neovim", "git"]
}
```

### Why Customization Matters

- **Team-specific tooling** — Create a company-wide `devcfg` with your organization's standard tools
- **Role-specific environments** — Build different configurations for backend, frontend, DevOps, etc.
- **Reproducible setups** — Ensure every team member has the same development environment
- **Easy onboarding** — New team members can set up their machines with a single command

The registry is embedded at build time using Go's `go:embed` directive, making your customized binary completely self-contained and portable.

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

- **Add a new tool** — edit `internal/registry/tools.json` (rebuild required to embed the updated file). See [🎨 Customization](#-customization) for detailed instructions.
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
