// Package regret — git history analysis and the regret engine.
package regret

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var suspectRE = regexp.MustCompile(`(?i)(quick ?fix|hotfix|temp(orary)?|hack(y|ed)?|work ?around|wip|do ?not (look|review|ship|deploy)|don'?t (look|ask)|sorry|oops|whoops|oopsie|damn|dammit|argh|ugh|fix ?later|later|band-?aid|kludge|jank|test commit|asdf|debug|why is this|please work|final fix|should work now|actually fix|for real this time)`)

var fixRE = regexp.MustCompile(`(?i)^(fix|fixes|fixed|bugfix|bug ?fix|hotfix|revert|rollback|undo|repair|patch)([:( ]|$)`)

// Commit is one parsed git commit.
type Commit struct {
	Hash    string
	Author  string
	Email   string
	Date    string // ISO-8601 with offset
	Msg     string
	Files   []string
	Hour    int  // hour in the commit's own timezone
	Weekend bool
	Kind    string // "suspect" | "fix" | "normal"
	UndoneBy *Fix  // set when proven regretted
}

// Fix is the commit that undid a suspect.
type Fix struct {
	Hash      string
	Msg       string
	DaysLater int
}

// Classify: a suspect message wins over a fix message (both = sketchy "fix").
func Classify(msg string) string {
	if suspectRE.MatchString(msg) {
		return "suspect"
	}
	if fixRE.MatchString(msg) {
		return "fix"
	}
	return "normal"
}

const logFormat = "@@@%H|%an|%ae|%ad|%s"

// LoadHistory returns commits oldest-first with their touched files.
func LoadHistory(root, author, since string) ([]Commit, error) {
	args := []string{"log", "--no-merges", "--pretty=format:" + logFormat, "--date=iso8601-strict", "--name-only"}
	if author != "" {
		args = append(args, "--author="+author)
	}
	if since != "" {
		args = append(args, "--since="+since)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository (or git is missing)")
	}

	var commits []Commit
	var cur *Commit
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "@@@") {
			if cur != nil {
				commits = append(commits, *cur)
			}
			parts := strings.SplitN(line[3:], "|", 5)
			if len(parts) < 5 {
				continue
			}
			date := parts[3]
			h, wk := parseDate(date)
			cur = &Commit{
				Hash:    short(parts[0]),
				Author:  parts[1],
				Email:   parts[2],
				Date:    date,
				Msg:     parts[4],
				Hour:    h,
				Weekend: wk,
				Kind:    Classify(parts[4]),
			}
		} else if cur != nil && strings.TrimSpace(line) != "" {
			cur.Files = append(cur.Files, strings.TrimSpace(line))
		}
	}
	if cur != nil {
		commits = append(commits, *cur)
	}
	// git prints newest first; reverse to oldest-first
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// parseDate extracts the commit's local hour and weekend flag from
// "2024-03-09T02:30:00+02:00".
func parseDate(iso string) (int, bool) {
	t, err := time.Parse("2006-01-02T15:04:05Z07:00", iso)
	if err != nil {
		return -1, false
	}
	return t.Hour(), t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

// LinkRegrets walks history forward, attributing each fix commit to the most
// recent pending suspect sharing a file. One fix redeems exactly one suspect.
func LinkRegrets(commits []Commit, windowDays int) {
	pending := []*Commit{}
	for i := range commits {
		c := &commits[i]
		t, err1 := time.Parse("2006-01-02T15:04:05Z07:00", c.Date)
		if c.Kind == "fix" {
			for j := len(pending) - 1; j >= 0; j-- {
				s := pending[j]
				st, err2 := time.Parse("2006-01-02T15:04:05Z07:00", s.Date)
				if err1 != nil || err2 != nil {
					continue
				}
				age := t.Sub(st).Hours() / 24
				if age > float64(windowDays) {
					continue
				}
				if sharesFile(s, c) {
					s.UndoneBy = &Fix{Hash: c.Hash, Msg: c.Msg, DaysLater: int(age)}
					pending = append(pending[:j], pending[j+1:]...)
					break
				}
			}
		} else if c.Kind == "suspect" {
			pending = append(pending, c)
		}
	}
}

func sharesFile(a, b *Commit) bool {
	for _, f := range a.Files {
		for _, g := range b.Files {
			if f == g {
				return true
			}
		}
	}
	return false
}

// Report is the scored outcome.
type Report struct {
	Score    int      `json:"regretIndex"`
	Grade    string   `json:"grade"`
	Verdict  string   `json:"verdict"`
	Total    int      `json:"commits"`
	Suspects int      `json:"suspects"`
	Proven   int      `json:"provenRegrets"`
	LateNight int     `json:"lateNight"`
	Weekend   int     `json:"weekend"`
	RegretRate float64 `json:"regretRate"`
	Hall     []Shame  `json:"hallOfShame"`
}

// Shame is one proven regret for display.
type Shame struct {
	Hash      string
	Msg       string
	Date      string
	UndoneBy  string
	UndoneMsg string
	DaysLater int
}

func pct(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole)
}

