package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v60/github"
	"github.com/lucasvavon/gtc/internal/models"
	"github.com/lucasvavon/gtc/internal/provider"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

const (
	testOwner = "owner"
	testRepo  = "repo"
)

// newTestServer starts an httptest.Server backed by handler and returns a
// *GitHub client wired to it with owner/repo pre-configured.
// The server is stopped automatically via t.Cleanup.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *GitHub) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	baseURL, err := parseBaseURL(srv.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}

	client := gh.NewClient(nil)
	client.BaseURL = baseURL

	return srv, newWithClient(client, provider.ProviderConfig{
		Token: "test-token",
		Owner: testOwner,
		Repo:  testRepo,
	})
}

// writeJSON writes v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// repoPath returns the base API path for the test owner/repo.
func repoPath() string {
	return fmt.Sprintf("/repos/%s/%s", testOwner, testRepo)
}

// fixedTime returns a deterministic UTC time for use in test fixtures.
func fixedTime() time.Time {
	return time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
}

// prJSON builds a minimal GitHub pull request JSON object.
// Optional fields can be overridden by merging into the returned map.
func prFixture(number int, title, state, author string, extra map[string]any) map[string]any {
	m := map[string]any{
		"number":     number,
		"title":      title,
		"state":      state,
		"draft":      false,
		"html_url":   fmt.Sprintf("https://github.com/%s/%s/pull/%d", testOwner, testRepo, number),
		"created_at": fixedTime().Format(time.RFC3339),
		"updated_at": fixedTime().Format(time.RFC3339),
		"user":       map[string]any{"login": author},
		"head":       map[string]any{"ref": "feat/branch"},
		"base":       map[string]any{"ref": "main"},
		"labels":     []any{},
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// branchFixture builds a minimal GitHub branch JSON object.
func branchFixture(name, sha string, protected bool) map[string]any {
	return map[string]any{
		"name":      name,
		"protected": protected,
		"commit":    map[string]any{"sha": sha},
	}
}

// ---------------------------------------------------------------------------
// Authenticate
// ---------------------------------------------------------------------------

func TestAuthenticate_Success(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"login": "alice", "id": 1})
	}))

	if err := g.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: unexpected error: %v", err)
	}
}

func TestAuthenticate_Unauthorized(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Bad credentials"})
	}))

	if err := g.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestAuthenticate_ContextCancelled(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := g.Authenticate(ctx); err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// ListPullRequests
// ---------------------------------------------------------------------------

func TestListPullRequests_Basic(t *testing.T) {
	prs := []any{
		prFixture(1, "First PR", "open", "alice", nil),
		prFixture(2, "Second PR", "open", "bob", map[string]any{
			"labels": []any{
				map[string]any{"name": "bug"},
				map[string]any{"name": "priority"},
			},
		}),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != repoPath()+"/pulls" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, prs)
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{State: "open"})
	if err != nil {
		t.Fatalf("ListPullRequests: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListPullRequests: got %d results, want 2", len(got))
	}

	if got[0].ID != 1 {
		t.Errorf("PR[0].ID: got %d, want 1", got[0].ID)
	}
	if got[0].Title != "First PR" {
		t.Errorf("PR[0].Title: got %q, want %q", got[0].Title, "First PR")
	}
	if got[0].Author != "alice" {
		t.Errorf("PR[0].Author: got %q, want %q", got[0].Author, "alice")
	}
	if got[0].State != "open" {
		t.Errorf("PR[0].State: got %q, want %q", got[0].State, "open")
	}
	if got[0].SourceBranch != "feat/branch" {
		t.Errorf("PR[0].SourceBranch: got %q", got[0].SourceBranch)
	}
	if got[0].TargetBranch != "main" {
		t.Errorf("PR[0].TargetBranch: got %q", got[0].TargetBranch)
	}

	// Labels on second PR
	if len(got[1].Labels) != 2 {
		t.Errorf("PR[1].Labels len: got %d, want 2", len(got[1].Labels))
	}
}

