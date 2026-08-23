package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	Index    int    `json:"index"`
	Ref      string `json:"ref"`
	Message  string `json:"message"`
	BaseHash string `json:"base_hash"`
	Branch   string `json:"branch"`
	Source   string `json:"source"`
	Age      string `json:"age"`
}

const (
	SourceExact    = "exact"
	SourceInferred = "inferred"
	SourceUnknown  = "unknown"
)

var branchPrefixRe = regexp.MustCompile(`^\[branch:([^\]]+)\]\s*`)
var reflogBranchRe = regexp.MustCompile(`^(?:On|WIP) (.+?):`)

func Run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	text := strings.TrimRight(out.String(), "\n")
	if err != nil {
		if ctx.Err() != nil {
			return text, fmt.Errorf("git %s: timed out", strings.Join(args, " "))
		}
		return text, fmt.Errorf("git %s: %s", strings.Join(args, " "), text)
	}
	return text, nil
}

func IsRepo(dir string) bool {
	_, err := Run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func CurrentBranch(dir string) (string, error) {
	out, err := Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "HEAD" {
		return "", nil
	}
	return b, nil
}

func StashList(dir string) ([]Entry, error) {
	out, err := Run(dir, "stash", "list", "--format=%gd%x1f%gs%x1f%H%x1f%P%x1f%ar")
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	if strings.TrimSpace(out) == "" {
		return entries, nil
	}
	cur, _ := CurrentBranch(dir)
	branchCache := map[string]string{}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) < 4 {
			continue
		}
		ref := parts[0]
		idx, perr := stashIndex(ref)
		if perr != nil {
			continue
		}
		subject := parts[1]
		parents := strings.Fields(parts[3])
		base := ""
		if len(parents) > 0 {
			base = parents[0]
		}
		age := ""
		if len(parts) > 4 {
			age = strings.TrimSpace(parts[4])
		}
		e := Entry{
			Index: idx, Ref: ref,
			Message:  cleanSubject(subject),
			BaseHash: base,
			Age:      age,
			Source:   SourceUnknown,
		}
		if m := reflogBranchRe.FindStringSubmatch(subject); m != nil {
			b := strings.TrimSpace(m[1])
			if b != "" && b != "(no branch)" {
				e.Branch = b
				e.Source = SourceExact
			}
		}
		if e.Source == SourceUnknown {
			if m := branchPrefixRe.FindStringSubmatch(subject); m != nil {
				e.Branch = m[1]
				e.Source = SourceExact
			} else if label, ok := inferBranch(dir, base, cur, branchCache); ok {
				e.Branch = label
				e.Source = SourceInferred
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func stashIndex(ref string) (int, error) {
	s := strings.TrimPrefix(ref, "stash@{")
	s = strings.TrimSuffix(s, "}")
	return strconv.Atoi(s)
}

func cleanSubject(s string) string {
	if strings.HasPrefix(s, "On ") {
		if i := strings.Index(s, ": "); i >= 0 {
			return s[i+2:]
		}
	}
	return s
}

func inferBranch(dir, baseHash, cur string, cache map[string]string) (string, bool) {
	if baseHash == "" {
		return "", false
	}
	if v, ok := cache[baseHash]; ok {
		return v, v != ""
	}
	out, err := Run(dir, "branch", "--contains", baseHash, "--format=%(refname:short)")
	if err != nil {
		cache[baseHash] = ""
		return "", false
	}
	lines := []string{}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(strings.TrimPrefix(l, "* "))
		if l != "" {
			lines = append(lines, l)
		}
	}
	sort.Strings(lines)
	label := ""
	for _, l := range lines {
		if l == cur {
			label = cur
			break
		}
	}
	if label == "" && len(lines) > 0 {
		label = lines[0]
	}
	cache[baseHash] = label
	return label, label != ""
}

func FilterByCurrent(entries []Entry, cur string) []Entry {
	out := []Entry{}
	for _, e := range entries {
		if e.Branch == cur || e.Source == SourceUnknown {
			out = append(out, e)
		}
	}
	return out
}

func Show(dir, ref string) (string, error) {
	return Run(dir, "stash", "show", "-p", "--stat", ref)
}

func Apply(dir, ref string) (string, error) {
	return Run(dir, "stash", "apply", ref)
}

func Pop(dir, ref string) (string, error) {
	return Run(dir, "stash", "pop", ref)
}

func Drop(dir, ref string) (string, error) {
	return Run(dir, "stash", "drop", ref)
}

func BranchFromStash(dir, ref, name string) (string, error) {
	return Run(dir, "stash", "branch", name, ref)
}

func Save(dir, msg string, includeUntracked bool) (string, error) {
	cur, _ := CurrentBranch(dir)
	full := msg
	if cur != "" {
		full = fmt.Sprintf("[branch:%s] %s", cur, msg)
	}
	args := []string{"stash", "push", "-m", full}
	if includeUntracked {
		args = append(args, "-u")
	}
	return Run(dir, args...)
}
