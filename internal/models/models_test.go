package models

import (
	"encoding/json"
	"testing"
	"time"
)

// mustMarshal serialises v to JSON and fails the test on error.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	return b
}

// mustUnmarshal deserialises data into v and fails the test on error.
func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
}

// fixedTime returns a deterministic UTC timestamp for use in tests.
func fixedTime() time.Time {
	return time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
}

func TestPullRequest_MarshalJSON(t *testing.T) {
	pr := PullRequest{
		ID:           42,
		Title:        "feat: add watch command",
		Author:       "alice",
		State:        "opened",
		SourceBranch: "feat/watch",
		TargetBranch: "main",
		URL:          "https://gitlab.com/org/repo/-/merge_requests/42",
		CreatedAt:    fixedTime(),
		UpdatedAt:    fixedTime(),
		Labels:       []string{"backend", "enhancement"},
		Draft:        false,
		CIStatus:     "success",
	}

	data := mustMarshal(t, pr)

	var got PullRequest
	mustUnmarshal(t, data, &got)

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, pr.ID},
		{"Title", got.Title, pr.Title},
		{"Author", got.Author, pr.Author},
		{"State", got.State, pr.State},
		{"SourceBranch", got.SourceBranch, pr.SourceBranch},
		{"TargetBranch", got.TargetBranch, pr.TargetBranch},
		{"URL", got.URL, pr.URL},
		{"Draft", got.Draft, pr.Draft},
		{"CIStatus", got.CIStatus, pr.CIStatus},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("PullRequest.%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(got.Labels) != len(pr.Labels) {
		t.Errorf("Labels len: got %d, want %d", len(got.Labels), len(pr.Labels))
	}
}