func TestListPullRequests_StateMapping(t *testing.T) {
	cases := []struct {
		state       string
		wantAPIState string
	}{
		{"open", "open"},
		{"", "open"},
		{"closed", "closed"},
		{"all", "all"},
		{"merged", "closed"}, // merged maps to closed + client filter
	}

	for _, tc := range cases {
		t.Run("state="+tc.state, func(t *testing.T) {
			_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotState := r.URL.Query().Get("state")
				if gotState != tc.wantAPIState {
					t.Errorf("API state param: got %q, want %q", gotState, tc.wantAPIState)
				}
				writeJSON(w, http.StatusOK, []any{})
			}))

			_, _ = g.ListPullRequests(context.Background(), models.PRListOptions{State: tc.state})
		})
	}
}

func TestListPullRequests_MergedFilter(t *testing.T) {
	mergedAt := fixedTime().Format(time.RFC3339)

	prs := []any{
		prFixture(1, "Open PR", "closed", "alice", nil),                         // no merged_at → excluded
		prFixture(2, "Merged PR", "closed", "bob", map[string]any{"merged_at": mergedAt}), // merged_at set → included
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, prs)
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{State: "merged"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 merged PR, got %d", len(got))
	}
	if got[0].ID != 2 {
		t.Errorf("expected PR #2, got #%d", got[0].ID)
	}
	if got[0].State != "merged" {
		t.Errorf("State: got %q, want %q", got[0].State, "merged")
	}
}

func TestListPullRequests_AuthorFilter(t *testing.T) {
	prs := []any{
		prFixture(1, "Alice PR", "open", "alice", nil),
		prFixture(2, "Bob PR", "open", "bob", nil),
		prFixture(3, "Alice PR 2", "open", "alice", nil),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, prs)
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{Author: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 PRs by alice, got %d", len(got))
	}
	for _, pr := range got {
		if pr.Author != "alice" {
			t.Errorf("expected author alice, got %q", pr.Author)
		}
	}
}

func TestListPullRequests_Limit(t *testing.T) {
	prs := []any{
		prFixture(1, "PR 1", "open", "alice", nil),
		prFixture(2, "PR 2", "open", "alice", nil),
		prFixture(3, "PR 3", "open", "alice", nil),
		prFixture(4, "PR 4", "open", "alice", nil),
		prFixture(5, "PR 5", "open", "alice", nil),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, prs)
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Limit=3: got %d results, want 3", len(got))
	}
}

func TestListPullRequests_Draft(t *testing.T) {
	prs := []any{
		prFixture(1, "Draft PR", "open", "alice", map[string]any{"draft": true}),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, prs)
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got[0].Draft {
		t.Error("Draft: expected true, got false")
	}
}

func TestListPullRequests_Pagination(t *testing.T) {
	page1 := []any{prFixture(1, "PR 1", "open", "alice", nil)}
	page2 := []any{prFixture(2, "PR 2", "open", "bob", nil)}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			// Signal that there is a next page via the Link header.
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			writeJSON(w, http.StatusOK, page1)
		case "2":
			writeJSON(w, http.StatusOK, page2)
		default:
			writeJSON(w, http.StatusOK, []any{})
		}
	}))

	got, err := g.ListPullRequests(context.Background(), models.PRListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 PRs across pages, got %d", len(got))
	}
}

func TestListPullRequests_APIError(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "server error"})
	}))

	_, err := g.ListPullRequests(context.Background(), models.PRListOptions{})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestListPullRequests_Timestamps(t *testing.T) {
	prs := []any{prFixture(1, "PR", "open", "alice", nil)}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, prs)
	}))

	got, _ := g.ListPullRequests(context.Background(), models.PRListOptions{})
	if got[0].CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if got[0].UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

// ---------------------------------------------------------------------------
// GetPullRequest
// ---------------------------------------------------------------------------

