package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.local")
	run("config", "user.name", "test")
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644)
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func modify(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveListExactBranchLabel(t *testing.T) {
	dir := gitInit(t)
	modify(t, dir, "base.txt", "wip on main\n")
	Save(dir, "my work", false)

	entries, err := StashList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(entries))
	}
	e := entries[0]
	if e.Index != 0 || e.Ref != "stash@{0}" {
		t.Fatalf("bad ref: %+v", e)
	}
	if e.Source != SourceExact || e.Branch != "main" {
		t.Fatalf("expected exact main label: %+v", e)
	}
	if !strings.Contains(e.Message, "my work") {
		t.Fatalf("message lost: %+v", e)
	}
}

func TestInferredBranchLabel(t *testing.T) {
	dir := gitInit(t)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	modify(t, dir, "base.txt", "feature wip\n")
	Save(dir, "on main first", false)
	run("checkout", "-b", "feature")
	modify(t, dir, "other.txt", "x\n")
	run("add", ".")
	run("commit", "-m", "feat commit")

	modify(t, dir, "base.txt", "feature wip v2\n")
	hashOut, err := exec.Command("git", "-C", dir, "stash", "create").Output()
	if err != nil {
		t.Fatalf("stash create failed: %v", err)
	}
	run("stash", "store", "-m", "plain push", strings.TrimSpace(string(hashOut)))

	entries, err := StashList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(entries))
	}
	var plain *Entry
	for i := range entries {
		if entries[i].Source == SourceInferred {
			plain = &entries[i]
		}
	}
	if plain == nil {
		t.Fatalf("no inferred entry: %+v", entries)
	}
	if plain.Branch != "feature" {
		t.Fatalf("unexpected inferred branch: %+v", plain)
	}

	filtered := FilterByCurrent(entries, "feature")
	found := false
	for _, e := range filtered {
		if e.Ref == plain.Ref {
			found = true
		}
	}
	if !found {
		t.Fatalf("filter should keep inferred current-branch stash: %+v", filtered)
	}
	filtered = FilterByCurrent(entries, "nonexistent-branch")
	if len(filtered) != 0 {
		t.Fatalf("filter should exclude other branches: %+v", filtered)
	}
}

func TestApplyPopDropRoundtrip(t *testing.T) {
	dir := gitInit(t)
	modify(t, dir, "new.txt", "stashed content\n")
	if _, err := Save(dir, "add new file", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("file should be stashed away")
	}

	out, err := Apply(dir, "stash@{0}")
	if err != nil {
		t.Fatalf("apply failed: %v %s", err, out)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if string(b) != "stashed content\n" {
		t.Fatalf("apply restored wrong content: %q", b)
	}

	Drop(dir, "stash@{0}")
	entries, _ := StashList(dir)
	if len(entries) != 0 {
		t.Fatalf("stash not dropped: %+v", entries)
	}

	os.Remove(filepath.Join(dir, "new.txt"))
	modify(t, dir, "base.txt", "changed for pop\n")
	if _, err := Save(dir, "pop me", true); err != nil {
		t.Fatal(err)
	}
	out, err = Pop(dir, "stash@{0}")
	if err != nil {
		t.Fatalf("pop failed: %v %s", err, out)
	}
	entries, _ = StashList(dir)
	if len(entries) != 0 {
		t.Fatalf("stash not popped: %+v", entries)
	}
	b, err = os.ReadFile(filepath.Join(dir, "base.txt"))
	if err != nil || string(b) != "changed for pop\n" {
		t.Fatalf("popped content wrong: %q %v", b, err)
	}
}

func TestBranchFromStash(t *testing.T) {
	dir := gitInit(t)
	modify(t, dir, "base.txt", "wip\n")
	Save(dir, "wip work", false)
	out, err := BranchFromStash(dir, "stash@{0}", "stashwork")
	if err != nil {
		t.Fatalf("stash branch failed: %v %s", err, out)
	}
	cur, err := CurrentBranch(dir)
	if err != nil || cur != "stashwork" {
		t.Fatalf("expected branch stashwork, got %q %v", cur, err)
	}
}