func TestPullRequest_JSONKeys(t *testing.T) {
	pr := PullRequest{SourceBranch: "feat/x", TargetBranch: "main"}
	data := mustMarshal(t, pr)

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{
		"id", "title", "author", "state",
		"source_branch", "target_branch", "url",
		"created_at", "updated_at", "labels", "draft", "ci_status",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestPRListOptions_MarshalJSON(t *testing.T) {
	opts := PRListOptions{
		State:  "opened",
		Author: "bob",
		Label:  "bug",
		Limit:  25,
	}

	data := mustMarshal(t, opts)

	var got PRListOptions
	mustUnmarshal(t, data, &got)

	if got != opts {
		t.Errorf("PRListOptions round-trip: got %+v, want %+v", got, opts)
	}
}

func TestBranch_MarshalJSON(t *testing.T) {
	b := Branch{
		Name:      "main",
		IsDefault: true,
		Protected: true,
		LastCommit: Commit{
			SHA:      "abc123def456",
			ShortSHA: "abc123d",
			Message:  "chore: release v1.0",
			Author:   "alice",
			Date:     fixedTime(),
			Branch:   "main",
		},
	}

	data := mustMarshal(t, b)

	var got Branch
	mustUnmarshal(t, data, &got)

	if got.Name != b.Name {
		t.Errorf("Branch.Name: got %q, want %q", got.Name, b.Name)
	}
	if got.IsDefault != b.IsDefault {
		t.Errorf("Branch.IsDefault: got %v, want %v", got.IsDefault, b.IsDefault)
	}
	if got.Protected != b.Protected {
		t.Errorf("Branch.Protected: got %v, want %v", got.Protected, b.Protected)
	}
	if got.LastCommit.SHA != b.LastCommit.SHA {
		t.Errorf("Branch.LastCommit.SHA: got %q, want %q", got.LastCommit.SHA, b.LastCommit.SHA)
	}
}

func TestBranch_JSONKeys(t *testing.T) {
	data := mustMarshal(t, Branch{})

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{"name", "is_default", "protected", "last_commit"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestCommit_MarshalJSON(t *testing.T) {
	c := Commit{
		SHA:      "deadbeefdeadbeef",
		ShortSHA: "deadbee",
		Message:  "fix: nil pointer in watch loop",
		Author:   "carol",
		Date:     fixedTime(),
		Branch:   "fix/nil-watch",
	}

	data := mustMarshal(t, c)

	var got Commit
	mustUnmarshal(t, data, &got)

	if got.SHA != c.SHA {
		t.Errorf("Commit.SHA: got %q, want %q", got.SHA, c.SHA)
	}
	if got.ShortSHA != c.ShortSHA {
		t.Errorf("Commit.ShortSHA: got %q, want %q", got.ShortSHA, c.ShortSHA)
	}
	if got.Author != c.Author {
		t.Errorf("Commit.Author: got %q, want %q", got.Author, c.Author)
	}
	if got.Branch != c.Branch {
		t.Errorf("Commit.Branch: got %q, want %q", got.Branch, c.Branch)
	}
	if !got.Date.Equal(c.Date) {
		t.Errorf("Commit.Date: got %v, want %v", got.Date, c.Date)
	}
}

func TestCommit_JSONKeys(t *testing.T) {
	data := mustMarshal(t, Commit{})

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{"sha", "short_sha", "message", "author", "date", "branch"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestCommitListOptions_MarshalJSON(t *testing.T) {
	opts := CommitListOptions{
		Branch: "main",
		Author: "alice",
		Since:  7 * 24 * time.Hour,
		Limit:  50,
	}

	data := mustMarshal(t, opts)

	var got CommitListOptions
	mustUnmarshal(t, data, &got)

	if got != opts {
		t.Errorf("CommitListOptions round-trip: got %+v, want %+v", got, opts)
	}
}

func TestCIStatus_MarshalJSON(t *testing.T) {
	ci := CIStatus{
		Ref:   "main",
		State: "failed",
		Jobs: []CIJob{
			{Name: "lint", Stage: "test", Status: "success", URL: "https://gitlab.com/job/1"},
			{Name: "unit-tests", Stage: "test", Status: "failed", URL: "https://gitlab.com/job/2"},
		},
	}

	data := mustMarshal(t, ci)

	var got CIStatus
	mustUnmarshal(t, data, &got)

	if got.Ref != ci.Ref {
		t.Errorf("CIStatus.Ref: got %q, want %q", got.Ref, ci.Ref)
	}
	if got.State != ci.State {
		t.Errorf("CIStatus.State: got %q, want %q", got.State, ci.State)
	}
	if len(got.Jobs) != len(ci.Jobs) {
		t.Fatalf("CIStatus.Jobs len: got %d, want %d", len(got.Jobs), len(ci.Jobs))
	}
	if got.Jobs[1].Status != "failed" {
		t.Errorf("CIStatus.Jobs[1].Status: got %q, want %q", got.Jobs[1].Status, "failed")
	}
}

func TestCIJob_MarshalJSON(t *testing.T) {
	job := CIJob{
		Name:   "build",
		Stage:  "build",
		Status: "success",
		URL:    "https://gitlab.com/job/99",
	}

	data := mustMarshal(t, job)

	var got CIJob
	mustUnmarshal(t, data, &got)

	if got != job {
		t.Errorf("CIJob round-trip: got %+v, want %+v", got, job)
	}
}

func TestCIJob_JSONKeys(t *testing.T) {
	data := mustMarshal(t, CIJob{})

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{"name", "stage", "status", "url"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestEvent_MarshalJSON(t *testing.T) {
	e := Event{
		Kind:      EventPROpened,
		Title:     "feat: add watch command",
		Detail:    "opened by alice against main",
		URL:       "https://gitlab.com/org/repo/-/merge_requests/1",
		Actor:     "alice",
		Timestamp: fixedTime(),
	}

	data := mustMarshal(t, e)

	var got Event
	mustUnmarshal(t, data, &got)

	if got.Kind != e.Kind {
		t.Errorf("Event.Kind: got %q, want %q", got.Kind, e.Kind)
	}
	if got.Title != e.Title {
		t.Errorf("Event.Title: got %q, want %q", got.Title, e.Title)
	}
	if got.Actor != e.Actor {
		t.Errorf("Event.Actor: got %q, want %q", got.Actor, e.Actor)
	}
	if !got.Timestamp.Equal(e.Timestamp) {
		t.Errorf("Event.Timestamp: got %v, want %v", got.Timestamp, e.Timestamp)
	}
}

func TestEvent_JSONKeys(t *testing.T) {
	data := mustMarshal(t, Event{})

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{"kind", "title", "detail", "url", "actor", "timestamp"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestEventKind_Constants(t *testing.T) {
	cases := []struct {
		kind EventKind
		want string
	}{
		{EventPROpened, "pr_opened"},
		{EventPRMerged, "pr_merged"},
		{EventPRClosed, "pr_closed"},
		{EventCIFailed, "ci_failed"},
		{EventCISuccess, "ci_success"},
		{EventBranchCreated, "branch_created"},
		{EventBranchDeleted, "branch_deleted"},
		{EventPush, "push"},
	}
	for _, c := range cases {
		if string(c.kind) != c.want {
			t.Errorf("EventKind %q: got %q", c.want, c.kind)
		}
	}
}

func TestEventKind_MarshalJSON(t *testing.T) {
	kinds := []EventKind{
		EventPROpened, EventPRMerged, EventPRClosed,
		EventCIFailed, EventCISuccess,
		EventBranchCreated, EventBranchDeleted,
		EventPush,
	}
	for _, k := range kinds {
		data := mustMarshal(t, k)

		var got EventKind
		mustUnmarshal(t, data, &got)

		if got != k {
			t.Errorf("EventKind round-trip: got %q, want %q", got, k)
		}
	}
}

func TestWatchOptions_MarshalJSON(t *testing.T) {
	opts := WatchOptions{
		Events:   []EventKind{EventPROpened, EventCIFailed},
		Interval: 30 * time.Second,
	}

	data := mustMarshal(t, opts)

	var got WatchOptions
	mustUnmarshal(t, data, &got)

	if len(got.Events) != len(opts.Events) {
		t.Fatalf("WatchOptions.Events len: got %d, want %d", len(got.Events), len(opts.Events))
	}
	for i, e := range opts.Events {
		if got.Events[i] != e {
			t.Errorf("WatchOptions.Events[%d]: got %q, want %q", i, got.Events[i], e)
		}
	}
	if got.Interval != opts.Interval {
		t.Errorf("WatchOptions.Interval: got %v, want %v", got.Interval, opts.Interval)
	}
}

func TestWatchOptions_JSONKeys(t *testing.T) {
	data := mustMarshal(t, WatchOptions{})

	var m map[string]any
	mustUnmarshal(t, data, &m)

	for _, key := range []string{"events", "interval"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}
