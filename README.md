> [!WARNING]
> This project is under active development. Features may change, break, or disappear without notice.

<p align="center">
  <!-- TODO: Replace with actual logo -->
  <!-- <img src="assets/logo.png" alt="kommit" width="500"> -->
  <h1 align="center">kommit</h1>
</p>

<p align="center">
  <strong>A terminal-native git client with built-in AI.</strong>
</p>

<p align="center">
  <a href="https://github.com/heesungjang/opengit/releases"><img src="https://img.shields.io/github/v/release/heesungjang/opengit?style=flat-square&color=a6e3a1" alt="Release"></a>
  <a href="https://github.com/heesungjang/opengit/actions"><img src="https://img.shields.io/github/actions/workflow/status/heesungjang/opengit/ci.yml?style=flat-square&color=89b4fa" alt="Build"></a>
  <a href="https://goreportcard.com/report/github.com/heesungjang/kommit"><img src="https://goreportcard.com/badge/github.com/heesungjang/kommit?style=flat-square" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-b4befe?style=flat-square" alt="License"></a>
  <a href="https://github.com/heesungjang/opengit/releases"><img src="https://img.shields.io/github/downloads/heesungjang/opengit/total?style=flat-square&color=f5c2e7" alt="Downloads"></a>
</p>

---

<p align="center">
  <img src="assets/demo.gif" alt="kommit demo" width="800">
</p>

## About

kommit is a TUI git client with AI commit message and pull request description generation. It provides visual diffs, commit graphs, staging, branch management, and the full range of git operations, all within the terminal.

AI features support Anthropic, OpenAI, GitHub Copilot, and any OpenAI-compatible endpoint, including local models through Ollama or LM Studio. Bring your own API key.

---

## Features

### AI-Powered Workflow

- **AI commit messages**: press `ctrl+g` on staged changes to generate a commit summary and description
- **AI pull request descriptions**: generates title and markdown body from the branch diff
- **4 providers**: Anthropic (Claude), OpenAI (GPT), GitHub Copilot (OAuth device flow), or any OpenAI-compatible API
- **Bring your own key**: use your existing API keys, no kommit account needed
- **Local models**: point the OpenAI-compatible provider at Ollama, LM Studio, or any local endpoint

<p align="center">
  <img src="assets/ai-commit.gif" alt="AI commit message generation" width="800">
</p>

### Git Operations

- **Commit**: write messages inline or in a focused dialog, amend previous commits
- **Staging**: stage/unstage files, hunks, or individual lines with visual selection
- **Branches**: create, rename, delete, checkout, merge, rebase
- **Stash**: save with message, pop, apply, drop, all from the sidebar
- **Push / Pull / Fetch**: with automatic credential injection for saved accounts
- **Interactive rebase**: squash, fixup, drop, reword commits from the commit list
- **Cherry-pick and revert**: single-key operations on any commit
- **Reset**: soft, mixed, or hard reset with a picker menu
- **Bisect**: start, good, bad, skip, reset
- **Tags**: create (lightweight or annotated) and delete

### Pull Requests

- **PR list in sidebar**: view all open PRs with status indicators, updated on push/pull/fetch
- **Create PRs**: fill in title and body, toggle draft mode, submit without leaving the terminal
- **AI-generated PR descriptions**: `ctrl+g` in the create dialog generates the title and body
- **Open in browser**: `o` to open the PR on GitHub

### Multi-Provider Authentication

- **GitHub**: OAuth device flow (one-click login, no token pasting)
- **GitLab / Azure DevOps / Bitbucket**: personal access token input
- **Auto-detect provider**: reads the remote URL and selects the correct provider
- **Credential injection**: saved tokens are automatically provided to push/pull/fetch via `GIT_ASKPASS`
- **Auth failure recovery**: if a push fails due to auth, kommit offers to log in and retry

### Diff Viewer

- **Inline (unified) and side-by-side**: toggle with `V`
- **Hunk navigation**: jump between hunks with `n`/`N`
- **Hunk staging**: stage or unstage entire hunks from the diff view
- **Visual line selection**: press `v`, select a range, stage those lines
- **Horizontal panning**: scroll long lines with `h`/`l`
- **Line numbers**: dual-gutter display with old and new line numbers

### Interface