func TestGetPullRequest_Success(t *testing.T) {
	fixture := prFixture(42, "feat: add feature", "open", "alice", map[string]any{
		"labels": []any{map[string]any{"name": "enhancement"}},
	})

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := repoPath() + "/pulls/42"
		if r.URL.Path != want {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, fixture)
	}))

	pr, err := g.GetPullRequest(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetPullRequest: unexpected error: %v", err)
	}

	if pr.ID != 42 {
		t.Errorf("ID: got %d, want 42", pr.ID)
	}
	if pr.Title != "feat: add feature" {
		t.Errorf("Title: got %q", pr.Title)
	}
	if pr.Author != "alice" {
		t.Errorf("Author: got %q", pr.Author)
	}
	if len(pr.Labels) != 1 || pr.Labels[0] != "enhancement" {
		t.Errorf("Labels: got %v", pr.Labels)
	}
}

func TestGetPullRequest_NotFound(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}))

	_, err := g.GetPullRequest(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGetPullRequest_MergedState(t *testing.T) {
	mergedAt := fixedTime().Format(time.RFC3339)
	fixture := prFixture(10, "Merged PR", "closed", "bob", map[string]any{
		"merged_at": mergedAt,
	})

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, fixture)
	}))

	pr, err := g.GetPullRequest(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetPullRequest: unexpected error: %v", err)
	}
	if pr.State != "merged" {
		t.Errorf("State: got %q, want %q", pr.State, "merged")
	}
}

// ---------------------------------------------------------------------------
// ListBranches
// ---------------------------------------------------------------------------

// newBranchTestServer builds a server that handles both the repo GET (for
// default branch detection) and the branches list endpoint.
func newBranchTestServer(t *testing.T, defaultBranch string, branches []any) *GitHub {
	t.Helper()

	mux := http.NewServeMux()

	// Repo info → supplies DefaultBranch.
	mux.HandleFunc(repoPath(), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"default_branch": defaultBranch})
	})

	// Branch list.
	mux.HandleFunc(repoPath()+"/branches", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, branches)
	})

	_, g := newTestServer(t, mux)
	return g
}

func TestListBranches_Basic(t *testing.T) {
	sha := "abc1234def5678901234567890abcdef12345678"
	branches := []any{
		branchFixture("main", sha, true),
		branchFixture("feat/watch", sha[:40], false),
	}

	g := newBranchTestServer(t, "main", branches)

	got, err := g.ListBranches(context.Background())
	if err != nil {
		t.Fatalf("ListBranches: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(got))
	}

	main := got[0]
	if main.Name != "main" {
		t.Errorf("Name: got %q, want %q", main.Name, "main")
	}
	if !main.IsDefault {
		t.Error("main should be the default branch")
	}
	if !main.Protected {
		t.Error("main should be protected")
	}

	feat := got[1]
	if feat.IsDefault {
		t.Error("feat/watch should not be the default branch")
	}
	if feat.Protected {
		t.Error("feat/watch should not be protected")
	}
}

func TestListBranches_ShortSHA(t *testing.T) {
	sha := "deadbeef1234567890abcdef1234567890abcdef"
	g := newBranchTestServer(t, "main", []any{branchFixture("main", sha, false)})

	got, err := g.ListBranches(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].LastCommit.SHA != sha {
		t.Errorf("SHA: got %q, want %q", got[0].LastCommit.SHA, sha)
	}
	if got[0].LastCommit.ShortSHA != sha[:7] {
		t.Errorf("ShortSHA: got %q, want %q", got[0].LastCommit.ShortSHA, sha[:7])
	}
}

func TestListBranches_DefaultBranchDetection(t *testing.T) {
	branches := []any{
		branchFixture("main", "aaa0000", false),
		branchFixture("develop", "bbb1111", false),
		branchFixture("release", "ccc2222", false),
	}

	g := newBranchTestServer(t, "develop", branches)

	got, err := g.ListBranches(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := 0
	for _, b := range got {
		if b.IsDefault {
			defaults++
			if b.Name != "develop" {
				t.Errorf("wrong default branch: got %q, want %q", b.Name, "develop")
			}
		}
	}
	if defaults != 1 {
		t.Errorf("expected exactly 1 default branch, got %d", defaults)
	}
}

func TestListBranches_RepoAPIError(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "server error"})
	}))

	_, err := g.ListBranches(context.Background())
	if err == nil {
		t.Fatal("expected error when repo endpoint fails, got nil")
	}
}

