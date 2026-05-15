package codeup

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppState 定义应用状态
type AppState int

const (
	StateMenu AppState = iota
	StateRepoSelect
	StateScanning
	StateBranchSelect
	StateFutureTODO
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeConfirm
	ModeDeleting
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Space   key.Binding
	Select  key.Binding
	Quit    key.Binding
	Confirm key.Binding
	Execute key.Binding
	Open    key.Binding
	Esc     key.Binding
	Enter   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/K", "上移"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/J", "下移"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("SPACE", "选择/取消"),
	),
	Select: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("A", "全选/反选"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("Q", "退出"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("D", "确认删除"),
	),
	Execute: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("D", "执行删除"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("O", "打开配置目录"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("ESC", "取消"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("ENTER", "确认"),
	),
}

type TUIOptions struct {
	Client *CodeupClient
	Ctx    context.Context
	DryRun bool
}

type DeletionProgressMsg struct {
	Total     int
	Completed int
	Current   string
	Success   bool
	Error     error
}

type DeletionDoneMsg struct {
	Result Result
}

// BranchModel 分支选择子模型
type BranchModel struct {
	candidates  []Candidate
	selected    map[int]bool
	cursor      int
	mode        Mode
	result      Result
	quitting    bool
	back        bool
	allSelected bool
	opts        TUIOptions
	deleting    int
	deleteTotal int
	deleteDone  bool
	msgChan     chan tea.Msg
}

func NewBranchModel(candidates []Candidate, opts TUIOptions) BranchModel {
	return BranchModel{
		candidates:  candidates,
		selected:    make(map[int]bool),
		cursor:      0,
		mode:        ModeNormal,
		allSelected: false,
		opts:        opts,
	}
}

func (m BranchModel) Init() tea.Cmd {
	return nil
}

func (m BranchModel) Update(msg tea.Msg) (BranchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.mode == ModeNormal && m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, keys.Down):
			if m.mode == ModeNormal && m.cursor < len(m.candidates)-1 {
				m.cursor++
			}

		case key.Matches(msg, keys.Space):
			if m.mode == ModeNormal {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}

		case key.Matches(msg, keys.Select):
			if m.mode == ModeNormal {
				if m.allSelected {
					m.selected = make(map[int]bool)
					m.allSelected = false
				} else {
					for i := range m.candidates {
						m.selected[i] = true
					}
					m.allSelected = true
				}
			}

		case key.Matches(msg, keys.Confirm):
			if m.mode == ModeNormal {
				hasSelected := false
				for _, selected := range m.selected {
					if selected {
						hasSelected = true
						break
					}
				}
				if hasSelected {
					m.mode = ModeConfirm
				}
			} else if m.mode == ModeConfirm {
				return m.startDeletion()
			}

		case key.Matches(msg, keys.Esc):
			if m.mode == ModeConfirm {
				m.mode = ModeNormal
			} else if m.mode == ModeNormal {
				m.back = true
			}
		}

	case DeletionProgressMsg:
		m.deleting = msg.Completed
		m.deleteTotal = msg.Total
		return m, m.listen()

	case DeletionDoneMsg:
		m.result = msg.Result
		m.mode = ModeNormal
		m.deleteDone = true

		successSet := make(map[string]bool)
		for _, c := range msg.Result.Success {
			successSet[c.RepoName+":"+c.BranchName] = true
		}

		newCandidates := make([]Candidate, 0)
		for _, candidate := range m.candidates {
			key := candidate.RepoName + ":" + candidate.BranchName
			if !successSet[key] {
				newCandidates = append(newCandidates, candidate)
			}
		}

		m.candidates = newCandidates
		m.selected = make(map[int]bool)
		m.cursor = 0
		m.allSelected = false
		m.deleting = 0
		m.deleteTotal = 0
		return m, nil
	}

	return m, nil
}

func (m BranchModel) listen() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m BranchModel) startDeletion() (BranchModel, tea.Cmd) {
	var toDelete []Candidate
	for i, selected := range m.selected {
		if selected {
			toDelete = append(toDelete, m.candidates[i])
		}
	}

	m.mode = ModeDeleting
	m.deleteTotal = len(toDelete)
	m.deleting = 0
	m.msgChan = make(chan tea.Msg, m.deleteTotal+1)

	go runDeletionAsync(m.opts, toDelete, m.msgChan)
	return m, m.listen()
}

