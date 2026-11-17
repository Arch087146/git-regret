#!/usr/bin/env node
/*
 * git-regret — finds your sketchy commits and proves they were regrets,
 * by linking them to the later commits that had to fix what they broke.
 * Zero dependencies. Asks git for everything.
 */
"use strict";

const { execFileSync } = require("child_process");

/* ---------- tiny ANSI helpers ---------- */
const c = (code, s) => `\x1b[${code}m${s}\x1b[0m`;
const bold = (s) => c("1", s);
const dim = (s) => c("2", s);
const red = (s) => c("31", s);
const green = (s) => c("32", s);
const yellow = (s) => c("33", s);
const magenta = (s) => c("35", s);
const cyan = (s) => c("36", s);

/* ---------- classification ---------- */
// A commit message that smells like a hack, an apology, or a Friday deploy.
const SUSPECT_RE =
  /(quick ?fix|hotfix|temp(orary)?|hack(y|ed)?|work ?around|wip|do ?not (look|review|ship|deploy)|don'?t (look|ask)|sorry|oops|whoops|oopsie|damn|dammit|argh|ugh|fix ?later|later|band-?aid|kludge|jank|test commit|asdf|debug|why is this|please work|final fix|should work now|actually fix|for real this time)/i;

// A commit that looks like it exists to repair another commit.
const FIX_RE =
  /^(fix|fixes|fixed|bugfix|bug ?fix|hotfix|revert|rollback|undo|repair|patch)([:( ]|$)/i;

// One commit can be both; a "quick fix" that later needed a real fix is our
// favorite suspect, so suspects win when both match.
function classify(msg) {
  if (SUSPECT_RE.test(msg)) return "suspect";
  if (FIX_RE.test(msg)) return "fix";
  return "normal";
}

/* ---------- parsing ---------- */
// Ask git for every commit plus its touched files in ONE pass:
//   @@@<hash>|<author>|<author-date-iso>|<subject>
//   <file>
//   <file>
//   ...
const LOG_FORMAT = "@@@%H|%an|%ae|%ad|%s";
const DATE_FORMAT = "iso8601-strict"; // "2024-03-05T02:14:33+02:00" — keeps the commit's own offset

function loadHistory(author, since, maxCount) {
  const args = [
    "log",
    "--no-merges",
    `--pretty=format:${LOG_FORMAT}`,
    `--date=${DATE_FORMAT}`,
    "--name-only",
  ];
  if (author) args.push(`--author=${author}`);
  if (since) args.push(`--since=${since}`);
  if (maxCount) args.push(`--max-count=${maxCount}`);

  let out;
  try {
    out = execFileSync("git", args, { encoding: "utf8", maxBuffer: 256e6 });
  } catch {
    console.error(red("✗ Not a git repository (or git is missing)."));
    process.exit(1);
  }
  if (!out.trim()) return [];

  const commits = [];
  let cur = null;
  for (const line of out.split("\n")) {
    if (line.startsWith("@@@")) {
      if (cur) commits.push(cur);
      const [hash, name, email, date, msg] = line.slice(3).split("|");
      cur = {
        hash: hash.slice(0, 7),
        author: name,
        email,
        date,
        msg,
        files: [],
        kind: classify(msg),
        hour: localHour(date),
        weekend: isWeekend(date),
      };
    } else if (cur && line.trim()) {
      cur.files.push(line.trim());
    }
  }
  if (cur) commits.push(cur);
  // git log prints newest first; we want oldest first for the blame chain.
  return commits.reverse();
}

// "2024-03-05T02:14:33+02:00" -> 2 (the hour *at the commit's own timezone*)
function localHour(iso) {
  const m = iso.match(/T(\d{2}):\d{2}:\d{2}([+-]\d{2}):?(\d{2})/);
  if (!m) return null;
  let h = parseInt(m[1], 10) - parseInt(m[2], 10); // shift by offset
  return (h + 24) % 24;
}

function isWeekend(iso) {
  const d = new Date(iso);
  const day = d.getUTCDay();
  return day === 0 || day === 6;
}

/* ---------- the regret chain ---------- */
/*
 * Walk history forward. Keep "pending suspects" (hacks not yet undone).
 * When a fix commit arrives, attribute it to the most recent pending
 * suspect that shares at least one file. That suspect is consumed, so
 * one fix can't redeem two hacks, and two fixes can't blame one hack twice.
 */
function linkRegrets(commits, windowDays = 90) {
  const pending = [];
  let totalFixes = 0;

  for (const commit of commits) {
    const t = new Date(commit.date).getTime();

    if (commit.kind === "fix") {
      totalFixes++;
      for (let i = pending.length - 1; i >= 0; i--) {
        const s = pending[i];
        const ageDays = (t - new Date(s.date).getTime()) / 864e5;
        if (ageDays > windowDays) continue; // too old to blame? keep waiting
        if (s.files.some((f) => commit.files.includes(f))) {
          s.regrettedBy = {
            hash: commit.hash,
            msg: commit.msg,
            date: commit.date,
            daysLater: Math.round(ageDays),
          };
          pending.splice(i, 1); // consumed
          break;
        }
      }
    } else if (commit.kind === "suspect") {
      pending.push(commit);
    }
  }
  return { totalFixes };
}

/* ---------- scoring ---------- */
function pct(part, whole) {
  return whole === 0 ? 0 : part / whole;
}

function scoreReport(commits) {
  const suspects = commits.filter((k) => k.kind === "suspect");
  const regretted = suspects.filter((s) => s.regrettedBy);
  const late = commits.filter((k) => k.hour !== null && (k.hour >= 22 || k.hour <= 5));
  const weekend = commits.filter((k) => k.weekend);

  const rRegret = pct(regretted.length, suspects.length);
  const rLate = pct(late.length, commits.length);
  const rWeekend = pct(weekend.length, commits.length);
  const rSuspect = pct(suspects.length, commits.length);

  // The Regret Index: proven damage dominates, vibes (when/why you commit) fill in.
  const score = Math.round(
    55 * rRegret + 20 * rSuspect + 15 * rLate + 10 * rWeekend
  );

  return {
    total: commits.length,
    suspects,
    regretted,
    counts: {
      suspects: suspects.length,
      regretted: regretted.length,
      lateNight: late.length,
      weekend: weekend.length,
    },
    rates: { regret: rRegret, suspect: rSuspect, late: rLate, weekend: rWeekend },
    score,
    grade: gradeFor(score),
  };
}

function gradeFor(score) {
  if (score <= 9) return ["S", "Machine-like discipline. Frankly, suspicious."];
  if (score <= 24) return ["A", "Adult supervision detected."];
  if (score <= 39) return ["B", "Functioning adult with occasional 2am episodes."];
  if (score <= 54) return ["C", "Serial quick-fixer. The hacks are load-bearing now."];
  if (score <= 69) return ["D", "Chaos agent. Your future self has PTSD."];
  return ["F", "Hold my beer."];
}

/* ---------- output ---------- */
function bar(ratio, width = 24) {
  const n = Math.round(ratio * width);
  return green("█".repeat(n)) + dim("░".repeat(width - n));
}

function prettyPrint(r) {
  const line = dim("─".repeat(58));
  console.log();
  console.log(bold(magenta("  ╔═╗╦═╗╦═╗╦╦  ╔═╗╦═╗╔═╗╦ ╦╔═╗╦═╗")));
  console.log(bold(magenta("  ║ ║╠╦╝╠╦╝║║  ║ ║╠╦╝║ ╦║ ║╚═╗╠╦╝")));
  console.log(bold(magenta("  ╚═╝╩╚═╩╚═╩═╝╚═╝╩╚═╚═╝╚═╝╚═╝╩╚═")));
  console.log();

  if (r.total < 5) {
    console.log(yellow("  Not enough history to judge you. Go commit something questionable."));
    return;
  }

  console.log(`  ${bold("REGRET INDEX:")} ${bold(red(String(r.score)))}/100   ${bold(`Grade ${red(r.grade[0])}`)}`);
  console.log(`  ${dim(r.grade[1])}`);
  console.log();

  const rows = [
    ["Regret rate (hacks later fixed)", r.rates.regret, `${r.counts.regretted}/${r.counts.suspects} hacks undone`],
    ["Sketchy commit share", r.rates.suspect, `${r.counts.suspects}/${r.total} commits`],
    ["Late-night commits (22:00–05:00)", r.rates.late, `${r.counts.lateNight}/${r.total} commits`],
    ["Weekend commits", r.rates.weekend, `${r.counts.weekend}/${r.total} commits`],
  ];
  for (const [label, ratio, detail] of rows) {
    console.log(`  ${label.padEnd(34)} ${bar(ratio)} ${dim(detail)}`);
  }
  console.log();

  const shamed = r.regretted
    .sort((a, b) => b.regrettedBy.daysLater - a.regrettedBy.daysLater)
    .slice(0, 5);

  if (shamed.length === 0) {
    console.log(green("  ✔ No proven regrets found. Either you're careful or you're lucky. Usually lucky."));
  } else {
    console.log(bold(`  HALL OF SHAME (${shamed.length} proven regrets)`));
    console.log();
    for (const s of shamed) {
      console.log(`  ${red("✗")} ${dim(s.hash)} ${yellow(s.msg)}`);
      console.log(`     ${dim(`undone ${s.regrettedBy.daysLater} days later by ${s.regrettedBy.hash}: "${s.regrettedBy.msg}"`)}`);
    }
  }
  console.log();
  console.log(line);
}

function jsonPrint(r) {
  console.log(
    JSON.stringify(
      {
        regretIndex: r.score,
        grade: r.grade[0],
        verdict: r.grade[1],
        commits: r.total,
        suspects: r.counts.suspects,
        provenRegrets: r.counts.regretted,
        rates: {
          regret: +r.rates.regret.toFixed(3),
          suspect: +r.rates.suspect.toFixed(3),
          lateNight: +r.rates.late.toFixed(3),
          weekend: +r.rates.weekend.toFixed(3),
        },
        hallOfShame: r.regretted.map((s) => ({
          hash: s.hash,
          msg: s.msg,
          date: s.date,
          author: s.author,
          undoneBy: s.regrettedBy.hash,
          undoneMsg: s.regrettedBy.msg,
          daysLater: s.regrettedBy.daysLater,
        })),
      },
      null,
      2
    )
  );
}

/* ---------- CLI ---------- */
function main() {
  const argv = process.argv.slice(2);
  if (argv.includes("--help") || argv.includes("-h")) {
    console.log(`git-regret — prove your hacks were regrets

Usage: git-regret [options]

  --author <name>   Only judge one sinner
  --since <date>    Only look at recent sins (e.g. "1 year ago")
  --json            Output machine-readable shame
  --help            This`);
    return;
  }
  const idx = (flag) => {
    const i = argv.indexOf(flag);
    return i >= 0 && argv[i + 1] ? argv[i + 1] : null;
  };

  const commits = loadHistory(idx("--author"), idx("--since"));
  const { totalFixes } = linkRegrets(commits);
  const report = scoreReport(commits);
  report.totalFixes = totalFixes;

  if (argv.includes("--json")) jsonPrint(report);
  else prettyPrint(report);
}

main();
