package codeup

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMainModelTracksWindowSize(t *testing.T) {
	m := NewMainModel(&Config{}, TUIOptions{})

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
	if cmd != nil {
		t.Fatal("WindowSizeMsg should not produce a command")
	}

	got := updated.(MainModel)
	if got.width != 48 || got.height != 12 {
		t.Fatalf("size = %dx%d, want 48x12", got.width, got.height)
	}
}

func TestTruncateText(t *testing.T) {
	if got := truncateText("abcdef", 4); got != "abc…" {
		t.Fatalf("truncateText() = %q, want %q", got, "abc…")
	}
	if got := truncateText("中文分支名称", 9); got != "中文分支…" {
		t.Fatalf("truncateText() = %q, want %q", got, "中文分支…")
	}
	if got := truncateText("abc", 0); got != "" {
		t.Fatalf("truncateText max 0 = %q, want empty", got)
	}
}

func TestVisibleRangeKeepsCursorVisible(t *testing.T) {
	start, end := visibleRange(10, 8, 4)
	if start != 5 || end != 9 {
		t.Fatalf("visibleRange = %d,%d, want 5,9", start, end)
	}

	start, end = visibleRange(3, 1, 10)
	if start != 0 || end != 3 {
		t.Fatalf("visibleRange = %d,%d, want 0,3", start, end)
	}
}

func TestRepoSelectorViewUsesVisibleHeightAndTruncates(t *testing.T) {
	repos := make([]RepoConfig, 8)
	for i := range repos {
		repos[i] = RepoConfig{Name: fmt.Sprintf("repo-%02d-with-a-very-long-name", i)}
	}

	m := NewRepoSelectorModel(repos)
	m.cursor = 6
	view := m.ViewWithSize(18, 5)

	if strings.Contains(view, "repo-00") {
		t.Fatalf("view should not include off-screen first repo:\n%s", view)
	}
	if !strings.Contains(view, "repo-06") {
		t.Fatalf("view should keep cursor visible:\n%s", view)
	}
	if !strings.Contains(view, "…") {
		t.Fatalf("view should truncate long repo names:\n%s", view)
	}
}

func TestBranchViewUsesVisibleHeightAndResponsiveProgress(t *testing.T) {
	candidates := make([]Candidate, 9)
	for i := range candidates {
		candidates[i] = Candidate{
			RepoName:   fmt.Sprintf("repo-%02d-with-a-long-name", i),
			BranchName: fmt.Sprintf("feature/super-long-branch-name-%02d", i),
		}
	}

	m := NewBranchModel(candidates, TUIOptions{})
	m.cursor = 7
	view := m.ViewWithSize(28, 6)

	if strings.Contains(view, "repo-00") {
		t.Fatalf("view should not include off-screen first branch:\n%s", view)
	}
	if !strings.Contains(view, "repo-07") {
		t.Fatalf("view should keep cursor visible:\n%s", view)
	}
	if !strings.Contains(view, "…") {
		t.Fatalf("view should truncate long branch lines:\n%s", view)
	}

	m.mode = ModeDeleting
	m.deleting = 1
	m.deleteTotal = 2
	view = m.ViewWithSize(24, 4)
	for _, line := range strings.Split(view, "\n") {
		if plainLen(line) > 24 {
			t.Fatalf("line exceeds width 24 (%d): %q\n%s", plainLen(line), line, view)
		}
	}
}