func TestListBranches_BranchesAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(repoPath(), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc(repoPath()+"/branches", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]any{"message": "forbidden"})
	})

	_, g := newTestServer(t, mux)

	_, err := g.ListBranches(context.Background())
	if err == nil {
		t.Fatal("expected error when branches endpoint fails, got nil")
	}
}

func TestListBranches_Pagination(t *testing.T) {
	sha := "abc1234def5678901234567890abcdef12345678"
	page1 := []any{branchFixture("main", sha, true)}
	page2 := []any{branchFixture("develop", sha, false)}

	mux := http.NewServeMux()
	mux.HandleFunc(repoPath(), func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc(repoPath()+"/branches", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			writeJSON(w, http.StatusOK, page1)
		default:
			writeJSON(w, http.StatusOK, page2)
		}
	})

	_, g := newTestServer(t, mux)

	got, err := g.ListBranches(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 branches across pages, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// NewGitHub / registration
// ---------------------------------------------------------------------------

func TestNewGitHub_MissingToken(t *testing.T) {
	_, err := NewGitHub(provider.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error when token is empty, got nil")
	}
}

func TestNewGitHub_EnterpriseURL(t *testing.T) {
	_, err := NewGitHub(provider.ProviderConfig{
		Token:   "ghp-test",
		BaseURL: "https://github.example.com",
	})
	if err != nil {
		t.Fatalf("NewGitHub with enterprise URL: %v", err)
	}
}

func TestNewGitHub_InvalidEnterpriseURL(t *testing.T) {
	_, err := NewGitHub(provider.ProviderConfig{
		Token:   "ghp-test",
		BaseURL: "://bad",
	})
	if err == nil {
		t.Fatal("expected error for invalid enterprise URL, got nil")
	}
}

func TestInit_RegistersProvider(t *testing.T) {
	factory, err := provider.Get(providerName)
	if err != nil {
		t.Fatalf("provider.Get(%q): %v", providerName, err)
	}
	p, err := factory(provider.ProviderConfig{Token: "tok"})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if p.Name() != providerName {
		t.Errorf("Name: got %q, want %q", p.Name(), providerName)
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func TestShortSHA(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc1234def5678", "abc1234"},
		{"abc1234", "abc1234"},         // exactly 7
		{"abc12", "abc12"},             // shorter than 7
		{"", ""},                       // empty
	}
	for _, c := range cases {
		if got := shortSHA(c.in); got != c.want {
			t.Errorf("shortSHA(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveState(t *testing.T) {
	cases := []struct {
		in            string
		wantState     string
		wantFiltered  bool
	}{
		{"open", "open", false},
		{"", "open", false},
		{"closed", "closed", false},
		{"all", "all", false},
		{"merged", "closed", true},
	}
	for _, c := range cases {
		state, filtered := resolveState(c.in)
		if state != c.wantState || filtered != c.wantFiltered {
			t.Errorf("resolveState(%q): got (%q, %v), want (%q, %v)",
				c.in, state, filtered, c.wantState, c.wantFiltered)
		}
	}
}

func TestPerPage(t *testing.T) {
	cases := []struct{ limit, want int }{
		{0, 100},
		{-1, 100},
		{50, 50},
		{100, 100},
		{200, 100},
	}
	for _, c := range cases {
		if got := perPage(c.limit); got != c.want {
			t.Errorf("perPage(%d): got %d, want %d", c.limit, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ListCommits
// ---------------------------------------------------------------------------

// commitFixture builds a minimal GitHub repository-commit JSON object.
func commitFixture(sha, message, authorName string) map[string]any {
	return map[string]any{
		"sha": sha,
		"commit": map[string]any{
			"message": message,
			"author": map[string]any{
				"name":  authorName,
				"email": authorName + "@example.com",
				"date":  fixedTime().Format(time.RFC3339),
			},
		},
		"author": map[string]any{"login": authorName},
	}
}

func TestListCommits_Basic(t *testing.T) {
	commits := []any{
		commitFixture("abc1234def5678901234567890abcdef12345678", "feat: add watch", "alice"),
		commitFixture("bcd2345ef0123456789012345678901234567890", "fix: nil pointer", "bob"),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != repoPath()+"/commits" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, commits)
	}))

	got, err := g.ListCommits(context.Background(), models.CommitListOptions{})
	if err != nil {
		t.Fatalf("ListCommits: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(got))
	}

	c0 := got[0]
	if c0.SHA != "abc1234def5678901234567890abcdef12345678" {
		t.Errorf("SHA: got %q", c0.SHA)
	}
	if c0.ShortSHA != "abc1234" {
		t.Errorf("ShortSHA: got %q, want %q", c0.ShortSHA, "abc1234")
	}
	if c0.Message != "feat: add watch" {
		t.Errorf("Message: got %q", c0.Message)
	}
	if c0.Author != "alice" {
		t.Errorf("Author: got %q, want %q", c0.Author, "alice")
	}
	if c0.Date.IsZero() {
		t.Error("Date should not be zero")
	}
}

func TestListCommits_BranchParam(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sha"); got != "feat/branch" {
			t.Errorf("sha param: got %q, want %q", got, "feat/branch")
		}
		writeJSON(w, http.StatusOK, []any{})
	}))

	_, _ = g.ListCommits(context.Background(), models.CommitListOptions{Branch: "feat/branch"})
}

func TestListCommits_AuthorParam(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("author"); got != "alice" {
			t.Errorf("author param: got %q, want %q", got, "alice")
		}
		writeJSON(w, http.StatusOK, []any{})
	}))

	_, _ = g.ListCommits(context.Background(), models.CommitListOptions{Author: "alice"})
}

