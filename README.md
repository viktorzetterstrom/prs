# `prs`

A terminal UI for your open GitHub pull requests.

## Install

Requires the [`gh` CLI](https://cli.github.com/), installed and logged in (`gh auth login`).

```bash
go install github.com/viktorzetterstrom/prs@latest
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure that is on your `PATH`. Re-run the same command to update.

## Usage

| Key     | PR list              | Config screen            |
| ------- | -------------------- | ------------------------ |
| ↑ / ↓   | Move                 | Move                     |
| Enter   | Copy for Slack       | Done                     |
| Space   |                      | Track / untrack repo     |
| `a`     |                      | Add a repo by name       |
| `o`     | Open in browser      |                          |
| `r`     | Refresh              |                          |
| `c`     | Open config          | Back to list             |
| `q`     | Quit                 | Quit                     |

Vim keys (`j`/`k`, `g`/`G`, `y` to copy) work too.

Config is stored in `~/.config/prs/config.json`.

## Scripts and agents

When stdout is not a terminal, `prs` prints a tab-separated list and exits instead of opening the UI.

| Command                     |                                                               |
| --------------------------- | ------------------------------------------------------------- |
| `prs --json`                | Your open PRs as JSON, with review and CI status and the Slack line |
| `prs repos`                 | List tracked repos                                            |
| `prs repos add OWNER/REPO…` | Start tracking repos                                          |
| `prs repos rm OWNER/REPO…`  | Stop tracking repos                                           |
| `prs skill install`         | Install a Claude Code skill that teaches Claude these commands |
| `prs --version`             | Print the installed version                                   |

`prs skill` prints the skill instead, to read it or to give it to another agent. Once installed, the skill stays in step with `prs`: after an update, the next run of `prs` refreshes it.
