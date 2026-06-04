# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

https://github.com/user-attachments/assets/7967eb51-ab46-4ff5-9083-1819c6beaf5a

Two tabs are available: **Active** (your open PRs) and **Last 7 days** (everything authored by you in the last seven days, including closed/merged). Use `Tab` to switch. Each tab is fetched lazily the first time you open it and auto-refreshes every 30s while visible.

`--last-week` makes the TUI start on the **Last 7 days** tab.


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
