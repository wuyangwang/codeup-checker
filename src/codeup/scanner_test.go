package codeup

import "testing"

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		branch  string
		want    bool
	}{
		{name: "exact match", pattern: "feature/a", branch: "feature/a", want: true},
		{name: "prefix wildcard match", pattern: "feature/*", branch: "feature/a", want: true},
		{name: "prefix wildcard no match", pattern: "feature/*", branch: "bugfix/a", want: false},
		{name: "empty pattern", pattern: "", branch: "feature/a", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPattern(tt.pattern, tt.branch); got != tt.want {
				t.Fatalf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.branch, got, tt.want)
			}
		})
	}
}

func TestIsExcludedBranch(t *testing.T) {
	patterns := []string{"feature/*", "hotfix/1"}

	if !isExcludedBranch("feature/a", patterns) {
		t.Fatal("feature/a should be excluded")
	}
	if !isExcludedBranch("hotfix/1", patterns) {
		t.Fatal("hotfix/1 should be excluded")
	}
	if isExcludedBranch("release/v1", patterns) {
		t.Fatal("release/v1 should not be excluded")
	}
}

func TestIsProtectedBranch(t *testing.T) {
	if !isProtectedBranch("master", Branch{}) {
		t.Fatal("master should be protected")
	}
	if !isProtectedBranch("feature/a", Branch{Protected: true}) {
		t.Fatal("branch with protected=true should be protected")
	}
	if isProtectedBranch("feature/a", Branch{Protected: false}) {
		t.Fatal("branch with protected=false should not be protected")
	}
	if !isProtectedBranch("feature/a", Branch{Protected: "true"}) {
		t.Fatal("branch with protected=\"true\" should be protected")
	}
}