- **3-panel layout**: sidebar (branches, tags, stash, PRs) | commit list with graph | detail/WIP view
- **Action bar**: context-aware toolbar showing current branch, sync status, and logged-in account
- **Hint bar**: dynamic keyboard shortcut hints that change based on the focused panel
- **Command palette**: `ctrl+p` for fuzzy search across all available actions
- **11 themes**: switch live with instant preview
- **Fully rebindable keys**: remap any of the 70+ keybindings in config
- **Custom commands**: define shell commands with template variables, trigger from a menu or shortcut

---

## Installation

### Go install

```bash
go install github.com/heesungjang/kommit@latest
```

### Build from source

```bash
git clone https://github.com/heesungjang/opengit.git
cd opengit
go build -o kommit .
```

<!-- ### Homebrew (coming soon)
```bash
brew install kommit
``` -->

---

## Quick Start

```bash
# Open kommit in the current repo
kommit

# Open a specific repo
kommit --repo /path/to/repo

# Enable debug logging
kommit --debug
```

Navigate with `hjkl` or arrow keys, `tab`/`shift+tab` between panels, `?` for the full shortcut reference.

---

## Configuration

kommit uses a layered config system. Settings are loaded in this order (last wins):

| Priority | Location | Scope |
|---|---|---|
| 1 | `~/.config/kommit/config.yaml` | Global |
| 2 | `$XDG_CONFIG_HOME/kommit/config.yaml` | Global (XDG) |
| 3 | `.kommit.yaml` | Project-local |

Open the settings dialog with `,` to configure themes, diff mode, AI provider, and more without editing files.

### Credential Storage

API keys and account tokens are stored separately from config:

```
~/.local/share/kommit/auth.json    (permissions: 0600)
```

This file is never committed and never synced with your config. Environment variables are also supported:

| Variable | Overrides |
|---|---|
| `ANTHROPIC_API_KEY` | AI API key (when provider is Anthropic) |
| `OPENAI_API_KEY` | AI API key (when provider is OpenAI) |
| `KOMMIT_AI_API_KEY` | AI API key (any provider) |
| `GITHUB_TOKEN` | GitHub hosting token |
| `GITLAB_TOKEN` | GitLab hosting token |

---

## AI Setup

Press `ctrl+g` on staged changes. If no provider is configured, kommit will walk through setup with an interactive dialog.

Or configure manually:

```yaml
# ~/.config/kommit/config.yaml
ai:
  provider: anthropic     # anthropic | openai | copilot | openai-compatible
  model: claude-sonnet-4-6  # model name for the selected provider
```

### Providers

