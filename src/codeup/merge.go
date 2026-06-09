package codeup

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	mergeTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorTitle)).
			MarginBottom(1)

	mergeStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Bold(true)

	mergeSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSuccess)).
				Bold(true)

	mergeErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
			Bold(true)

	mergeWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarn)).
			Bold(true)

	mergeMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))
)

type mergeKeyMap struct {
	Merge key.Binding
	Back  key.Binding
	Quit  key.Binding
	Scan  key.Binding
}

var mergeKeys = mergeKeyMap{
	Merge: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("M", "合并/评审"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("ESC", "返回菜单"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("Q", "退出"),
	),
	Scan: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("S", "扫描分支"),
	),
}

type MergeDoneMsg struct {
	Success bool
	Error   error
}

type MergeResultMsg struct {
	CR           *ChangeRequest
	Repository   *Repository
	Existing     bool
	CommitsCount int
}

type MergeModel struct {
	repo          RepoConfig
	repository    *Repository
	sourceBranch  string
	targetBranch  string
	status        MergeStatus
	cr            *ChangeRequest
	err           error
	opts          TUIOptions
	msgChan       chan tea.Msg
	done          bool
	existing      bool
	scanTriggered bool
	commitsCount  int
}

func NewMergeModel(repo RepoConfig, opts TUIOptions) MergeModel {
	return MergeModel{
		repo:         repo,
		sourceBranch: "production",
		targetBranch: "master",
		status:       MergeStatusIdle,
		opts:         opts,
		msgChan:      make(chan tea.Msg, 10),
	}
}

func (m MergeModel) Init() tea.Cmd {
	return m.startCreateChangeRequest
}

func (m MergeModel) startCreateChangeRequest() tea.Msg {
	ctx := context.Background()

	repoID := m.repo.Identity()

	repos, err := m.opts.Client.ListRepositories(ctx, repoID, 1, 10)
	if err != nil {
		return MergeDoneMsg{Success: false, Error: fmt.Errorf("获取仓库信息失败: %w", err)}
	}

	var repository *Repository
	for i := range repos {
		r := &repos[i]
		if r.Name == repoID || r.Path == repoID ||
			r.PathWithNamespace == repoID || fmt.Sprintf("%d", r.ID) == repoID {
			repository = r
			break
		}
	}
	if repository == nil && len(repos) == 1 {
		repository = &repos[0]
	}
	if repository == nil {
		return MergeDoneMsg{Success: false, Error: fmt.Errorf("未找到仓库: %s", repoID)}
	}

	crs, err := m.opts.Client.ListChangeRequests(ctx)
	if err == nil {
		for _, cr := range crs {
			if cr.ProjectId == repository.ID &&
				cr.SourceBranch == m.sourceBranch &&
				cr.TargetBranch == m.targetBranch {
				return MergeResultMsg{CR: &cr, Repository: repository, Existing: true}
			}
		}
	}

	repoIdentity := repositoryIdentity(repository)

	// 使用分支比对代替立即创建合并请求
	compare, err := m.opts.Client.GetCompareDetail(ctx, repoIdentity, m.targetBranch, m.sourceBranch)
	if err != nil {
		return MergeDoneMsg{Success: false, Error: fmt.Errorf("分支对比失败: %w", err)}
	}

	commitsCount := len(compare.Commits)

	return MergeResultMsg{
		CR:           nil,
		Repository:   repository,
		Existing:     false,
		CommitsCount: commitsCount,
	}
}

