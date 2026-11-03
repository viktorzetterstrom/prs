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