func runDeletionAsync(opts TUIOptions, toDelete []Candidate, msgChan chan tea.Msg) {
	result := Result{}
	semaphore := make(chan struct{}, 5)
	var mu sync.Mutex
	var completed int32

	var wg sync.WaitGroup
	for _, candidate := range toDelete {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(c Candidate) {
			defer wg.Done()
			defer func() { <-semaphore }()

			select {
			case <-opts.Ctx.Done():
				mu.Lock()
				result.Failed = append(result.Failed, FailedDeletion{Candidate: c, Error: opts.Ctx.Err()})
				mu.Unlock()
				current := atomic.AddInt32(&completed, 1)
				msgChan <- DeletionProgressMsg{
					Total:     len(toDelete),
					Completed: int(current),
				}
				return
			default:
			}

			if opts.DryRun {
				mu.Lock()
				result.Success = append(result.Success, c)
				mu.Unlock()
				current := atomic.AddInt32(&completed, 1)
				msgChan <- DeletionProgressMsg{
					Total:     len(toDelete),
					Completed: int(current),
				}
				return
			}

			err := opts.Client.DeleteBranch(opts.Ctx, c.RepoID, c.BranchName)

			mu.Lock()
			if err != nil {
				result.Failed = append(result.Failed, FailedDeletion{Candidate: c, Error: err})
			} else {
				result.Success = append(result.Success, c)
			}
			mu.Unlock()
			current := atomic.AddInt32(&completed, 1)
			msgChan <- DeletionProgressMsg{
				Total:     len(toDelete),
				Completed: int(current),
			}
		}(candidate)
	}

	wg.Wait()
	msgChan <- DeletionDoneMsg{Result: result}
}

// MainModel 统一状态机模型
type MainModel struct {
	state       AppState
	repoModel   RepoSelectorModel
	branchModel BranchModel
	scanLogs    []string
	opts        TUIOptions
	config      *Config
	err         error
	quitting    bool
	msgChan     chan tea.Msg
	menuCursor  int
}

func NewMainModel(cfg *Config, opts TUIOptions) MainModel {
	return MainModel{
		state:      StateMenu,
		repoModel:  NewRepoSelectorModel(cfg.Repositories),
		opts:       opts,
		config:     cfg,
		msgChan:    make(chan tea.Msg, 100),
		menuCursor: 0,
	}
}

func (m MainModel) Init() tea.Cmd {
	return nil
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			m.quitting = true
			return m, tea.Quit
		}
		if key.Matches(msg, keys.Open) {
			OpenConfigDir()
		}

		if m.state == StateMenu {
			switch {
			case key.Matches(msg, keys.Up):
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case key.Matches(msg, keys.Down):
				if m.menuCursor < 1 {
					m.menuCursor++
				}
			case key.Matches(msg, keys.Enter):
				if m.menuCursor == 0 {
					m.state = StateRepoSelect
				} else {
					m.state = StateFutureTODO
				}
			case key.Matches(msg, keys.Esc):
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		if m.state == StateFutureTODO {
			if key.Matches(msg, keys.Esc) {
				m.state = StateMenu
			}
			return m, nil
		}

	case ScanProgressMsg:
		m.scanLogs = append(m.scanLogs, msg.Message)
		if len(m.scanLogs) > 15 {
			m.scanLogs = m.scanLogs[len(m.scanLogs)-15:]
		}
		return m, m.listenScan()

	case ScanDoneMsg:
		if msg.Error != nil {
			m.err = msg.Error
		}
		if len(msg.Candidates) == 0 && m.err == nil {
			m.scanLogs = append(m.scanLogs, styleInfoText.Render("未找到已合并的分支。"))
		}
		m.branchModel = NewBranchModel(msg.Candidates, m.opts)
		m.state = StateBranchSelect
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case StateRepoSelect:
		var newRepoModel tea.Model
		newRepoModel, cmd = m.repoModel.Update(msg)
		m.repoModel = newRepoModel.(RepoSelectorModel)
		if m.repoModel.confirmed {
			selected := m.repoModel.GetSelected()
			m.state = StateScanning
			return m, m.startScanning(selected)
		}
		if m.repoModel.quitting {
			m.state = StateMenu
			m.repoModel.quitting = false
			return m, nil
		}

	case StateBranchSelect:
		m.branchModel, cmd = m.branchModel.Update(msg)
		if m.branchModel.back {
			m.state = StateMenu
			m.branchModel.back = false
			return m, nil
		}
	}

	return m, cmd
}

func (m MainModel) listenScan() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m MainModel) startScanning(repos []RepoConfig) tea.Cmd {
	return func() tea.Msg {
		// 先解析仓库
		if err := ResolveRepositories(m.opts.Ctx, m.opts.Client, repos); err != nil {
			return ScanDoneMsg{Error: err}
		}
		// 开始异步扫描
		go ScanRepositoriesAsync(m.opts.Ctx, m.opts.Client, m.config, repos, m.msgChan)
		return <-m.msgChan
	}
}

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTitle)).
			Bold(true).
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color(colorMuted))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color(colorMuted)).
			MarginTop(1)
)

func (m MainModel) View() string {
	if m.quitting {
		return "正在退出...\n"
	}

	var s strings.Builder

	// Header
	header := "Codeup 分支清理工具"
	s.WriteString(headerStyle.Render(header))
	s.WriteString("\n")

	// Body
	s.WriteString(m.renderContent())
	s.WriteString("\n")

	// Footer (仅在扫描及后续阶段显示带边框的页脚)
	if m.state > StateRepoSelect {
		s.WriteString(footerStyle.Render(m.renderFooter()))
	} else if m.state == StateRepoSelect {
		// 在仓库选择阶段，不显示边框页脚，但显示基础操作说明
		s.WriteString("\n" + m.renderFooter())
	}

	return s.String()
}

