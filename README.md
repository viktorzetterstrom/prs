# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

Two tabs are available: **Active** (your open PRs) and **Last 7 days** (PRs authored by you that were merged or closed in the last seven days). Use `Tab` to switch. Each tab is fetched lazily the first time you open it and auto-refreshes every 30s while visible.

PRs can come from more than one repository at once — see [Tracking repos](#tracking-repos).

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
- **A repo to track.** With no config, `prs` uses the GitHub repo of the current directory, so run it from inside a clone. Once you have picked repos with `c` (see below) it works from anywhere.

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

## Tracking repos

Press `c` to open the **Tracked repos** screen. It lists every repo `prs` is following, plus the repo of the current directory as a candidate if it isn't tracked yet.

| Key           | Action                                        |
| ------------- | --------------------------------------------- |
| ↑ / ↓         | Move between repos                            |
| Space / Enter | Track or untrack the highlighted repo         |
| `a`           | Add a repo by typing `OWNER/REPO`             |
| Esc / `c`     | Back to the PR list (refetches if you changed anything) |

The list is saved to `~/.config/prs/config.json` (or `$XDG_CONFIG_HOME/prs/config.json`) as soon as you change it, so you can also edit it by hand:

```json
{
  "repos": ["sanalabs/sierra-platform", "sanalabs/sierra-config"]
}
```

When more than one repo is tracked, each row shows the repo name and the merged list is sorted by last update. The copied Slack line then links as `repo#123` instead of a bare `#123`. Each repo is fetched in parallel; if one fails, the others still show and the error appears in the footer.

## Keybindings

| Key                | Action                                       |
| ------------------ | -------------------------------------------- |
| ↑ / ↓              | Navigate the list                            |
| Tab / Shift+Tab    | Switch between tabs                          |
| Space / Enter      | Copy the selected PR (Slack format)          |
| `o`                | Open the selected PR in your default browser |
| `r`                | Refresh the current tab                      |
| `c`                | Choose which repos to track                  |
| `q` / Ctrl+C       | Quit                                         |
