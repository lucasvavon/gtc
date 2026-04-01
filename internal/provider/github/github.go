// Package github implements the gtc provider interface for GitHub and
// GitHub Enterprise Server. It registers itself under the name "github"
// via an init() function so callers only need to blank-import this package.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"

	"github.com/lucasvavon/gtc/internal/models"
	"github.com/lucasvavon/gtc/internal/provider"
)

const providerName = "github"

// GitHub implements provider.Provider backed by the GitHub REST API.
type GitHub struct {
	client *gh.Client
	cfg    provider.ProviderConfig
}

// NewGitHub is the provider.Factory for GitHub. It builds an OAuth2 HTTP
// client from cfg.Token, constructs a go-github client, and configures a
// custom BaseURL for GitHub Enterprise Server when cfg.BaseURL is set.
func NewGitHub(cfg provider.ProviderConfig) (provider.Provider, error) {
	if cfg.Token == "" {
		return nil, errors.New("github: token is required (set 'token' in .gtc.yaml or GITHUB_TOKEN)")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	tc := oauth2.NewClient(context.Background(), ts)

	client := gh.NewClient(tc)

	if cfg.BaseURL != "" {
		// GitHub Enterprise Server: WithEnterpriseURLs appends /api/v3/ automatically.
		uploadURL := strings.TrimRight(cfg.BaseURL, "/") + "/api/uploads/"
		var err error
		client, err = client.WithEnterpriseURLs(cfg.BaseURL, uploadURL)
		if err != nil {
			return nil, fmt.Errorf("github: invalid enterprise base URL %q: %w", cfg.BaseURL, err)
		}
	}

	return &GitHub{client: client, cfg: cfg}, nil
}

// newWithClient injects a pre-configured go-github client (used in tests).
func newWithClient(client *gh.Client, cfg provider.ProviderConfig) *GitHub {
	return &GitHub{client: client, cfg: cfg}
}

// Name returns the canonical provider name.
func (g *GitHub) Name() string { return providerName }

// Authenticate calls GET /user to verify the token and returns a descriptive
// error on failure.
func (g *GitHub) Authenticate(ctx context.Context) error {
	_, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("github: authentication failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Pull requests
// ---------------------------------------------------------------------------

// ListPullRequests fetches pull requests for the configured repository,
// applying State, Author and Limit filters. Pagination is handled
// automatically; at most Limit results are returned (0 = unlimited).
func (g *GitHub) ListPullRequests(ctx context.Context, opts models.PRListOptions) ([]models.PullRequest, error) {
	apiState, filterMerged := resolveState(opts.State)

	listOpts := &gh.PullRequestListOptions{
		State:       apiState,
		ListOptions: gh.ListOptions{PerPage: perPage(opts.Limit)},
	}

	var result []models.PullRequest

	for {
		prs, resp, err := g.client.PullRequests.List(ctx, g.cfg.Owner, g.cfg.Repo, listOpts)
		if err != nil {
			return nil, fmt.Errorf("github: listing pull requests: %w", err)
		}

		for _, pr := range prs {
			// "merged" is a pseudo-state: closed PRs where MergedAt is set.
			if filterMerged && pr.MergedAt == nil {
				continue
			}
			// Author filter is not supported server-side; apply client-side.
			if opts.Author != "" && pr.GetUser().GetLogin() != opts.Author {
				continue
			}

			result = append(result, mapPR(pr))

			if opts.Limit > 0 && len(result) >= opts.Limit {
				return result, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return result, nil
}

// GetPullRequest fetches a single pull request by its number.
func (g *GitHub) GetPullRequest(ctx context.Context, id int) (*models.PullRequest, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, g.cfg.Owner, g.cfg.Repo, id)
	if err != nil {
		return nil, fmt.Errorf("github: getting pull request #%d: %w", id, err)
	}

	m := mapPR(pr)
	return &m, nil
}

// ---------------------------------------------------------------------------
// Branches
// ---------------------------------------------------------------------------

// ListBranches fetches all branches for the configured repository. It makes
// one extra call to GET /repos/{owner}/{repo} to resolve the default branch.
func (g *GitHub) ListBranches(ctx context.Context) ([]models.Branch, error) {
	repo, _, err := g.client.Repositories.Get(ctx, g.cfg.Owner, g.cfg.Repo)
	if err != nil {
		return nil, fmt.Errorf("github: fetching repository info: %w", err)
	}
	defaultBranch := repo.GetDefaultBranch()

	listOpts := &gh.BranchListOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	var result []models.Branch

	for {
		branches, resp, err := g.client.Repositories.ListBranches(ctx, g.cfg.Owner, g.cfg.Repo, listOpts)
		if err != nil {
			return nil, fmt.Errorf("github: listing branches: %w", err)
		}

		for _, b := range branches {
			result = append(result, mapBranch(b, defaultBranch))
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Commits
// ---------------------------------------------------------------------------

// ListCommits fetches commits for the configured repository.
// Branch maps to SHA (branch name), Author is filtered server-side,
// Since is converted from a duration to an absolute time, and Limit
// caps the total number of results across pages.
func (g *GitHub) ListCommits(ctx context.Context, opts models.CommitListOptions) ([]models.Commit, error) {
	listOpts := &gh.CommitsListOptions{
		SHA:         opts.Branch,
		Author:      opts.Author,
		ListOptions: gh.ListOptions{PerPage: perPage(opts.Limit)},
	}
	if opts.Since > 0 {
		listOpts.Since = time.Now().UTC().Add(-opts.Since)
	}

	var result []models.Commit

	for {
		commits, resp, err := g.client.Repositories.ListCommits(ctx, g.cfg.Owner, g.cfg.Repo, listOpts)
		if err != nil {
			return nil, fmt.Errorf("github: listing commits: %w", err)
		}

		for _, rc := range commits {
			result = append(result, mapCommit(rc, opts.Branch))
			if opts.Limit > 0 && len(result) >= opts.Limit {
				return result, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// CI status
// ---------------------------------------------------------------------------

// GetCIStatus fetches both the legacy commit-status API and the Checks API
// for the given ref, then merges them into a unified CIStatus. The two HTTP
// calls run concurrently. The overall state reflects the worst status across
// both sources: failure > pending > success.
func (g *GitHub) GetCIStatus(ctx context.Context, ref string) (*models.CIStatus, error) {
	type combinedResult struct {
		status *gh.CombinedStatus
		err    error
	}
	type checksResult struct {
		runs *gh.ListCheckRunsResults
		err  error
	}

	combinedCh := make(chan combinedResult, 1)
	checksCh := make(chan checksResult, 1)

	go func() {
		s, _, err := g.client.Repositories.GetCombinedStatus(ctx, g.cfg.Owner, g.cfg.Repo, ref, nil)
		combinedCh <- combinedResult{s, err}
	}()

	go func() {
		r, _, err := g.client.Checks.ListCheckRunsForRef(ctx, g.cfg.Owner, g.cfg.Repo, ref,
			&gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}})
		checksCh <- checksResult{r, err}
	}()

	cr := <-combinedCh
	runs := <-checksCh

	if cr.err != nil {
		return nil, fmt.Errorf("github: commit status for %q: %w", ref, cr.err)
	}
	if runs.err != nil {
		return nil, fmt.Errorf("github: check runs for %q: %w", ref, runs.err)
	}

	return buildCIStatus(ref, cr.status, runs.runs), nil
}

// Watch is not yet implemented.
func (g *GitHub) Watch(_ context.Context, _ models.WatchOptions) (<-chan models.Event, error) {
	return nil, errors.New("github: Watch not yet implemented")
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

// mapPR converts a go-github PullRequest into the canonical models.PullRequest.
func mapPR(pr *gh.PullRequest) models.PullRequest {
	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.GetName())
	}

	// GitHub represents merged PRs as state=closed with MergedAt set.
	state := pr.GetState()
	if pr.MergedAt != nil {
		state = "merged"
	}

	return models.PullRequest{
		ID:           pr.GetNumber(),
		Title:        pr.GetTitle(),
		Author:       pr.GetUser().GetLogin(),
		State:        state,
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		URL:          pr.GetHTMLURL(),
		CreatedAt:    tsToTime(pr.CreatedAt),
		UpdatedAt:    tsToTime(pr.UpdatedAt),
		Labels:       labels,
		Draft:        pr.GetDraft(),
	}
}

// mapBranch converts a go-github Branch into the canonical models.Branch.
func mapBranch(b *gh.Branch, defaultBranch string) models.Branch {
	var lastCommit models.Commit
	if c := b.GetCommit(); c != nil {
		sha := c.GetSHA()
		lastCommit = models.Commit{
			SHA:      sha,
			ShortSHA: shortSHA(sha),
		}
	}

	return models.Branch{
		Name:       b.GetName(),
		IsDefault:  b.GetName() == defaultBranch,
		Protected:  b.GetProtected(),
		LastCommit: lastCommit,
	}
}

// mapCommit converts a go-github RepositoryCommit into the canonical
// models.Commit. Branch is not returned by the API and is sourced from opts.
func mapCommit(rc *gh.RepositoryCommit, branch string) models.Commit {
	sha := rc.GetSHA()

	var message, authorName string
	var date time.Time

	if c := rc.GetCommit(); c != nil {
		message = c.GetMessage()
		if a := c.GetAuthor(); a != nil {
			authorName = a.GetName()
			date = tsToTime(a.Date)
		}
	}

	return models.Commit{
		SHA:      sha,
		ShortSHA: shortSHA(sha),
		Message:  message,
		Author:   authorName,
		Date:     date,
		Branch:   branch,
	}
}

// buildCIStatus merges a CombinedStatus (legacy status API) and a
// ListCheckRunsResults (Checks API) into a single models.CIStatus.
func buildCIStatus(ref string, combined *gh.CombinedStatus, checks *gh.ListCheckRunsResults) *models.CIStatus {
	// Seed with the aggregated state from the legacy status API.
	overall := normalizeState(combined.GetState())

	var jobs []models.CIJob

	// Each legacy status becomes a job in the "status" stage.
	for _, s := range combined.Statuses {
		jobs = append(jobs, models.CIJob{
			Name:   s.GetContext(),
			Stage:  "status",
			Status: normalizeState(s.GetState()),
			URL:    s.GetTargetURL(),
		})
	}

	// Each check run becomes a job in the "check" stage.
	// Its state is merged into the overall result.
	if checks != nil {
		for _, run := range checks.CheckRuns {
			state := checkRunStatus(run)
			jobs = append(jobs, models.CIJob{
				Name:   run.GetName(),
				Stage:  "check",
				Status: state,
				URL:    run.GetHTMLURL(),
			})
			overall = mergeState(overall, state)
		}
	}

	return &models.CIStatus{
		Ref:   ref,
		State: overall,
		Jobs:  jobs,
	}
}

// checkRunStatus converts a CheckRun's status/conclusion to our three-value
// vocabulary: "success", "pending", or "failure".
func checkRunStatus(cr *gh.CheckRun) string {
	if cr.GetStatus() != "completed" {
		return "pending"
	}
	switch cr.GetConclusion() {
	case "success", "neutral", "skipped":
		return "success"
	case "failure", "timed_out", "action_required", "cancelled":
		return "failure"
	default:
		return "pending"
	}
}

// ---------------------------------------------------------------------------
// Small utilities
// ---------------------------------------------------------------------------

// resolveState maps the gtc state vocabulary to a GitHub API state parameter
// and a boolean indicating whether to filter merged-only results client-side.
func resolveState(state string) (apiState string, filterMerged bool) {
	switch state {
	case "merged":
		return "closed", true
	case "closed":
		return "closed", false
	case "all":
		return "all", false
	default: // "open" or empty
		return "open", false
	}
}

// normalizeState maps GitHub state strings to the three canonical values
// (success, pending, failure), preserving unknown values as-is.
func normalizeState(s string) string {
	switch s {
	case "failure", "error":
		return "failure"
	case "pending":
		return "pending"
	case "success":
		return "success"
	default:
		return s
	}
}

// mergeState returns the higher-priority state of the two.
// Priority: failure(3) > pending(2) > success(1) > ""(0).
func mergeState(a, b string) string {
	pri := func(s string) int {
		switch s {
		case "failure":
			return 3
		case "pending":
			return 2
		case "success":
			return 1
		default:
			return 0
		}
	}
	if pri(a) >= pri(b) {
		return a
	}
	return b
}

// perPage returns the items-per-page value to pass to the GitHub API.
// GitHub caps pages at 100 items.
func perPage(limit int) int {
	if limit > 0 && limit < 100 {
		return limit
	}
	return 100
}

// tsToTime safely extracts a time.Time from a nullable *gh.Timestamp.
func tsToTime(ts *gh.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.Time
}

// shortSHA returns the first 7 characters of a commit SHA.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// parseBaseURL builds a *url.URL with a trailing slash (required by go-github).
func parseBaseURL(raw string) (*url.URL, error) {
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	return url.Parse(raw)
}

// ---------------------------------------------------------------------------
// Self-registration
// ---------------------------------------------------------------------------

func init() {
	provider.Register(providerName, NewGitHub)
}

// Compile-time interface check.
var _ provider.Provider = (*GitHub)(nil)
