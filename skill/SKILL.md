---
name: prs
description: Look up the user's own open GitHub pull requests across the repos they track, with review and CI status and a ready-made Slack line for each, using the `prs` CLI. Use when the user asks about their open PRs, which of their PRs need attention or are waiting on review, wants their PRs listed or posted in Slack, or wants to change which repos prs tracks.
---

# prs

`prs` lists the pull requests the user authored that are still open, across the repos they track. It wraps the `gh` CLI, which must be installed and logged in.

Always run `prs --json` or one of the subcommands below. Bare `prs` is a full-screen interface for a human; without a terminal it prints a plain tab-separated list instead, which has less in it than the JSON.

## List open PRs

```bash
prs --json
```

```json
{
  "repos": ["owner/platform", "owner/config"],
  "prs": [
    {
      "repo": "owner/platform",
      "number": 142,
      "title": "Retry failed webhooks",
      "url": "https://github.com/owner/platform/pull/142",
      "status": "approved",
      "draft": false,
      "review": "APPROVED",
      "ci": "SUCCESS",
      "mergeable": "MERGEABLE",
      "additions": 12,
      "deletions": 3,
      "updatedAt": "2026-11-19T09:12:44Z",
      "slack": "`(+12/-3)` Retry failed webhooks [#142](https://github.com/owner/platform/pull/142)"
    }
  ],
  "errors": []
}
```

- `repos`: the repos that were queried. A repo listed here with no entry in `prs` has no open PRs by the user.
- `prs`: most recently updated first.
  - `status`: what most needs attention, first match wins:
    - `draft`: still a draft.
    - `conflicting`: has merge conflicts with its base branch.
    - `ci_failing`: at least one check failed.
    - `changes_requested`: a reviewer asked for changes.
    - `approved`: approved, CI not failing. Check `ci` for `PENDING` before calling it ready to merge.
    - `review_required`: waiting on a reviewer.
    - `open`: nothing flagged.
  - `ci`: `SUCCESS`, `PENDING`, `FAILURE`, or `""` when the repo runs no checks.
  - `review`: GitHub's review decision: `APPROVED`, `CHANGES_REQUESTED`, `REVIEW_REQUIRED`, or `""` when GitHub gives none, usually because the repo does not require reviews.
  - `mergeable`: `MERGEABLE`, `CONFLICTING`, or `UNKNOWN` when GitHub has not computed it yet.
  - `slack`: the line the user posts in Slack for this PR. When they ask for Slack lines, give these exactly as they are, one per line.
- `errors`: repos that could not be fetched, each with gh's message. When there are any, the exit code is 1, but the other repos' PRs are still in `prs`.

## Tracked repos

```bash
prs repos                         # one per line
prs repos add OWNER/REPO [...]    # start tracking
prs repos rm OWNER/REPO [...]     # stop tracking
```

The list is the user's own setting, stored in `~/.config/prs/config.json`. Change it only when they ask, and with these commands rather than by editing the file. Adding a repo that is already tracked, or removing one that is not, does nothing, so the commands are safe to repeat.

When no repos are tracked, prs falls back to the repo of the current directory. Look at `repos` in the output before telling the user they have no open PRs anywhere else.

## What prs does not cover

It lists only open PRs, only ones the user authored, and only in tracked repos. For anything else, such as other people's PRs, merged PRs, diffs, comments or checks in detail, use `gh` directly, for example `gh pr view 142 --repo owner/platform --comments`.
