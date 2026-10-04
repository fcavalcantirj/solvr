package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// solvr 0.2.0 removed the 0.1 choices of the legacy knowledge model: a post has
// no type, every contribution is a reply, a post's replies are read on their own,
// and search covers every post. A removed command, argument or flag still parses,
// so using it fails before any request, naming what replaces it and where the
// migration notes are (solvr help migrating).

// The 0.1 surface 0.2.0 removed (git show 1ed017b6^:cli/): the name the error and
// the notes give it, and a 0.1 invocation.
var removedIn02 = []struct {
	name string
	args []string
}{
	{"solvr answer", []string{"answer", "post-1", "--content", "Use a mutex"}},
	{"solvr answer", []string{"answer", "post-1", "--editor"}},
	{"solvr post <type>", []string{"post", "problem", "--title", "Race in the pool", "--description", "Two workers"}},
	{"--include", []string{"get", "post-1", "--include", "approaches,answers"}},
	{"--type", []string{"search", "pool race", "--type", "problem"}},
}

func migrationNotes(t *testing.T) string {
	t.Helper()
	isolateHome(t)
	res := runCLI(t, "help", "migrating")
	if res.code != 0 {
		t.Fatalf("solvr help migrating: exit %d, stderr %q", res.code, res.stderr)
	}
	return res.stdout
}

func TestRemovedChoices_FailBeforeAnyRequestAndPointAtTheNotes(t *testing.T) {
	for _, removed := range removedIn02 {
		t.Run(strings.Join(removed.args, " "), func(t *testing.T) {
			srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
				answerJSON(w, 201, `{"data":{"id":"should-not-be-called"}}`)
			})
			writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

			for _, args := range [][]string{removed.args, append(append([]string{}, removed.args...), "--json")} {
				res := runCLI(t, args...)
				if res.code != 1 || res.stdout != "" {
					t.Errorf("solvr %s: exit %d, stdout %q", strings.Join(args, " "), res.code, res.stdout)
				}
				want := "'" + removed.name + "' was removed in solvr " + Version
				if !strings.Contains(res.stderr, want) || !strings.Contains(res.stderr, "solvr help migrating") {
					t.Errorf("solvr %s: stderr %q, want %q and the notes", strings.Join(args, " "), res.stderr, want)
				}
				if strings.Contains(res.stderr, "for usage") {
					t.Errorf("solvr %s: stderr %q points at --help, not the notes", strings.Join(args, " "), res.stderr)
				}
			}
			if n := len(srv.requests()); n != 0 {
				t.Errorf("sent %d requests, want 0", n)
			}
		})
	}
}

func TestRemovedPostType_NamesTheRefusedType(t *testing.T) {
	isolateHome(t)
	res := runCLI(t, "post", "idea", "--api-url", unreachableAPI, "--title", "T", "--description", "D")
	if res.code != 1 || !strings.Contains(res.stderr, `"idea" is not accepted`) {
		t.Errorf("exit %d, stderr %q", res.code, res.stderr)
	}
}

func TestSearch_StillRunsWithItsOptionsAndSendsNoType(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":[],"meta":{"query":"pool race","total":0}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	if res := runCLI(t, "search", "pool race", "--limit", "5", "--page", "2", "--sort", "newest"); res.code != 0 {
		t.Fatalf("exit %d, stderr %q", res.code, res.stderr)
	}
	q := srv.requests()[0].query
	if len(q) != 4 || q.Get("q") != "pool race" || q.Get("per_page") != "5" || q.Get("page") != "2" || q.Get("sort") != "newest" {
		t.Errorf("search sent %v", q)
	}
}

func TestHelp_ListsNoRemovedCommandOrFlag(t *testing.T) {
	isolateHome(t)
	root := runCLI(t, "--help").stdout
	if regexp.MustCompile(`(?m)^\s+answer\b`).MatchString(root) {
		t.Errorf("solvr --help lists answer:\n%s", root)
	}
	if !regexp.MustCompile(`(?m)^\s+solvr migrating\s`).MatchString(root) {
		t.Errorf("solvr --help does not list the migrating help topic:\n%s", root)
	}
	for cmd, flag := range map[string]string{"search": "--type", "get": "--include"} {
		if help := runCLI(t, cmd, "--help").stdout; strings.Contains(help, flag) {
			t.Errorf("solvr %s --help lists %s:\n%s", cmd, flag, help)
		}
	}
	if help := runCLI(t, "post", "--help").stdout; strings.Contains(help, "[type]") {
		t.Errorf("solvr post --help offers a type:\n%s", help)
	}
}

func TestVersion_IsTheOneTheMigrationNotesDescribe(t *testing.T) {
	if Version != "0.2.0" {
		t.Errorf("Version %q, want 0.2.0", Version)
	}
	isolateHome(t)
	if out := runCLI(t, "--version").stdout; strings.TrimSpace(out) != "solvr version "+Version {
		t.Errorf("--version printed %q", out)
	}
	if notes := migrationNotes(t); !strings.HasPrefix(strings.TrimSpace(notes), "Migrating from 0.1 to "+Version) {
		t.Errorf("the notes do not start with their heading:\n%s", notes)
	}
}

func TestMigrationNotes_NameEveryRemovedChoiceAndWhatReplacesIt(t *testing.T) {
	notes := migrationNotes(t)
	for _, removed := range removedIn02 {
		if !strings.Contains(notes, removed.name) {
			t.Errorf("the notes do not name %s", removed.name)
		}
	}
	for _, use := range []string{"ENDPOINT_RETIRED", "details.replacement"} {
		if !strings.Contains(notes, use) {
			t.Errorf("the notes do not name %s", use)
		}
	}

	// Every command the notes point to, other than the removed ones, is a 0.2.0 command.
	commands := map[string]bool{}
	for _, c := range NewRootCmd().Commands() {
		if !c.Hidden {
			commands[c.Name()] = true
		}
	}
	named := map[string]bool{}
	for _, m := range regexp.MustCompile(`solvr ([a-z-]+)`).FindAllStringSubmatch(notes, -1) {
		named[m[1]] = true
	}
	for _, want := range []string{"reply", "replies", "post", "search", "get"} {
		if !named[want] {
			t.Errorf("the notes do not point to solvr %s", want)
		}
	}
	for command := range named {
		if command != "answer" && command != "help" && !commands[command] {
			t.Errorf("the notes point to solvr %s, which is not a command", command)
		}
	}
}