func (m MainModel) renderContent() string {
	switch m.state {
	case StateMenu:
		var sb strings.Builder
		sb.WriteString(styleTitleText.Render("=== 主菜单 ==="))
		sb.WriteString("\n\n")
		options := []string{"分支清理 (清理已合并的分支)", "代码合并 (TODO: 合并 Prod 到 Master)"}
		for i, opt := range options {
			cursor := "  "
			if m.menuCursor == i {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%d. %s", cursor, i+1, opt)
			if m.menuCursor == i {
				line = styleAccentText.Render(line)
			}
			sb.WriteString(line + "\n")
		}
		return sb.String()
	case StateRepoSelect:
		var sb strings.Builder
		sb.WriteString(m.repoModel.View())

		selected := m.repoModel.GetSelected()
		if len(selected) > 0 {
			sb.WriteString(styleAccentText.Render(fmt.Sprintf("\n已选择 %d 个仓库", len(selected))) + "\n")
		}
		return sb.String()
	case StateScanning:
		var sb strings.Builder
		sb.WriteString(styleInfoText.Render("正在扫描仓库...\n\n"))
		for _, log := range m.scanLogs {
			sb.WriteString(log + "\n")
		}
		return sb.String()
	case StateBranchSelect:
		return m.branchModel.View()
	case StateFutureTODO:
		return styleWarnText.Render("TODO: 合并/评审功能开发中...\n\n按 ESC 返回主菜单")
	default:
		return ""
	}
}

func (m MainModel) renderFooter() string {
	var sb strings.Builder

	// 帮助文本
	sb.WriteString(m.renderHelp())

	// 处理时显示 HTTP 请求数
	if m.state > StateRepoSelect {
		sb.WriteString("\n")
		sb.WriteString(styleMutedText.Render(fmt.Sprintf("HTTP 请求: %d", m.opts.Client.RequestCount())))
	}

	return sb.String()
}

func (m MainModel) renderHelp() string {
	switch m.state {
	case StateMenu:
		return "↑/↓: 移动  ENTER: 确认  Q: 退出"
	case StateRepoSelect:
		return "↑/↓: 移动  SPACE: 选择仓库  A: 全选/反选  D: 开始扫描  ESC: 返回菜单  Q: 退出"
	case StateScanning:
		return "正在处理，请稍候...  Q: 强制退出"
	case StateBranchSelect:
		if m.branchModel.mode == ModeConfirm {
			return "D: 确认执行删除  ESC: 取消并返回  Q: 退出"
		}
		return "↑/↓: 移动  SPACE: 选中分支  A: 全选/反选  D: 执行删除  ESC: 返回菜单  Q: 退出"
	default:
		return "Q: 退出"
	}
}

func StartMainTUI(cfg *Config, opts TUIOptions) (Result, error) {
	m := NewMainModel(cfg, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	fm := finalModel.(MainModel)
	return fm.branchModel.result, nil
}

// 兼容旧的 View 方法
func (m BranchModel) View() string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorTitle)).
		MarginBottom(1)
	s.WriteString(titleStyle.Render("=== 可删除的分支（已合并到 master）==="))
	s.WriteString("\n\n")

	if len(m.candidates) == 0 {
		s.WriteString(styleMutedText.Render("未找到符合条件的分支。"))
		return s.String()
	}

	for i, candidate := range m.candidates {
		cursor := "  "
		if m.cursor == i && m.mode == ModeNormal {
			cursor = "> "
		}

		checked := " "
		if m.selected[i] {
			checked = "✓"
		}

		line := fmt.Sprintf("%s[%s] %s: %s", cursor, checked, candidate.RepoName, candidate.BranchName)

		if m.cursor == i && m.mode == ModeNormal {
			lineStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTitle)).
				Bold(true)
			line = lineStyle.Render(line)
		}

		s.WriteString(line)
		s.WriteString("\n")
	}

	s.WriteString("\n")

	switch m.mode {
	case ModeNormal:
		selectedCount := 0
		for _, selected := range m.selected {
			if selected {
				selectedCount++
			}
		}
		if selectedCount > 0 {
			countStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Bold(true)
			s.WriteString(countStyle.Render(fmt.Sprintf("已选择 %d 个分支", selectedCount)))
			s.WriteString("\n")
		}

		if m.deleteDone {
			doneStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorSuccess)).
				Bold(true)
			s.WriteString(doneStyle.Render(fmt.Sprintf("\n删除完成: 成功 %d, 失败 %d",
				len(m.result.Success), len(m.result.Failed))))
			s.WriteString("\n")
		}

	case ModeConfirm:
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarn)).
			Bold(true)
		s.WriteString(confirmStyle.Render("确认删除选中的分支？"))
		s.WriteString("\n")

	case ModeDeleting:
		progressStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true)
		barWidth := 40
		progress := float64(m.deleting) / float64(m.deleteTotal)
		filled := int(progress * float64(barWidth))

		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		s.WriteString(progressStyle.Render(fmt.Sprintf("删除进度: [%s] %d/%d", bar, m.deleting, m.deleteTotal)))
		s.WriteString("\n")
	}

	return s.String()
}