| Provider | Default Model | Auth | Notes |
|---|---|---|---|
| `anthropic` | `claude-sonnet-4-6` | API key | Via [console.anthropic.com](https://console.anthropic.com) |
| `openai` | `gpt-4o-mini` | API key | Via [platform.openai.com](https://platform.openai.com) |
| `copilot` | `gpt-4o` | OAuth device flow | Uses your existing GitHub Copilot subscription |
| `openai-compatible` | `default` | Optional API key | Any endpoint that speaks the OpenAI API |

### Using Local Models

Point the `openai-compatible` provider at any local inference server:

```yaml
ai:
  provider: openai-compatible
  model: llama3
  apiBaseURL: http://localhost:11434/v1   # Ollama
```

Works with Ollama, LM Studio, and any server that implements the OpenAI chat completions API.

---

## Themes

kommit ships with 11 themes. Switch between them in the settings dialog (`,`) with live preview.

| Theme | Style |
|---|---|
| `catppuccin-mocha` | Dark (default) |
| `catppuccin-latte` | Light |
| `catppuccin-frappe` | Mid-dark, French lavender undertones |
| `catppuccin-macchiato` | Dark, warm and cozy |
| `tokyo-night` | Inspired by the lights of downtown Tokyo |
| `dracula` | Distinctive pink, purple, and green accents |
| `nord` | Arctic, north-bluish palette with cool, muted tones |
| `gruvbox-dark` | Warm retro palette with earthy tones |
| `rose-pine` | Soft, muted dark with warm rose and gold |
| `kanagawa-wave` | Inspired by Hokusai's *The Great Wave off Kanagawa* |
| `auto` | Detects your terminal background and picks light or dark |

Every theme color is overridable in config:

```yaml
appearance:
  theme: tokyo-night
  themeColors:
    base: "#1a1b26"
    text: "#c0caf5"
    # ... any of the 25 color slots
```

---

## Custom Commands

Define your own shell commands and trigger them from the command menu (`:`) or a custom keybinding.

```yaml
customCommands:
  - name: "Fixup last commit"
    command: "git commit --amend --no-edit"
    key: "ctrl+f"
    confirm: true
    showOutput: true

  - name: "Open in editor"
    command: "code {{.RepoRoot}}"
    context: global
    suspend: true

  - name: "Blame file"
    command: "git blame {{.Path}}"
    context: file
    showOutput: true
```

### Template Variables

| Variable | Value |
|---|---|
| `{{.Hash}}` | Full commit hash |
| `{{.ShortHash}}` | Abbreviated hash |
| `{{.Branch}}` | Current branch name |
| `{{.Path}}` | Selected file path |
| `{{.RepoRoot}}` | Repository root directory |
| `{{.Subject}}` | Commit subject line |
| `{{.Author}}` | Commit author name |

---

<details>
<summary><strong>Keyboard Shortcuts</strong></summary>

All keybindings are rebindable in your config file. Press `?` in the app to see the full reference.

### Global

| Key | Action |
|---|---|
| `q` | Quit |
| `ctrl+c` | Force quit |
| `?` | Toggle help |
| `/` | Search |
| `ctrl+p` | Command palette |
| `:` | Custom commands |
| `,` | Settings |
| `1` `2` `3` | Focus panel |
| `tab` / `shift+tab` | Next / previous panel |

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Down / up |
| `h` / `l` | Left / right |
| `ctrl+d` / `ctrl+u` | Page down / up |
| `g` / `G` | Top / bottom |
| `enter` | Select |

### WIP Panel (Staging & Committing)

| Key | Action |
|---|---|
| `s` | Stage file |
| `u` | Unstage file |
| `a` | Stage all |
| `S` | Stage hunk |
| `d` | Discard changes |
| `c` | Commit |
| `A` | Amend commit |
| `ctrl+g` | AI commit message |
| `p` | Push |
| `P` | Pull |
| `f` | Fetch |
| `W` | Stash save |
| `X` | Stash pop |
| `z` | Undo |
| `r` | Refresh |

### Branch Operations

| Key | Action |
|---|---|
| `enter` / `o` | Checkout |
| `n` | New branch |
| `R` | Rename |
| `D` | Delete |
| `m` | Merge into current |
| `b` | Rebase onto current |

### Commit Operations

| Key | Action |
|---|---|
| `R` | Revert |
| `C` | Cherry-pick |
| `y` | Copy hash |
| `X` | Reset menu |
| `s` | Squash |
| `f` | Fixup |
| `d` | Drop |
| `B` | Bisect menu |
| `z` / `Z` | Undo / redo |

### Diff Viewer

| Key | Action |
|---|---|
| `n` / `N` | Next / previous hunk |
| `s` / `u` | Stage / unstage hunk |
| `v` | Visual line selection |
| `V` | Toggle inline / side-by-side |
| `h` / `l` | Pan left / right |

### Stash

| Key | Action |
|---|---|
| `s` | Save |
| `p` | Pop |
| `a` | Apply |
| `D` | Drop |

### Pull Requests

| Key | Action |
|---|---|
| `n` | Create PR |
| `o` | Open in browser |
| `ctrl+g` | AI description |

### Rebinding Keys

```yaml
keybinds:
  custom:
    global.quit: "Q"
    status.stage: "space"
    nav.up: "up k"          # multiple keys separated by space
    nav.down: "down j"
```

</details>

<details>
<summary><strong>Appearance Config</strong></summary>

```yaml
appearance:
  theme: catppuccin-mocha
  diffMode: inline           # inline | side-by-side
  showGraph: true            # commit graph in center panel
  compactLog: false          # compact commit list
  sidebarWidth: 0            # fixed width (0 = auto)
  sidebarMaxPct: 15          # max sidebar width as % of terminal
  centerPct: 70              # center panel width as % of remaining space
```

</details>

---

## Contributing

Contributions are welcome. Open an issue to discuss what you'd like to change, then submit a PR.

```bash
# Build
go build ./...

# Vet
go vet ./...

# Lint (requires golangci-lint)
golangci-lint run ./...
```

---

## License

[MIT](LICENSE)

---

<p align="center">
  Built with <a href="https://github.com/charmbracelet/bubbletea">Bubble Tea</a> and <a href="https://github.com/charmbracelet/lipgloss">Lip Gloss</a>.
</p>