func (m MergeModel) Update(msg tea.Msg) (MergeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, mergeKeys.Quit):
			return m, tea.Quit

		case key.Matches(msg, mergeKeys.Back):
			if m.status != MergeStatusCreating && m.status != MergeStatusMerging {
				m.done = true
				return m, nil
			}

		case key.Matches(msg, mergeKeys.Merge):
			if m.status == MergeStatusCanMerge || m.status == MergeStatusNeedReview {
				return m.startMergeOrReview()
			}

		case key.Matches(msg, mergeKeys.Scan):
			if m.status == MergeStatusDone || m.status == MergeStatusAlreadyMerged {
				m.scanTriggered = true
				return m, nil
			}
		}

	case *ChangeRequest:
		m.cr = msg
		m.status = m.evaluateStatus(msg)
		return m, nil

	case MergeResultMsg:
		m.cr = msg.CR
		m.repository = msg.Repository
		m.existing = msg.Existing
		m.commitsCount = msg.CommitsCount
		m.status = m.evaluateStatus(msg.CR)
		return m, nil

	case MergeDoneMsg:
		if msg.Error != nil {
			m.err = msg.Error
			m.status = MergeStatusError
		} else {
			m.status = MergeStatusDone
		}
		return m, nil
	}

	return m, nil
}

func (m MergeModel) evaluateStatus(cr *ChangeRequest) MergeStatus {
	if cr == nil {
		if m.commitsCount == 0 {
			return MergeStatusNoChanges
		}
		return MergeStatusCanMerge
	}

	state := cr.Status
	if state == "" {
		state = cr.State
	}

	if state == "CLOSED" {
		return MergeStatusClosed
	}

	if state == "MERGED" {
		return MergeStatusAlreadyMerged
	}

	if !m.existing && cr.Ahead == 0 {
		return MergeStatusNoChanges
	}

	if cr.HasConflict || cr.ConflictCheckStatus == "HAS_CONFLICT" {
		return MergeStatusHasConflict
	}

	if cr.ConflictCheckStatus == "CHECKING" {
		return MergeStatusChecking
	}

	if state == "TO_BE_MERGED" && cr.AllRequirementsPass {
		return MergeStatusCanMerge
	}

	if state == "UNDER_REVIEW" || !cr.AllRequirementsPass {
		return MergeStatusNeedReview
	}

	return MergeStatusChecking
}

func (m MergeModel) startMergeOrReview() (MergeModel, tea.Cmd) {
	m.status = MergeStatusMerging
	m.msgChan = make(chan tea.Msg, 2)

	go func() {
		ctx := context.Background()
		repoID := repositoryIdentity(m.repository)

		cr := m.cr
		if cr == nil {
			// 1. 创建合并请求
			commitTitle := ""
			sourceBranch, err := m.opts.Client.GetBranch(ctx, repoID, m.sourceBranch)
			if err == nil && sourceBranch.Commit != nil {
				commitTitle = sourceBranch.Commit.Title
			}

			title := commitTitle
			if title == "" {
				title = fmt.Sprintf("Merge %s to %s", m.sourceBranch, m.targetBranch)
			}

			req := CreateChangeRequestReq{
				Title:           title,
				Description:     fmt.Sprintf("Auto-merge from %s to %s", m.sourceBranch, m.targetBranch),
				SourceBranch:    m.sourceBranch,
				TargetBranch:    m.targetBranch,
				SourceProjectID: m.repository.ID,
				TargetProjectID: m.repository.ID,
				CreateFrom:      "WEB",
			}

			var createErr error
			cr, createErr = m.opts.Client.CreateChangeRequest(ctx, repoID, req)
			if createErr != nil {
				m.msgChan <- MergeDoneMsg{Success: false, Error: fmt.Errorf("创建合并请求失败: %w", createErr)}
				return
			}
		}

		// 检查是否需要评审
		needReview := cr.Status == "UNDER_REVIEW" || !cr.AllRequirementsPass
		if needReview {
			reviewReq := ReviewChangeRequestReq{
				ReviewOpinion: "PASS",
			}
			if err := m.opts.Client.ReviewChangeRequest(ctx, repoID, cr.LocalID, reviewReq); err != nil {
				m.msgChan <- MergeDoneMsg{Success: false, Error: fmt.Errorf("评审合并请求失败: %w", err)}
				return
			}
		}

		mergeReq := MergeChangeRequestReq{
			MergeType:          "no-fast-forward",
			RemoveSourceBranch: false,
		}
		_, err := m.opts.Client.MergeChangeRequest(ctx, repoID, cr.LocalID, mergeReq)
		if err != nil {
			m.msgChan <- MergeDoneMsg{Success: false, Error: fmt.Errorf("合并失败: %w", err)}
			return
		}

		m.msgChan <- MergeDoneMsg{Success: true}
	}()

	return m, m.listenMerge()
}

