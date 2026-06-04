# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

https://github.com/user-attachments/assets/7967eb51-ab46-4ff5-9083-1819c6beaf5a

Three tabs are available: **Active** (your open PRs), **Action needed** (PRs in this repo where someone has requested your review or mentioned you), and **Last 7 days** (PRs authored by you that were merged or closed in the last seven days). Use `Tab` to switch. Each tab is fetched lazily the first time you open it and auto-refreshes every 30s while visible.

`--last-week` makes the TUI start on the **Last 7 days** tab.

`--demo` shows a fixed list of fake PRs that exercise every status emoji — handy when you just want to see what the TUI looks like.

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


## Install
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
