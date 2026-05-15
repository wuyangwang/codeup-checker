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

// ChangeRequest 表示合并请求
type ChangeRequest struct {
	LocalID             int    `json:"localId"`
	Title               string `json:"title"`
	SourceBranch        string `json:"sourceBranch"`
	TargetBranch        string `json:"targetBranch"`
	SourceProjectID     int64  `json:"sourceProjectId"`
	TargetProjectID     int64  `json:"targetProjectId"`
	ProjectId           int64  `json:"projectId"`
	Status              string `json:"status"`
	State               string `json:"state"`
	ConflictCheckStatus string `json:"conflictCheckStatus"`
	HasConflict         bool   `json:"hasConflict"`
	Ahead               int    `json:"ahead"`
	Behind              int    `json:"behind"`
	AllRequirementsPass bool   `json:"allRequirementsPass"`
	DetailURL           string `json:"detailUrl"`
	WebURL              string `json:"webUrl"`
}

// MergeStatus 合并状态枚举
type MergeStatus int

const (
	MergeStatusIdle MergeStatus = iota
	MergeStatusCreating
	MergeStatusChecking
	MergeStatusCanMerge
	MergeStatusNeedReview
	MergeStatusUnderReview
	MergeStatusHasConflict
	MergeStatusNoChanges
	MergeStatusMerging
	MergeStatusDone
	MergeStatusClosed
	MergeStatusAlreadyMerged
	MergeStatusError
)

// CreateChangeRequestReq 创建合并请求参数
type CreateChangeRequestReq struct {
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	SourceBranch    string `json:"sourceBranch"`
	TargetBranch    string `json:"targetBranch"`
	SourceProjectID int64  `json:"sourceProjectId"`
	TargetProjectID int64  `json:"targetProjectId"`
	SourceCommit    string `json:"sourceCommit,omitempty"`
	SourceCommitID  string `json:"sourceCommitId,omitempty"`
	CreateFrom      string `json:"createFrom,omitempty"`
}

// ReviewChangeRequestReq 评审合并请求参数
type ReviewChangeRequestReq struct {
	ReviewOpinion string `json:"reviewOpinion"`
	ReviewComment string `json:"reviewComment,omitempty"`
}

// MergeChangeRequestReq 合并合并请求参数
type MergeChangeRequestReq struct {
	MergeType          string `json:"mergeType"`
	MergeMessage       string `json:"mergeMessage,omitempty"`
	RemoveSourceBranch bool   `json:"removeSourceBranch"`
}
