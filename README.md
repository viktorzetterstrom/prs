# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

Two tabs are available: **Active** (your open PRs) and **Last 7 days** (PRs authored by you that were merged or closed in the last seven days). Use `Tab` to switch. Each tab is fetched lazily the first time you open it and auto-refreshes every 30s while visible.

`--last-week` makes the TUI start on the **Last 7 days** tab.

## Status emojis

| Emoji | Meaning                                      |
| ----- | -------------------------------------------- |
| 🟣    | Merged                                       |
| ⚫    | Closed without merging                       |
| 📝    | Draft                                        |
| ⚠️    | Merge conflict — needs a rebase              |
| ❌    | CI is failing                                |
| 💬    | Reviewer requested changes                   |
| ✅    | Approved — ready to merge                    |
| 👀    | Waiting on a reviewer                        |
| 🟢    | Open, nothing flagged                        |

Priority order: the first matching state wins (so an approved PR with failing CI shows ❌, because that's the actionable thing).

## Prerequisites

- **The [`gh` CLI](https://cli.github.com/)** — `prs` shells out to `gh pr list`, so it has to be installed and authenticated:
  ```bash
  brew install gh
  gh auth login
  ```
- **Run from inside a git repo with a GitHub remote.** `gh pr list` needs the repo context.

## Install

```bash
go install github.com/viktorzetterstrom/prs@latest
```

`go install` drops the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`). If that directory isn't on your `$PATH`, the `prs` command won't be found after install — add it once:

```bash
# append to ~/.bashrc or ~/.zshrc
export PATH="$HOME/go/bin:$PATH"
```

## Update

Re-run the install command — `@latest` re-fetches the current `main`:

```bash
go install github.com/viktorzetterstrom/prs@latest
```

## Keybindings

| Key                | Action                                       |
| ------------------ | -------------------------------------------- |
| ↑ / ↓              | Navigate the list                            |
| Tab / Shift+Tab    | Switch between tabs                          |
| Space / Enter      | Copy the selected PR (Slack format)          |
| `o`                | Open the selected PR in your default browser |
| `r`                | Refresh the current tab                      |
| `q` / Ctrl+C       | Quit                                         |
