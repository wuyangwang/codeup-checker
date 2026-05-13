package codeup

type Config struct {
	OrganizationId  string       `json:"organizationId" yaml:"organizationId"`
	AccessToken     string       `json:"accessToken" yaml:"accessToken"`
	TargetBranch    string       `json:"targetBranch,omitempty" yaml:"targetBranch,omitempty"`
	Repositories    []RepoConfig `json:"repositories" yaml:"repositories"`
	ExcludePatterns []string     `json:"excludePatterns,omitempty" yaml:"excludePatterns,omitempty"`
}

func (c Config) GetTargetBranch() string {
	if c.TargetBranch != "" {
		return c.TargetBranch
	}
	return "master"
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