func TestListCommits_SinceParam(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since := r.URL.Query().Get("since")
		if since == "" {
			t.Error("expected 'since' query param to be set, got empty")
		}
		writeJSON(w, http.StatusOK, []any{})
	}))

	_, _ = g.ListCommits(context.Background(), models.CommitListOptions{Since: 7 * 24 * time.Hour})
}

func TestListCommits_NoSinceWhenZero(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if since := r.URL.Query().Get("since"); since != "" {
			t.Errorf("expected no 'since' param, got %q", since)
		}
		writeJSON(w, http.StatusOK, []any{})
	}))

	_, _ = g.ListCommits(context.Background(), models.CommitListOptions{Since: 0})
}

func TestListCommits_Limit(t *testing.T) {
	commits := []any{
		commitFixture("sha1", "msg1", "alice"),
		commitFixture("sha2", "msg2", "alice"),
		commitFixture("sha3", "msg3", "alice"),
		commitFixture("sha4", "msg4", "alice"),
	}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, commits)
	}))

	got, err := g.ListCommits(context.Background(), models.CommitListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Limit=2: got %d commits, want 2", len(got))
	}
}

func TestListCommits_BranchSetOnResult(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{
			commitFixture("abc1234", "msg", "alice"),
		})
	}))

	got, err := g.ListCommits(context.Background(), models.CommitListOptions{Branch: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Branch != "main" {
		t.Errorf("Branch: got %q, want %q", got[0].Branch, "main")
	}
}

func TestListCommits_Pagination(t *testing.T) {
	page1 := []any{commitFixture("sha1", "first", "alice")}
	page2 := []any{commitFixture("sha2", "second", "bob")}

	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			writeJSON(w, http.StatusOK, page1)
		default:
			writeJSON(w, http.StatusOK, page2)
		}
	}))

	got, err := g.ListCommits(context.Background(), models.CommitListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 commits across pages, got %d", len(got))
	}
}

func TestListCommits_APIError(t *testing.T) {
	_, g := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "server error"})
	}))

	_, err := g.ListCommits(context.Background(), models.CommitListOptions{})
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetCIStatus
// ---------------------------------------------------------------------------

// ciMux builds an http.ServeMux that handles both CI endpoints for a given ref.
func ciMux(t *testing.T, ref string, statusResp, checksResp any) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	statusPath := fmt.Sprintf("%s/commits/%s/status", repoPath(), ref)
	checksPath := fmt.Sprintf("%s/commits/%s/check-runs", repoPath(), ref)

	mux.HandleFunc(statusPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, statusResp)
	})
	mux.HandleFunc(checksPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, checksResp)
	})
	return mux
}

