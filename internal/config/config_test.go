package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// makeFakeGitRepo creates a temp directory with a .git subdirectory,
// changes the working directory into subdir (relative to root), and registers
// a cleanup to restore the original directory.
func makeFakeGitRepo(t *testing.T, subdir string) (root string) {
	t.Helper()
	root = t.TempDir()

	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, subdir)
	if subdir != "" {
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
	} else {
		target = root
	}

	original, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(original) })

	if err := os.Chdir(target); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeYAML writes content to <dir>/.gtc.yaml with mode 0600.
func writeYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".gtc.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// LoadFrom
// ---------------------------------------------------------------------------

func TestLoadFrom_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
provider:
  name: gitlab
  base_url: https://gitlab.example.com
  token: glpat-secret
  owner: mygroup
  repo: myrepo
watch:
  interval: 30s
  events:
    - pr_opened
    - ci_failed
`)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: unexpected error: %v", err)
	}

	if cfg.Provider.Name != "gitlab" {
		t.Errorf("Provider.Name: got %q, want %q", cfg.Provider.Name, "gitlab")
	}
	if cfg.Provider.BaseURL != "https://gitlab.example.com" {
		t.Errorf("Provider.BaseURL: got %q", cfg.Provider.BaseURL)
	}
	if cfg.Provider.Token != "glpat-secret" {
		t.Errorf("Provider.Token: got %q", cfg.Provider.Token)
	}
	if cfg.Provider.Owner != "mygroup" {
		t.Errorf("Provider.Owner: got %q", cfg.Provider.Owner)
	}
	if cfg.Provider.Repo != "myrepo" {
		t.Errorf("Provider.Repo: got %q", cfg.Provider.Repo)
	}
	if cfg.Watch.Interval != "30s" {
		t.Errorf("Watch.Interval: got %q, want %q", cfg.Watch.Interval, "30s")
	}
	if len(cfg.Watch.Events) != 2 {
		t.Fatalf("Watch.Events len: got %d, want 2", len(cfg.Watch.Events))
	}
	if cfg.Watch.Events[0] != "pr_opened" {
		t.Errorf("Watch.Events[0]: got %q", cfg.Watch.Events[0])
	}
}

func TestLoadFrom_MinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
provider:
  name: github
  token: ghp-token
  owner: alice
  repo: myapp
watch:
  interval: 1m
`)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: unexpected error: %v", err)
	}

	if cfg.Provider.BaseURL != "" {
		t.Errorf("Provider.BaseURL: expected empty, got %q", cfg.Provider.BaseURL)
	}
	if cfg.Watch.Events != nil {
		t.Errorf("Watch.Events: expected nil, got %v", cfg.Watch.Events)
	}
}

func TestLoadFrom_FileNotFound(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/.gtc.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `provider: [broken: yaml: :::`)

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadFrom_UnknownField(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
provider:
  name: github
  token: tok
  owner: alice
  repo: r
  unknown_field: oops
watch:
  interval: 1m
`)

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for unknown YAML field (KnownFields=true), got nil")
	}
}

// ---------------------------------------------------------------------------
// Load (via FindGitRoot)
// ---------------------------------------------------------------------------

func TestLoad_FindsConfigAtGitRoot(t *testing.T) {
	root := makeFakeGitRepo(t, "sub/deep")

	writeYAML(t, root, `
provider:
  name: github
  token: tok
  owner: org
  repo: rep
watch:
  interval: 1m
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Provider.Name != "github" {
		t.Errorf("Provider.Name: got %q, want %q", cfg.Provider.Name, "github")
	}
}

func TestLoad_MissingConfig(t *testing.T) {
	makeFakeGitRepo(t, "")
	// No .gtc.yaml written — Load should return a descriptive error.
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when .gtc.yaml is absent, got nil")
	}
}

func TestLoad_NotInGitRepo(t *testing.T) {
	dir := t.TempDir() // no .git here
	original, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(original) })
	_ = os.Chdir(dir)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

// ---------------------------------------------------------------------------
// parseRemoteURL
// ---------------------------------------------------------------------------

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantHost  string
	}{
		{
			name:      "SSH github",
			input:     "git@github.com:lucasvavon/gtc.git",
			wantOwner: "lucasvavon",
			wantRepo:  "gtc",
			wantHost:  "github.com",
		},
		{
			name:      "HTTPS gitlab",
			input:     "https://gitlab.com/lucasvavon/gtc.git",
			wantOwner: "lucasvavon",
			wantRepo:  "gtc",
			wantHost:  "gitlab.com",
		},
		{
			name:      "SSH self-hosted gitlab",
			input:     "git@gitlab.mycompany.com:team/project.git",
			wantOwner: "team",
			wantRepo:  "project",
			wantHost:  "gitlab.mycompany.com",
		},
		{
			name:      "HTTPS without .git suffix",
			input:     "https://github.com/lucasvavon/gtc",
			wantOwner: "lucasvavon",
			wantRepo:  "gtc",
			wantHost:  "github.com",
		},
		{
			name:      "SSH bitbucket",
			input:     "git@bitbucket.org:atlassian/python-bitbucket.git",
			wantOwner: "atlassian",
			wantRepo:  "python-bitbucket",
			wantHost:  "bitbucket.org",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, host, err := parseRemoteURL(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tc.wantOwner {
				t.Errorf("owner: got %q, want %q", owner, tc.wantOwner)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tc.wantRepo)
			}
			if host != tc.wantHost {
				t.Errorf("host: got %q, want %q", host, tc.wantHost)
			}
		})
	}
}

