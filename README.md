# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

It shows your open PRs across the repos you choose to track and auto-refreshes every 30s. A key legend at the bottom of every screen shows what you can do.

## Prerequisites

- **The [`gh` CLI](https://cli.github.com/)** — `prs` shells out to `gh pr list`, so it has to be installed and authenticated:
  ```bash
  brew install gh
  gh auth login
  ```
- **A repo to track.** On first launch `prs` opens a picker pre-filled with the GitHub repo of the current directory (if any) and the repos you have recently opened PRs in. Once you have picked, it works from any directory.

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

The first time you run `prs` it opens the **Tracked repos** picker. Afterwards press `c` to get back to it. It lists every repo `prs` is following, then the repo of the current directory and the repos you have recently authored PRs in (looked up via `gh search prs --author=@me`) as untracked candidates.

| Key           | Action                                                        |
| ------------- | ------------------------------------------------------------- |
| ↑ / ↓         | Move between repos                                            |
| Space         | Track or untrack the highlighted repo                         |
| `a`           | Add a repo by typing `OWNER/REPO`                             |
| Enter / Esc   | Done: back to the PR list (refetches if you changed anything) |

The list is saved to `~/.config/prs/config.json` (or `$XDG_CONFIG_HOME/prs/config.json`) as soon as you change it, so you can also edit it by hand:

```json
{
  "repos": ["sanalabs/sierra-platform", "sanalabs/sierra-config"]
}
```

When more than one repo is tracked, the list is grouped under one heading per repo (in the order you track them), newest update first within each group. The copied Slack line then links as `repo#123` instead of a bare `#123`. Each repo is fetched in parallel; if one fails, the others still show and the error appears in the footer.

## Keybindings

| Key          | Action                                       |
| ------------ | -------------------------------------------- |
| ↑ / ↓        | Navigate the list                            |
| Enter        | Copy the selected PR (Slack format)          |
| `o`          | Open the selected PR in your default browser |
| `r`          | Refresh                                      |
| `c`          | Choose which repos to track                  |
| `q` / Ctrl+C | Quit                                         |

Vim-style aliases also work on both screens: `j` / `k` to move, `g` / `G` (or Home / End) for first / last, PgUp / PgDn (or Ctrl+U / Ctrl+D) for half a page, and `y` to copy.
