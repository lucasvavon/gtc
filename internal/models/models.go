package models

import "time"

// EventKind identifies the type of event emitted by the watch subsystem.
type EventKind string

const (
	EventPROpened      EventKind = "pr_opened"
	EventPRMerged      EventKind = "pr_merged"
	EventPRClosed      EventKind = "pr_closed"
	EventCIFailed      EventKind = "ci_failed"
	EventCISuccess     EventKind = "ci_success"
	EventBranchCreated EventKind = "branch_created"
	EventBranchDeleted EventKind = "branch_deleted"
	EventPush          EventKind = "push"
)

// PullRequest represents a merge/pull request from a provider.
type PullRequest struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	State        string    `json:"state"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Labels       []string  `json:"labels"`
	Draft        bool      `json:"draft"`
	CIStatus     string    `json:"ci_status"`
}

// PRListOptions filters the list of pull requests returned by a provider.
type PRListOptions struct {
	State  string `json:"state"`
	Author string `json:"author"`
	Label  string `json:"label"`
	Limit  int    `json:"limit"`
}

// Branch represents a VCS branch.
type Branch struct {
	Name       string `json:"name"`
	IsDefault  bool   `json:"is_default"`
	Protected  bool   `json:"protected"`
	LastCommit Commit `json:"last_commit"`
}

// Commit represents a single VCS commit.
type Commit struct {
	SHA     string    `json:"sha"`
	ShortSHA string   `json:"short_sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Branch  string    `json:"branch"`
}

// CommitListOptions filters commits returned by a provider.
type CommitListOptions struct {
	Branch string        `json:"branch"`
	Author string        `json:"author"`
	Since  time.Duration `json:"since"`
	Limit  int           `json:"limit"`
}

// CIStatus holds the overall pipeline status for a ref.
type CIStatus struct {
	Ref   string  `json:"ref"`
	State string  `json:"state"`
	Jobs  []CIJob `json:"jobs"`
}

// CIJob represents a single job within a pipeline.
type CIJob struct {
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// Event is emitted by the watch subsystem when something changes.
type Event struct {
	Kind      EventKind `json:"kind"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail"`
	URL       string    `json:"url"`
	Actor     string    `json:"actor"`
	Timestamp time.Time `json:"timestamp"`
}

// WatchOptions configures which events are observed and how often.
type WatchOptions struct {
	Events   []EventKind   `json:"events"`
	Interval time.Duration `json:"interval"`
}