// Score turns classified commits into a report. Expects LinkRegrets already run.
func Score(commits []Commit) Report {
	var suspects, proven, late, weekend int
	for i := range commits {
		c := &commits[i]
		if c.Kind == "suspect" {
			suspects++
			if c.UndoneBy != nil {
				proven++
			}
		}
		if c.Hour >= 22 || c.Hour <= 5 {
			late++
		}
		if c.Weekend {
			weekend++
		}
	}
	score := int(55*pct(proven, suspects) + 20*pct(suspects, len(commits)) + 15*pct(late, len(commits)) + 10*pct(weekend, len(commits)))
	grade, verdict := Grade(score)

	hall := []Shame{}
	for i := range commits {
		c := &commits[i]
		if c.Kind == "suspect" && c.UndoneBy != nil {
			hall = append(hall, Shame{
				Hash: c.Hash, Msg: c.Msg, Date: c.Date,
				UndoneBy: c.UndoneBy.Hash, UndoneMsg: c.UndoneBy.Msg, DaysLater: c.UndoneBy.DaysLater,
			})
		}
	}
	// most patient regrets first (longest time until someone fixed it)
	for i := 0; i < len(hall); i++ {
		for j := i + 1; j < len(hall); j++ {
			if hall[j].DaysLater > hall[i].DaysLater {
				hall[i], hall[j] = hall[j], hall[i]
			}
		}
	}
	return Report{
		Score: score, Grade: grade, Verdict: verdict, Total: len(commits),
		Suspects: suspects, Proven: proven, LateNight: late, Weekend: weekend,
		RegretRate: pct(proven, suspects), Hall: hall,
	}
}

// Grade maps a 0-100 score to a verdict.
func Grade(score int) (string, string) {
	switch {
	case score <= 9:
		return "S", "Machine-like discipline. Frankly, suspicious."
	case score <= 24:
		return "A", "Adult supervision detected."
	case score <= 39:
		return "B", "Functioning adult with occasional 2am episodes."
	case score <= 54:
		return "C", "Serial quick-fixer. The hacks are load-bearing now."
	case score <= 69:
		return "D", "Chaos agent. Your future self has PTSD."
	default:
		return "F", "Hold my beer."
	}
}

// Bar renders a meter row.
func Bar(label string, ratio float64, detail string) string {
	w := 24
	filled := int(ratio*float64(w) + 0.5)
	if filled > w {
		filled = w
	}
	return fmt.Sprintf("  %-34s %s%s %s", label,
		"\x1b[32m"+strings.Repeat("█", filled)+"\x1b[0m",
		"\x1b[2m"+strings.Repeat("░", w-filled)+"\x1b[0m",
		"\x1b[2m"+detail+"\x1b[0m")
}
