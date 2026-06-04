# `prs`

A Go CLI tool to list GitHub pull requests and copy formatted PR info to your clipboard for Slack.

https://github.com/user-attachments/assets/7967eb51-ab46-4ff5-9083-1819c6beaf5a

You can also slap on `--last-week` to show all PRs you've been part of for the last seven days. In case you, like me, forget what you did.


## Install
```bash
go install github.com/viktorzetterstrom/prs@latest
```

## Keybindings

| Key            | Action                                       |
| -------------- | -------------------------------------------- |
| ↑ / ↓          | Navigate the list                            |
| Space / Enter  | Copy the selected PR (Slack format)          |
| `o`            | Open the selected PR in your default browser |
| `r`            | Refresh                                      |
| `q` / Ctrl+C   | Quit                                         |
