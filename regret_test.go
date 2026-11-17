package main

import (
	"testing"

	"github.com/Arch087146/git-regret/internal/regret"
)

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"quick fix lol":            "suspect",
		"hacky workaround":         "suspect",
		"whoops actually fix auth": "suspect",
		"fix parser security hole": "fix",
		"revert bad commit":        "fix",
		"clean feature work":       "normal",
	}
	for msg, want := range cases {
		if got := regret.Classify(msg); got != want {
			t.Errorf("Classify(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestLinkRegrets(t *testing.T) {
	commits := []regret.Commit{
		{Hash: "aaa0001", Msg: "initial setup", Kind: "normal", Files: []string{"main.js"}, Date: "2024-03-03T14:00:00+03:00"},
		{Hash: "aaa0002", Msg: "quick fix lol", Kind: "suspect", Files: []string{"auth.js"}, Date: "2024-03-04T11:00:00+03:00"},
		{Hash: "aaa0003", Msg: "fix auth bypass", Kind: "fix", Files: []string{"auth.js"}, Date: "2024-03-25T10:00:00+03:00"},
	}
	regret.LinkRegrets(commits, 90)
	rep := regret.Score(commits)
	if rep.Suspects != 1 || rep.Proven != 1 {
		t.Fatalf("want 1 suspect 1 proven, got %d/%d", rep.Proven, rep.Suspects)
	}
	if rep.Hall[0].DaysLater != 20 {
		t.Fatalf("want 20 days later, got %d", rep.Hall[0].DaysLater)
	}
}

func TestOneFixRedeemsOneSuspect(t *testing.T) {
	commits := []regret.Commit{
		{Hash: "bbb0001", Msg: "quick fix lol", Kind: "suspect", Files: []string{"a.js"}, Date: "2024-03-04T11:00:00+03:00"},
		{Hash: "bbb0002", Msg: "temporary hack", Kind: "suspect", Files: []string{"a.js"}, Date: "2024-03-05T11:00:00+03:00"},
		{Hash: "bbb0003", Msg: "fix it properly", Kind: "fix", Files: []string{"a.js"}, Date: "2024-03-20T11:00:00+03:00"},
	}
	regret.LinkRegrets(commits, 90)
	rep := regret.Score(commits)
	if rep.Proven != 1 {
		t.Fatalf("one fix must redeem exactly one suspect, got %d", rep.Proven)
	}
}

func TestGrades(t *testing.T) {
	if g, _ := regret.Grade(0); g != "S" {
		t.Fatal("0 should be grade S")
	}
	if g, _ := regret.Grade(100); g != "F" {
		t.Fatal("100 should be grade F")
	}
}
