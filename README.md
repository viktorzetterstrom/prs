<img src="assets/icon.svg" alt="" width="96" height="96">

# `prs`

A terminal UI for your open GitHub pull requests, and the ones merged or closed in the last 7 days.

## Install

Requires the [`gh` CLI](https://cli.github.com/), installed and logged in (`gh auth login`).

```bash
go install github.com/viktorzetterstrom/prs@latest
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure that is on your `PATH`. Re-run the same command to update.

## Usage

| Key     | PR list              | Config screen            |
| ------- | -------------------- | ------------------------ |
| ← / →   | Switch tab           |                          |
| ↑ / ↓   | Move                 | Move                     |
| Enter   | Copy for Slack       | Done                     |
| Space   |                      | Track / untrack repo     |
| `a`     |                      | Add a repo by name       |
| `o`     | Open in browser      |                          |
| `r`     | Refresh              |                          |
| `c`     | Open config          | Back to list             |
| `q`     | Quit                 | Quit                     |

Tab and Shift+Tab switch tabs too, and vim keys work (`h`/`l`, `j`/`k`, `g`/`G`, `y` to copy).

Config is stored in `~/.config/prs/config.json`.

### Experimental

The config screen's experimental section has opt-in features that may change or go away. Space turns one on or off.

- **Stack support** asks GitHub which [stack](https://docs.github.com/en/pull-requests/get-started/about-stacked-prs) each open PR belongs to. A stack's layers are listed together, bottom layer first, under a header row with their combined size, how many PRs the stack has and the lowest layer that holds the stack back. Each layer starts with its position, such as `2.`, in the list and in the copied Slack line, so the layers read as a numbered list. Enter on the header copies the whole stack as the list shows it: the header's line, then each layer's, joined by the same tree lines.

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
