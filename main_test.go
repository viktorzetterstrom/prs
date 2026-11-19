package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/viktorzetterstrom/prs/config"
	"github.com/viktorzetterstrom/prs/github"
)

// runCLI runs prs as if stdout were a pipe, which is how an agent calls it.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Cleanup(func() { github.DemoMode = false })
	var out, errOut bytes.Buffer
	err = run(args, &out, &errOut, false)
	return out.String(), errOut.String(), err
}

func TestJSONListsEveryDemoPR(t *testing.T) {
	stdout, _, err := runCLI(t, "--demo", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got listJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout)
	}
	if !reflect.DeepEqual(got.Repos, github.DemoRepos) {
		t.Errorf("repos = %v, want %v", got.Repos, github.DemoRepos)
	}
	if len(got.PRs) != 7 {
		t.Fatalf("expected the 7 demo PRs, got %d", len(got.PRs))
	}
	for _, pr := range got.PRs {
		if pr.Repo == "" || pr.Status == "" || pr.URL == "" {
			t.Errorf("PR #%d is missing repo, status or url: %+v", pr.Number, pr)
		}
		if !strings.HasSuffix(pr.Slack, "("+pr.URL+")") {
			t.Errorf("PR #%d slack line %q does not end with its link", pr.Number, pr.Slack)
		}
	}
	if !strings.Contains(stdout, `"errors": []`) {
		t.Errorf("errors should be an empty array, not null:\n%s", stdout)
	}
}

func TestJSONDoesNotEscapeHTMLCharacters(t *testing.T) {
	var out bytes.Buffer
	prs := []github.PR{{Repo: "a/b", Number: 1, Title: "Fix <b> & co", URL: "u"}}
	if err := writeJSON(&out, []string{"a/b"}, prs, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Fix <b> & co") {
		t.Errorf("title was escaped:\n%s", out.String())
	}
}

func TestWithoutTerminalPrintsPlainList(t *testing.T) {
	stdout, _, err := runCLI(t, "--demo")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 7 {
		t.Fatalf("expected one line per demo PR, got %d:\n%s", len(lines), stdout)
	}
	for _, l := range lines {
		if cols := strings.Split(l, "\t"); len(cols) != 6 {
			t.Errorf("expected 6 tab-separated columns, got %d: %q", len(cols), l)
		}
	}
}

func TestPartialFailureStillPrintsAndFails(t *testing.T) {
	var out bytes.Buffer
	prs := []github.PR{{Repo: "a/ok", Number: 1, URL: "u"}}
	errs := []github.RepoError{{Repo: "a/broken", Err: errors.New("not found")}}
	if err := writeJSON(&out, []string{"a/ok", "a/broken"}, prs, errs); err != nil {
		t.Fatal(err)
	}
	var got listJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.PRs) != 1 || len(got.Errors) != 1 || got.Errors[0].Repo != "a/broken" {
		t.Fatalf("unexpected output: %+v", got)
	}
}

func TestNoReposIsAnError(t *testing.T) {
	var out, errOut bytes.Buffer
	err := printPRs(nil, &out, &errOut, true)
	if err == nil || !strings.Contains(err.Error(), "prs repos add") {
		t.Fatalf("expected a hint to add a repo, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be printed, got %q", out.String())
	}
}

func TestReposAddListRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout, _, err := runCLI(t, "repos", "add", "a/b", "c/d")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "tracking a/b\ntracking c/d\n" {
		t.Errorf("add output = %q", stdout)
	}

	_, stderr, err := runCLI(t, "repos", "add", "A/B")
	if err != nil || !strings.Contains(stderr, "already tracked") {
		t.Errorf("re-adding with other case should be a no-op, got err=%v stderr=%q", err, stderr)
	}

	stdout, _, _ = runCLI(t, "repos")
	if stdout != "a/b\nc/d\n" {
		t.Errorf("list = %q", stdout)
	}

	if _, _, err := runCLI(t, "repos", "rm", "a/b"); err != nil {
		t.Fatal(err)
	}
	_, stderr, err = runCLI(t, "repos", "remove", "a/b")
	if err != nil || !strings.Contains(stderr, "not tracked") {
		t.Errorf("removing an untracked repo should be a no-op, got err=%v stderr=%q", err, stderr)
	}

	cfg, _, _ := config.Load()
	if !reflect.DeepEqual(cfg.Repos, []string{"c/d"}) {
		t.Errorf("config = %v, want [c/d]", cfg.Repos)
	}
}

func TestReposRejectsBadNameAndSavesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _, err := runCLI(t, "repos", "add", "a/b", "not-a-repo")
	if !errors.As(err, new(usageError)) {
		t.Fatalf("expected a usage error, got %v", err)
	}
	if _, found, _ := config.Load(); found {
		t.Error("no config should be written when any name is invalid")
	}
}

func TestReposWithNoneTrackedExplainsFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stdout, stderr, err := runCLI(t, "repos")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" || !strings.Contains(stderr, "no repos tracked") {
		t.Errorf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestReposDemoNeverWritesConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, _, err := runCLI(t, "--demo", "repos", "add", "a/b"); err == nil {
		t.Fatal("expected demo mode to refuse")
	}
	if _, found, _ := config.Load(); found {
		t.Error("demo mode wrote the config")
	}
}

func TestSkillPrintsAndInstalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	stdout, _, err := runCLI(t, "skill")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "---\nname: prs\ndescription: ") {
		t.Errorf("skill should start with name and description frontmatter:\n%.80s", stdout)
	}

	stdout, _, err = runCLI(t, "skill", "install")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "skills", "prs", "SKILL.md")
	if stdout != "installed "+path+"\n" {
		t.Errorf("install output = %q", stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != skillDoc {
		t.Error("installed file differs from the embedded skill")
	}
}

// TestSkillDocumentsTheJSON keeps skill/SKILL.md in step with the output: a
// new field has to appear in its example, and a new status label has to be
// explained.
func TestSkillDocumentsTheJSON(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(listJSON{}), reflect.TypeOf(prJSON{})} {
		for i := range typ.NumField() {
			tag := typ.Field(i).Tag.Get("json")
			if !strings.Contains(skillDoc, `"`+tag+`":`) {
				t.Errorf("the example in skill/SKILL.md has no %q field", tag)
			}
		}
	}

	github.DemoMode = true
	t.Cleanup(func() { github.DemoMode = false })
	prs, _ := github.GetPRs(github.DemoRepos)
	for _, pr := range prs {
		if !strings.Contains(skillDoc, "`"+pr.Status()+"`") {
			t.Errorf("skill/SKILL.md does not describe the %q status", pr.Status())
		}
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	for _, args := range [][]string{{"nope"}, {"repos", "nope"}, {"repos", "add"}, {"skill", "nope"}} {
		if _, _, err := runCLI(t, args...); !errors.As(err, new(usageError)) {
			t.Errorf("%v: expected a usage error, got %v", args, err)
		}
	}
}
