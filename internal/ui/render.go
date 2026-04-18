// Package ui provides terminal-friendly rendering helpers backed by lipgloss.
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"

	"github.com/lucasvavon/gtc/internal/models"
)

// ── Palette ───────────────────────────────────────────────────────────────────

var (
	colorGreen  = lipgloss.Color("2")
	colorRed    = lipgloss.Color("1")
	colorViolet = lipgloss.Color("5")
	colorYellow = lipgloss.Color("3")
	colorGray   = lipgloss.Color("240")
	colorWhite  = lipgloss.Color("255")
)

// ── Base styles ───────────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Padding(0, 1)

	styleBorder = lipgloss.NewStyle().Foreground(colorGray)

	styleCell = lipgloss.NewStyle().Padding(0, 1)

	styleBold = lipgloss.NewStyle().Bold(true)

	styleKeyLabel = lipgloss.NewStyle().Bold(true).Foreground(colorGray).Width(16)
)

// ── Low-level helpers ─────────────────────────────────────────────────────────

// termWidth returns the current terminal width, defaulting to 120.
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 120
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// ── Semantic color helpers ────────────────────────────────────────────────────

func prStateColored(state string) string {
	switch state {
	case "open":
		return lipgloss.NewStyle().Foreground(colorGreen).Render(state)
	case "closed":
		return lipgloss.NewStyle().Foreground(colorRed).Render(state)
	case "merged":
		return lipgloss.NewStyle().Foreground(colorViolet).Render(state)
	default:
		return state
	}
}

func ciStateColored(state string) string {
	switch state {
	case "success":
		return lipgloss.NewStyle().Foreground(colorGreen).Render("✓ " + state)
	case "failure", "error":
		return lipgloss.NewStyle().Foreground(colorRed).Render("✗ " + state)
	case "pending":
		return lipgloss.NewStyle().Foreground(colorYellow).Render("◌ " + state)
	default:
		return state
	}
}

// ── Table factory ─────────────────────────────────────────────────────────────

func newTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(styleBorder).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return styleHeader
			}
			return styleCell
		}).
		Headers(headers...)
}

// ── Public render functions ───────────────────────────────────────────────────

// RenderPRTable renders a coloured table of pull requests.
// The title column adapts to the terminal width; state is colour-coded
// (green = open, red = closed, violet = merged).
func RenderPRTable(prs []models.PullRequest) string {
	// Fixed columns total ≈ 5+15+8+32+10 = 70 chars + borders; remainder for title.
	titleWidth := max(20, termWidth()-75)

	t := newTable("ID", "TITLE", "AUTHOR", "STATE", "BRANCH", "CI")
	for _, pr := range prs {
		branch := pr.SourceBranch + " → " + pr.TargetBranch
		t.Row(
			fmt.Sprintf("%d", pr.ID),
			truncate(pr.Title, titleWidth),
			pr.Author,
			prStateColored(pr.State),
			truncate(branch, 30),
			pr.CIStatus,
		)
	}
	return t.String()
}

// RenderBranchTable renders a table of branches.
// The default branch is marked ✓ (green) and protected branches show 🔒.
func RenderBranchTable(branches []models.Branch) string {
	t := newTable("NAME", "DEFAULT", "PROTECTED", "LAST COMMIT")
	for _, b := range branches {
		def := "·"
		if b.IsDefault {
			def = lipgloss.NewStyle().Foreground(colorGreen).Render("✓")
		}
		prot := lipgloss.NewStyle().Foreground(colorRed).Render("✗")
		if b.Protected {
			prot = "🔒"
		}
		t.Row(b.Name, def, prot, b.LastCommit.ShortSHA)
	}
	return t.String()
}

// RenderCommitTable renders a table of commits with relative dates.
// The message column shows only the first line, truncated to fit the terminal.
func RenderCommitTable(commits []models.Commit) string {
	// Fixed columns: 7+15+10 = 32 chars + borders; remainder for message.
	msgWidth := max(20, termWidth()-45)

	t := newTable("SHA", "MESSAGE", "AUTHOR", "DATE")
	for _, c := range commits {
		t.Row(
			c.ShortSHA,
			truncate(firstLine(c.Message), msgWidth),
			c.Author,
			relativeTime(c.Date),
		)
	}
	return t.String()
}

// RenderCIStatus renders an overall ref/state summary followed by a
// per-job table. Job statuses are colour-coded (green/yellow/red).
func RenderCIStatus(status *models.CIStatus) string {
	var sb strings.Builder

	sb.WriteString(styleKeyLabel.Render("Ref:"))
	sb.WriteString("  " + status.Ref + "\n")
	sb.WriteString(styleKeyLabel.Render("Overall:"))
	sb.WriteString("  " + ciStateColored(status.State) + "\n")

	if len(status.Jobs) == 0 {
		return sb.String()
	}

	sb.WriteString("\n")

	t := newTable("JOB", "STAGE", "STATUS", "URL")
	for _, j := range status.Jobs {
		t.Row(
			j.Name,
			j.Stage,
			ciStateColored(j.Status),
			truncate(j.URL, 50),
		)
	}
	sb.WriteString(t.String())
	return sb.String()
}

// RenderPRDetail renders a single pull request as a labelled key-value block.
func RenderPRDetail(pr *models.PullRequest) string {
	labels := strings.Join(pr.Labels, ", ")

	rows := [][]string{
		{"Title", pr.Title},
		{"Author", pr.Author},
		{"State", prStateColored(pr.State)},
		{"Source branch", pr.SourceBranch},
		{"Target branch", pr.TargetBranch},
		{"Draft", fmt.Sprintf("%v", pr.Draft)},
		{"Labels", labels},
		{"CI", pr.CIStatus},
		{"URL", pr.URL},
		{"Created", pr.CreatedAt.Format("2006-01-02 15:04")},
		{"Updated", pr.UpdatedAt.Format("2006-01-02 15:04")},
	}

	t := newTable("FIELD", "VALUE")
	for _, r := range rows {
		t.Row(r[0], r[1])
	}
	return styleBold.Render(fmt.Sprintf("PR #%d", pr.ID)) + "\n" + t.String()
}
