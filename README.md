# git-regret 🔥

Finds your sketchy commits (`"quick fix lol"`, `"temporary hack"`, 2am commits)
and **proves** they were regrets by linking them to the later commits that had
to fix what they broke. Then it scores you.

A useless commit is an opinion. A "temporary hack" that caused a bugfix 41 days
later is a conviction.

## Usage

```bash
node index.js            # human-readable shame
node index.js --json     # machine-readable shame (CI-friendly)
node index.js --author "me@work.com"
node index.js --since "1 year ago"
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

MIT
