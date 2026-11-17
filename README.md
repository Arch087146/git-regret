# git-regret 🔥

Finds your sketchy commits (`"quick fix lol"`, `"temporary hack"`, 2am commits)
and **proves** they were regrets by linking them to the later commits that had
to fix what they broke. Then it scores you.

A useless commit is an opinion. A "temporary hack" that caused a bugfix 41 days
later is a conviction.

## Usage

```bash
# from source (requires Go 1.22+)
go install github.com/Arch087146/git-regret@latest

# or grab a prebuilt binary from GitHub Releases
# or build locally
git clone https://github.com/Arch087146/git-regret && cd git-regret
go build -o git-regret .
```

```bash
git-regret                    # interactive TUI
git-regret .                  # judge the current repo
git-regret . --json           # machine-readable shame (CI-friendly)
git-regret . --author "me@work.com"
git-regret . --since "1 year ago"
git-regret . --md REGRET.md   # write a shareable markdown report
```

## The Regret Index

Weighted score 0–100:

| Signal | Weight |
|---|---|
| Regret rate — hacks later undone by fix commits | 55% |
| Sketchy commit share (message-based suspects) | 20% |
| Late-night commits (22:00–05:00, commit's own timezone) | 15% |
| Weekend commits | 10% |

A suspect is only "proven" when a later fix-type commit touches a file the
suspect touched, within 90 days. One fix redeems one suspect, so scores can't
be farmed by reverting twice.

Grades: **S** (robot, suspicious) → **A** → **B** → **C** (serial quick-fixer)
→ **D** (chaos agent) → **F** (hold my beer).

## Pairs well with

- [`repo-roast`](https://github.com/Arch087146/repo-roast) — roasts the rest of your repo
- [`repo-sanitizer`](https://github.com/Arch087146/repo-sanitizer) — finds the secrets you're about to leak

MIT
