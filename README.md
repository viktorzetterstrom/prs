# `prs`

A terminal UI for your open GitHub pull requests across the repos you track, with one-key copy to Slack.

## Install

Requires the [`gh` CLI](https://cli.github.com/), installed and logged in (`gh auth login`).

```bash
go install github.com/viktorzetterstrom/prs@latest
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure that is on your `PATH`. Re-run the same command to update.

## Usage

Run `prs`. On first launch it opens the config screen, where you pick which repos to follow: the repo of the current directory and the repos you have recently opened PRs in are suggested. Press `c` any time to change the selection.

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

The selection is stored in `~/.config/prs/config.json`. `prs --reset` deletes it and starts over.
