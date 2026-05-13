package codeup

type Config struct {
	OrganizationId  string       `json:"organization_id"`
	AccessToken     string       `json:"access_token"`
	Repositories    []RepoConfig `json:"repositories"`
	ExcludePatterns []string     `json:"exclude_patterns,omitempty"`
}

type RepoConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