// combinedStatusFixture builds a CombinedStatus JSON object.
func combinedStatusFixture(state string, statuses []any) map[string]any {
	if statuses == nil {
		statuses = []any{}
	}
	return map[string]any{
		"state":    state,
		"statuses": statuses,
	}
}

// checkRunsFixture builds the check-runs list response JSON.
func checkRunsFixture(runs []any) map[string]any {
	return map[string]any{
		"total_count": len(runs),
		"check_runs":  runs,
	}
}

// checkRunItem builds a single CheckRun JSON object.
func checkRunItem(name, status, conclusion, htmlURL string) map[string]any {
	return map[string]any{
		"id":          1,
		"name":        name,
		"status":      status,
		"conclusion":  conclusion,
		"html_url":    htmlURL,
		"details_url": htmlURL,
	}
}

func TestGetCIStatus_BothSources(t *testing.T) {
	ref := "main"

	statusResp := combinedStatusFixture("success", []any{
		map[string]any{
			"state":      "success",
			"context":    "ci/lint",
			"target_url": "https://ci.example.com/lint",
		},
	})
	checksResp := checkRunsFixture([]any{
		checkRunItem("unit-tests", "completed", "success", "https://ci.example.com/unit"),
		checkRunItem("build", "completed", "success", "https://ci.example.com/build"),
	})

	_, g := newTestServer(t, ciMux(t, ref, statusResp, checksResp))

	ci, err := g.GetCIStatus(context.Background(), ref)
	if err != nil {
		t.Fatalf("GetCIStatus: unexpected error: %v", err)
	}

	if ci.Ref != ref {
		t.Errorf("Ref: got %q, want %q", ci.Ref, ref)
	}
	if ci.State != "success" {
		t.Errorf("State: got %q, want %q", ci.State, "success")
	}
	// 1 legacy status + 2 check runs = 3 jobs
	if len(ci.Jobs) != 3 {
		t.Fatalf("Jobs len: got %d, want 3", len(ci.Jobs))
	}
}