func TestParseRemoteURL_Errors(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"unsupported scheme", "ssh://git@github.com/owner/repo.git"},
		{"SSH missing colon", "git@github.com/owner/repo.git"},
		{"SSH missing repo", "git@github.com:owner"},
		{"HTTPS missing repo", "https://github.com/owner"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parseRemoteURL(tc.input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// guessProvider
// ---------------------------------------------------------------------------

func TestGuessProvider(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"github.com", "github"},
		{"gitlab.com", "gitlab"},
		{"bitbucket.org", "bitbucket"},
		{"gitlab.mycompany.com", ""},
		{"git.internal", ""},
		{"", ""},
	}

	for _, tc := range cases {
		got := guessProvider(tc.host)
		if got != tc.want {
			t.Errorf("guessProvider(%q): got %q, want %q", tc.host, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// FindGitRoot
// ---------------------------------------------------------------------------

func TestFindGitRoot_Found(t *testing.T) {
	root := makeFakeGitRepo(t, "sub/sub2")

	got, err := FindGitRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)

	if gotResolved != wantResolved {
		t.Errorf("FindGitRoot(): got %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindGitRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	original, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(original) })
	_ = os.Chdir(dir)

	_, err := FindGitRoot()
	if err == nil {
		t.Error("expected error when no .git directory exists, got nil")
	}
}

// ---------------------------------------------------------------------------
// RunInteractiveInit
// ---------------------------------------------------------------------------

func TestRunInteractiveInit_WritesConfig(t *testing.T) {
	root := makeFakeGitRepo(t, "")

	// Simulate user typing: provider, base_url, owner, repo, token, interval
	input := strings.NewReader("gitlab\nhttps://gitlab.example.com\nalice\nmyrepo\nglpat-abc\n2m\n")
	var out strings.Builder

	if err := RunInteractiveInit(input, &out, false); err != nil {
		t.Fatalf("RunInteractiveInit: unexpected error: %v", err)
	}

	// Verify the file was created and is parseable.
	cfg, err := LoadFrom(filepath.Join(root, ".gtc.yaml"))
	if err != nil {
		t.Fatalf("LoadFrom after init: %v", err)
	}

	if cfg.Provider.Name != "gitlab" {
		t.Errorf("Provider.Name: got %q, want %q", cfg.Provider.Name, "gitlab")
	}
	if cfg.Provider.Owner != "alice" {
		t.Errorf("Provider.Owner: got %q, want %q", cfg.Provider.Owner, "alice")
	}
	if cfg.Provider.Token != "glpat-abc" {
		t.Errorf("Provider.Token: got %q, want %q", cfg.Provider.Token, "glpat-abc")
	}
	if cfg.Watch.Interval != "2m" {
		t.Errorf("Watch.Interval: got %q, want %q", cfg.Watch.Interval, "2m")
	}
}

func TestRunInteractiveInit_NotInGitRepo(t *testing.T) {
	dir := t.TempDir()
	original, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(original) })
	_ = os.Chdir(dir)

	err := RunInteractiveInit(strings.NewReader(""), &strings.Builder{}, false)
	if err == nil {
		t.Fatal("expected error when not in a git repository, got nil")
	}
}

func TestRunInteractiveInit_GitIgnoreReminder(t *testing.T) {
	makeFakeGitRepo(t, "")

	input := strings.NewReader("github\nalice\nmyrepo\nghp-tok\n1m\n")
	var out strings.Builder

	if err := RunInteractiveInit(input, &out, false); err != nil {
		t.Fatalf("RunInteractiveInit: unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), ".gitignore") {
		t.Errorf("expected .gitignore reminder in output, got:\n%s", out.String())
	}
}

func TestRunInteractiveInit_FilePermissions(t *testing.T) {
	root := makeFakeGitRepo(t, "")

	input := strings.NewReader("github\nalice\nrepo\ntok\n1m\n")
	if err := RunInteractiveInit(input, &strings.Builder{}, false); err != nil {
		t.Fatalf("RunInteractiveInit: %v", err)
	}

	info, err := os.Stat(filepath.Join(root, ".gtc.yaml"))
	if err != nil {
		t.Fatalf("stat .gtc.yaml: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf(".gtc.yaml permissions: got %04o, want 0600", perm)
	}
}
