package codeup

type Config struct {
	OrganizationId  string       `json:"organizationId" yaml:"organizationId"`
	AccessToken     string       `json:"accessToken" yaml:"accessToken"`
	TargetBranch    string       `json:"targetBranch,omitempty" yaml:"targetBranch,omitempty"`
	Repositories    []RepoConfig `json:"repositories" yaml:"repositories"`
	ExcludePatterns []string     `json:"excludePatterns,omitempty" yaml:"excludePatterns,omitempty"`
	// ScanConcurrency 仓库扫描并发数（默认 5）
	ScanConcurrency int `json:"scanConcurrency,omitempty" yaml:"scanConcurrency,omitempty"`
	// CompareConcurrency 分支比较并发数（默认 10）
	CompareConcurrency int `json:"compareConcurrency,omitempty" yaml:"compareConcurrency,omitempty"`
}

func (c Config) GetTargetBranch() string {
	if c.TargetBranch != "" {
		return c.TargetBranch
	}
	return "master"
}

func (c Config) GetScanConcurrency() int {
	if c.ScanConcurrency > 0 {
		return c.ScanConcurrency
	}
	return 5
}

func (c Config) GetCompareConcurrency() int {
	if c.CompareConcurrency > 0 {
		return c.CompareConcurrency
	}
	return 10
}

type RepoConfig struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

func (r RepoConfig) Identity() string {
	if r.ID != "" {
		return r.ID
	}
	return r.Name
}

func (r RepoConfig) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
}

type Candidate struct {
	RepoName   string
	RepoID     string
	BranchName string
}

type Result struct {
	Success []Candidate
	Failed  []FailedDeletion
}

type FailedDeletion struct {
	Candidate Candidate
	Error     error
}