func (m MergeModel) listenMerge() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m MergeModel) View() string {
	var s strings.Builder

	s.WriteString(mergeTitleStyle.Render("=== 代码合并 ==="))
	s.WriteString("\n\n")

	s.WriteString(fmt.Sprintf("仓库: %s\n", m.repo.DisplayName()))
	s.WriteString(fmt.Sprintf("源分支: %s → 目标分支: %s\n\n", m.sourceBranch, m.targetBranch))

	switch m.status {
	case MergeStatusIdle, MergeStatusCreating:
		s.WriteString(mergeStatusStyle.Render("正在检查分支对比状态..."))
		s.WriteString("\n")

	case MergeStatusChecking:
		s.WriteString(mergeStatusStyle.Render("正在检查合并状态..."))
		s.WriteString("\n")

	case MergeStatusNoChanges:
		s.WriteString(mergeMutedStyle.Render("没有新的提交需要合并"))
		s.WriteString("\n")

	case MergeStatusClosed:
		s.WriteString(mergeMutedStyle.Render("合并请求已关闭"))
		s.WriteString("\n")

	case MergeStatusAlreadyMerged:
		s.WriteString(mergeMutedStyle.Render("已经合并过了"))
		s.WriteString("\n\n")
		s.WriteString(mergeStatusStyle.Render("按 S 扫描该仓库 of 已合并分支"))
		s.WriteString("\n")

	case MergeStatusHasConflict:
		s.WriteString(mergeErrorStyle.Render("存在合并冲突，无法自动合并"))
		s.WriteString("\n")
		if m.cr != nil {
			s.WriteString(fmt.Sprintf("详情: %s\n", m.cr.WebURL))
		}

	case MergeStatusCanMerge:
		s.WriteString(mergeSuccessStyle.Render("可以合并"))
		s.WriteString("\n")
		if m.cr != nil {
			s.WriteString(fmt.Sprintf("领先 %d 个提交\n", m.cr.Ahead))
			s.WriteString(fmt.Sprintf("详情: %s\n", m.cr.WebURL))
		} else {
			s.WriteString(fmt.Sprintf("领先 %d 个提交\n", m.commitsCount))
		}
		s.WriteString("\n")
		s.WriteString(mergeStatusStyle.Render("按 M 执行合并"))

	case MergeStatusNeedReview:
		s.WriteString(mergeWarnStyle.Render("需要评审"))
		s.WriteString("\n")
		if m.cr != nil {
			s.WriteString(fmt.Sprintf("领先 %d 个提交\n", m.cr.Ahead))
			s.WriteString(fmt.Sprintf("详情: %s\n", m.cr.WebURL))
		} else {
			s.WriteString(fmt.Sprintf("领先 %d 个提交\n", m.commitsCount))
		}
		s.WriteString("\n")
		s.WriteString(mergeStatusStyle.Render("按 M 评审并合并"))

	case MergeStatusMerging:
		s.WriteString(mergeStatusStyle.Render("正在执行合并..."))
		s.WriteString("\n")

	case MergeStatusDone:
		s.WriteString(mergeSuccessStyle.Render("合并完成！"))
		s.WriteString("\n")
		if m.cr != nil {
			s.WriteString(fmt.Sprintf("详情: %s\n", m.cr.WebURL))
		}
		s.WriteString("\n")
		s.WriteString(mergeStatusStyle.Render("按 S 扫描该仓库的已合并分支"))
		s.WriteString("\n")

	case MergeStatusError:
		s.WriteString(mergeErrorStyle.Render("操作失败"))
		s.WriteString("\n")
		if m.err != nil {
			s.WriteString(mergeErrorStyle.Render(m.err.Error()))
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (m MergeModel) ViewWithSize(width int) string {
	width = effectiveWidth(width)
	return strings.Join(truncateLines(strings.Split(m.View(), "\n"), width), "\n")
}
