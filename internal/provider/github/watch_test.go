package github

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/lucasvavon/gtc/internal/models"
)

// watchEventFixture builds a minimal GitHub Events API JSON object.
// payload is serialised as a raw JSON value so that go-github stores it
// as *json.RawMessage and our mapGitHubEvent helper can decode it.
func watchEventFixture(id, eventType, actor string, payload map[string]any) map[string]any {
	payloadBytes, _ := json.Marshal(payload)
	return map[string]any{
		"id":         id,
		"type":       eventType,
		"actor":      map[string]any{"login": actor},
		"created_at": fixedTime().Format(time.RFC3339),
		"payload":    json.RawMessage(payloadBytes),
	}
}

// TestWatch_TwoPolls simulates two successive polls against a fake GitHub
// Events endpoint and verifies that:
//   - events from poll 1 are emitted
//   - only the new event from poll 2 is emitted (no duplicate from poll 1)
//   - the channel is closed after the context is cancelled
func TestWatch_TwoPolls(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	callCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc(repoPath()+"/events", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()

		var events []map[string]any
		switch n {
		case 1:
			// Poll 1: a single push event (ID "1").
			events = []map[string]any{
				watchEventFixture("1", "PushEvent", "alice", map[string]any{
					"ref": "refs/heads/main",
				}),
			}
		default:
			// Poll 2+: a new PR-opened event (ID "2") plus the existing push (ID "1").
			// Only ID "2" should be emitted; ID "1" was already seen.
			events = []map[string]any{
				watchEventFixture("2", "PullRequestEvent", "bob", map[string]any{
					"action": "opened",
					"pull_request": map[string]any{
						"number":   42,
						"title":    "feat: add watch",
						"html_url": "https://github.com/owner/repo/pull/42",
						"merged":   false,
					},
				}),
				watchEventFixture("1", "PushEvent", "alice", map[string]any{
					"ref": "refs/heads/main",
				}),
			}
		}

		// Omit X-Poll-Interval so the test interval (5 ms) is not overridden.
		writeJSON(w, http.StatusOK, events)
	})

	_, g := newTestServer(t, mux)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := g.Watch(ctx, models.WatchOptions{
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Watch() unexpected error: %v", err)
	}

	// Collect exactly 2 events, then cancel so the goroutine exits and the
	// channel closes promptly.
	var received []models.Event
	for e := range ch {
		received = append(received, e)
		if len(received) == 2 {
			cancel()
		}
	}

	if len(received) != 2 {
		t.Fatalf("got %d events, want 2", len(received))
	}

	// ── Poll 1: PushEvent ─────────────────────────────────────────────────────
	got := received[0]
	if got.Kind != models.EventPush {
		t.Errorf("event[0].Kind = %q, want %q", got.Kind, models.EventPush)
	}
	if got.Actor != "alice" {
		t.Errorf("event[0].Actor = %q, want %q", got.Actor, "alice")
	}
	if got.Detail != "refs/heads/main" {
		t.Errorf("event[0].Detail = %q, want %q", got.Detail, "refs/heads/main")
	}

	// ── Poll 2: PullRequestEvent (opened) — no duplicate PushEvent ───────────
	got = received[1]
	if got.Kind != models.EventPROpened {
		t.Errorf("event[1].Kind = %q, want %q", got.Kind, models.EventPROpened)
	}
	if got.Actor != "bob" {
		t.Errorf("event[1].Actor = %q, want %q", got.Actor, "bob")
	}
	if got.Title != "feat: add watch" {
		t.Errorf("event[1].Title = %q, want %q", got.Title, "feat: add watch")
	}
	if got.Detail != "#42" {
		t.Errorf("event[1].Detail = %q, want %q", got.Detail, "#42")
	}
}

// TestWatch_XPollInterval verifies that a non-zero X-Poll-Interval header is
// parsed and used as the next timer value (smoke-test: we just check that Watch
// doesn't error and closes the channel on cancellation).
func TestWatch_XPollInterval(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(repoPath()+"/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Poll-Interval", "60")
		writeJSON(w, http.StatusOK, []map[string]any{})
	})

	_, g := newTestServer(t, mux)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch, err := g.Watch(ctx, models.WatchOptions{Interval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Watch() unexpected error: %v", err)
	}

	// Drain; the channel must close once the context expires.
	for range ch {
	}
}

// TestWatch_ContextCancelled ensures that Watch closes the channel immediately
// when a pre-cancelled context is passed.
func TestWatch_ContextCancelled(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(repoPath()+"/events", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]any{})
	})

	_, g := newTestServer(t, mux)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	ch, err := g.Watch(ctx, models.WatchOptions{Interval: time.Minute})
	if err != nil {
		t.Fatalf("Watch() unexpected error: %v", err)
	}

	// Channel must close (possibly after one failed poll attempt).
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed, got an event")
		}
	case <-time.After(2 * time.Second):
		t.Error("channel did not close after context cancellation")
	}
}