func TestGetCIStatus_StatePrecedenceFailure(t *testing.T) {
	ref := "main"

	// Combined says success, but a check run has failed → overall must be failure.
	statusResp := combinedStatusFixture("success", []any{})
	checksResp := checkRunsFixture([]any{
		checkRunItem("deploy", "completed", "failure", "https://ci.example.com/deploy"),
	})

	_, g := newTestServer(t, ciMux(t, ref, statusResp, checksResp))

	ci, err := g.GetCIStatus(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ci.State != "failure" {
		t.Errorf("State: got %q, want %q", ci.State, "failure")
	}
}

func TestGetCIStatus_StatePrecedencePending(t *testing.T) {
	ref := "main"

	// Combined says success, but a check run is still running.
	statusResp := combinedStatusFixture("success", []any{})
	checksResp := checkRunsFixture([]any{
		checkRunItem("slow-test", "in_progress", "", "https://ci.example.com/slow"),
	})

	_, g := newTestServer(t, ciMux(t, ref, statusResp, checksResp))

	ci, err := g.GetCIStatus(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ci.State != "pending" {
		t.Errorf("State: got %q, want %q", ci.State, "pending")
	}
}

func TestGetCIStatus_LegacyStatusOnly(t *testing.T) {
	ref := "v1.0.0"

	statusResp := combinedStatusFixture("failure", []any{
		map[string]any{
			"state":      "failure",
			"context":    "ci/test",
			"target_url": "https://ci.example.com/test",
		},
	})
	checksResp := checkRunsFixture([]any{}) // no check runs

	_, g := newTestServer(t, ciMux(t, ref, statusResp, checksResp))

	ci, err := g.GetCIStatus(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ci.State != "failure" {
		t.Errorf("State: got %q, want %q", ci.State, "failure")
	}
	if len(ci.Jobs) != 1 {
		t.Fatalf("Jobs len: got %d, want 1", len(ci.Jobs))
	}
	if ci.Jobs[0].Stage != "status" {
		t.Errorf("Stage: got %q, want %q", ci.Jobs[0].Stage, "status")
	}
	if ci.Jobs[0].Name != "ci/test" {
		t.Errorf("Name: got %q", ci.Jobs[0].Name)
	}
}

func TestGetCIStatus_CheckRunsOnly(t *testing.T) {
	ref := "sha-abc"

	statusResp := combinedStatusFixture("", []any{}) // no legacy statuses
	checksResp := checkRunsFixture([]any{
		checkRunItem("lint", "completed", "success", "https://github.com/lint"),
	})

	_, g := newTestServer(t, ciMux(t, ref, statusResp, checksResp))

	ci, err := g.GetCIStatus(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ci.State != "success" {
		t.Errorf("State: got %q, want %q", ci.State, "success")
	}
	if ci.Jobs[0].Stage != "check" {
		t.Errorf("Stage: got %q, want %q", ci.Jobs[0].Stage, "check")
	}
}

func TestGetCIStatus_CheckRunConclusions(t *testing.T) {
	cases := []struct {
		status, conclusion string
		wantState          string
	}{
		{"completed", "success", "success"},
		{"completed", "neutral", "success"},
		{"completed", "skipped", "success"},
		{"completed", "failure", "failure"},
		{"completed", "timed_out", "failure"},
		{"completed", "action_required", "failure"},
		{"completed", "cancelled", "failure"},
		{"in_progress", "", "pending"},
		{"queued", "", "pending"},
	}

	for _, tc := range cases {
		t.Run(tc.status+"/"+tc.conclusion, func(t *testing.T) {
			run := &gh.CheckRun{}
			run.Status = &tc.status
			run.Conclusion = &tc.conclusion

			got := checkRunStatus(run)
			if got != tc.wantState {
				t.Errorf("checkRunStatus(status=%q, conclusion=%q): got %q, want %q",
					tc.status, tc.conclusion, got, tc.wantState)
			}
		})
	}
}

func TestGetCIStatus_StatusAPIError(t *testing.T) {
	ref := "main"
	mux := http.NewServeMux()
	statusPath := fmt.Sprintf("%s/commits/%s/status", repoPath(), ref)
	checksPath := fmt.Sprintf("%s/commits/%s/check-runs", repoPath(), ref)

	mux.HandleFunc(statusPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "oops"})
	})
	mux.HandleFunc(checksPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, checkRunsFixture([]any{}))
	})

	_, g := newTestServer(t, mux)

	_, err := g.GetCIStatus(context.Background(), ref)
	if err == nil {
		t.Fatal("expected error when status endpoint fails, got nil")
	}
}

func TestGetCIStatus_ChecksAPIError(t *testing.T) {
	ref := "main"
	mux := http.NewServeMux()
	statusPath := fmt.Sprintf("%s/commits/%s/status", repoPath(), ref)
	checksPath := fmt.Sprintf("%s/commits/%s/check-runs", repoPath(), ref)

	mux.HandleFunc(statusPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, combinedStatusFixture("success", []any{}))
	})
	mux.HandleFunc(checksPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, map[string]any{"message": "forbidden"})
	})

	_, g := newTestServer(t, mux)

	_, err := g.GetCIStatus(context.Background(), ref)
	if err == nil {
		t.Fatal("expected error when checks endpoint fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// mergeState / normalizeState
// ---------------------------------------------------------------------------

func TestMergeState(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"failure", "success", "failure"},
		{"success", "failure", "failure"},
		{"pending", "success", "pending"},
		{"success", "pending", "pending"},
		{"success", "success", "success"},
		{"", "success", "success"},
		{"success", "", "success"},
		{"", "", ""},
	}
	for _, c := range cases {
		got := mergeState(c.a, c.b)
		if got != c.want {
			t.Errorf("mergeState(%q,%q): got %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestNormalizeState(t *testing.T) {
	cases := []struct{ in, want string }{
		{"success", "success"},
		{"failure", "failure"},
		{"error", "failure"},
		{"pending", "pending"},
		{"", ""},
		{"unknown", "unknown"},
	}
	for _, c := range cases {
		if got := normalizeState(c.in); got != c.want {
			t.Errorf("normalizeState(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}
