package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v60/github"

	"github.com/lucasvavon/gtc/internal/models"
)

// ── Payload shapes (minimal subset of the GitHub Events API) ─────────────────

type prEventPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
	} `json:"pull_request"`
}

type pushEventPayload struct {
	Ref string `json:"ref"`
}

type statusEventPayload struct {
	State string `json:"state"`
	Name  string `json:"context"` // GitHub calls this "context"
}

type branchRefPayload struct {
	RefType string `json:"ref_type"`
	Ref     string `json:"ref"`
}

// ── Watch ─────────────────────────────────────────────────────────────────────

// Watch polls GET /repos/{owner}/{repo}/events at the configured interval and
// emits matching events on the returned channel. The channel is closed when ctx
// is cancelled. GitHub's X-Poll-Interval response header is respected: if the
// server requests a longer interval it overrides opts.Interval for subsequent
// polls.
func (g *GitHub) Watch(ctx context.Context, opts models.WatchOptions) (<-chan models.Event, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	// Pre-build a filter set; empty means "accept all kinds".
	filter := make(map[models.EventKind]struct{}, len(opts.Events))
	for _, k := range opts.Events {
		filter[k] = struct{}{}
	}
	wantKind := func(k models.EventKind) bool {
		if len(filter) == 0 {
			return true
		}
		_, ok := filter[k]
		return ok
	}

	ch := make(chan models.Event, 32)

	go func() {
		defer close(ch)

		// seenID holds the GitHub event ID of the most-recently emitted event.
		// The Events API returns results newest-first; we stop collecting when
		// we encounter this ID so that each event is emitted exactly once.
		var seenID string

		poll := func() (hint time.Duration) {
			ghEvents, pollInterval, err := g.fetchEvents(ctx)
			if err != nil {
				return 0
			}

			// Collect only events newer than seenID (newest-first order).
			var toEmit []models.Event
			for _, ev := range ghEvents {
				if ev.GetID() == seenID {
					break
				}
				e, ok := mapGitHubEvent(ev)
				if !ok || !wantKind(e.Kind) {
					continue
				}
				toEmit = append(toEmit, e)
			}

			// Advance the watermark to the newest event on this page.
			if len(ghEvents) > 0 {
				seenID = ghEvents[0].GetID()
			}

			// Emit in chronological order (oldest first = reverse of API order).
			for i := len(toEmit) - 1; i >= 0; i-- {
				select {
				case ch <- toEmit[i]:
				case <-ctx.Done():
					return pollInterval
				}
			}

			return pollInterval
		}

		// First poll fires immediately.
		if hint := poll(); hint > 0 {
			interval = hint
		}

		for {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if hint := poll(); hint > 0 {
					interval = hint
				}
			}
		}
	}()

	return ch, nil
}

// fetchEvents calls GET /repos/{owner}/{repo}/events and returns the raw event
// list along with the X-Poll-Interval value from the response header (zero when
// the header is absent or unparseable).
func (g *GitHub) fetchEvents(ctx context.Context) ([]*gh.Event, time.Duration, error) {
	events, resp, err := g.client.Activity.ListRepositoryEvents(
		ctx, g.cfg.Owner, g.cfg.Repo,
		&gh.ListOptions{PerPage: 100},
	)
	if err != nil {
		return nil, 0, fmt.Errorf("github: fetching events: %w", err)
	}

	var pollInterval time.Duration
	if v := resp.Header.Get("X-Poll-Interval"); v != "" {
		if secs, parseErr := strconv.Atoi(v); parseErr == nil && secs > 0 {
			pollInterval = time.Duration(secs) * time.Second
		}
	}

	return events, pollInterval, nil
}

// mapGitHubEvent converts a single GitHub API event into a models.Event.
// Returns false when the event type is unknown or the action is not mapped.
func mapGitHubEvent(ev *gh.Event) (models.Event, bool) {
	actor := ""
	if a := ev.GetActor(); a != nil {
		actor = a.GetLogin()
	}
	ts := ev.GetCreatedAt().Time
	raw := ev.GetRawPayload()

	switch ev.GetType() {
	case "PullRequestEvent":
		var p prEventPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return models.Event{}, false
		}
		var kind models.EventKind
		switch p.Action {
		case "opened":
			kind = models.EventPROpened
		case "closed":
			if p.PullRequest.Merged {
				kind = models.EventPRMerged
			} else {
				kind = models.EventPRClosed
			}
		default:
			return models.Event{}, false
		}
		return models.Event{
			Kind:      kind,
			Title:     p.PullRequest.Title,
			Detail:    fmt.Sprintf("#%d", p.PullRequest.Number),
			URL:       p.PullRequest.HTMLURL,
			Actor:     actor,
			Timestamp: ts,
		}, true

	case "PushEvent":
		var p pushEventPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return models.Event{}, false
		}
		branch := strings.TrimPrefix(p.Ref, "refs/heads/")
		return models.Event{
			Kind:      models.EventPush,
			Title:     fmt.Sprintf("Push to %s", branch),
			Detail:    p.Ref,
			Actor:     actor,
			Timestamp: ts,
		}, true

	case "StatusEvent":
		var p statusEventPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return models.Event{}, false
		}
		var kind models.EventKind
		switch p.State {
		case "success":
			kind = models.EventCISuccess
		case "failure", "error":
			kind = models.EventCIFailed
		default:
			return models.Event{}, false
		}
		return models.Event{
			Kind:      kind,
			Title:     p.Name,
			Actor:     actor,
			Timestamp: ts,
		}, true

	case "CreateEvent":
		var p branchRefPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return models.Event{}, false
		}
		if p.RefType != "branch" {
			return models.Event{}, false
		}
		return models.Event{
			Kind:      models.EventBranchCreated,
			Title:     fmt.Sprintf("Branch created: %s", p.Ref),
			Detail:    p.Ref,
			Actor:     actor,
			Timestamp: ts,
		}, true

	case "DeleteEvent":
		var p branchRefPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return models.Event{}, false
		}
		if p.RefType != "branch" {
			return models.Event{}, false
		}
		return models.Event{
			Kind:      models.EventBranchDeleted,
			Title:     fmt.Sprintf("Branch deleted: %s", p.Ref),
			Detail:    p.Ref,
			Actor:     actor,
			Timestamp: ts,
		}, true

	default:
		return models.Event{}, false
	}
}
