package codeup

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	StateMergeSelect
	StateMerge
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
		time.Sleep(100 * time.Millisecond)
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
	width       int
	height      int
	repoModel   RepoSelectorModel
	branchModel BranchModel
	mergeModel  MergeModel
	scanLogs    []string
	opts        TUIOptions
	config      *Config
	err         error
	quitting    bool
	msgChan     chan tea.Msg
	menuCursor  int
	mergeMode   bool
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

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
			case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Space):
				if m.menuCursor == 0 {
					m.mergeMode = false
					m.repoModel = NewRepoSelectorModel(m.config.Repositories)
					m.state = StateRepoSelect
				} else {
					m.mergeMode = true
					m.repoModel = NewSingleRepoSelectorModel(m.config.Repositories, "=== 请选择要合并的仓库 ===")
					m.state = StateRepoSelect
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
		// 去除任何潜在的末尾空白，确保左对齐
		cleanMsg := strings.TrimRight(msg.Message, " \n\r\t")
		m.scanLogs = append(m.scanLogs, cleanMsg)
		if len(m.scanLogs) > 15 {
			m.scanLogs = m.scanLogs[len(m.scanLogs)-15:]
		}
		return m, m.listenScan()

	case ScanTickMsg:
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
			if len(selected) > 0 {
				if m.mergeMode {
					m.mergeModel = NewMergeModel(selected[0], m.opts)
					m.state = StateMerge
					return m, m.mergeModel.Init()
				}
				m.state = StateScanning
				return m, m.startScanning(selected)
			}
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

	case StateMerge:
		m.mergeModel, cmd = m.mergeModel.Update(msg)
		if m.mergeModel.scanTriggered {
			m.mergeModel.scanTriggered = false
			m.state = StateScanning
			selected := []RepoConfig{m.mergeModel.repo}
			return m, m.startScanning(selected)
		}
		if m.mergeModel.done {
			m.state = StateMenu
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

	branchTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorTitle)).
				MarginBottom(1)

	branchLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTitle)).
			Bold(true)

	branchCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Bold(true)

	branchDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSuccess)).
			Bold(true)

	branchConfirmStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorWarn)).
				Bold(true)

	branchProgressStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAccent)).
				Bold(true)
)

func (m MainModel) View() string {
	if m.quitting {
		return "正在退出...\n"
	}

	layout := m.layout()
	var s strings.Builder

	header := truncateText("Codeup 分支清理工具", layout.width)
	s.WriteString(headerStyle.Width(layout.width).Render(header))
	s.WriteString("\n")

	s.WriteString(m.renderContent())
	s.WriteString("\n")

	if m.state > StateRepoSelect {
		s.WriteString(footerStyle.Width(layout.width).Render(m.renderFooter()))
	} else if m.state == StateRepoSelect || m.state == StateMenu {
		s.WriteString("\n" + m.renderFooter())
	}

	return s.String()
}

type mainLayout struct {
	width      int
	bodyWidth  int
	bodyHeight int
}

func (m MainModel) layout() mainLayout {
	width := effectiveWidth(m.width)
	height := effectiveHeight(m.height)

	bodyHeight := height - 3
	if m.state >= StateRepoSelect {
		bodyHeight -= 2
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	return mainLayout{width: width, bodyWidth: width, bodyHeight: bodyHeight}
}

func (m MainModel) renderContent() string {
	layout := m.layout()
	return m.renderContentWithSize(layout.bodyWidth, layout.bodyHeight)
}

func (m MainModel) renderContentWithSize(width, height int) string {
	switch m.state {
	case StateMenu:
		return m.renderMenu(width)
	case StateRepoSelect:
		return m.renderRepoSelect(width, height)
	case StateScanning:
		return m.renderScanning(width, height)
	case StateBranchSelect:
		return m.branchModel.ViewWithSize(width, height)
	case StateMerge:
		return m.mergeModel.ViewWithSize(width)
	case StateFutureTODO:
		return styleWarnText.Render(truncateText("TODO: 合并/评审功能开发中...", width) + "\n\n" + truncateText("按 ESC 返回主菜单", width))
	default:
		return ""
	}
}

func (m MainModel) renderMenu(width int) string {
	var sb strings.Builder
	sb.WriteString(styleTitleText.Render(truncateText("=== 主菜单 ===", width)))
	sb.WriteString("\n\n")
	options := []string{"分支清理 (清理已合并的分支)", "代码合并 (合并 Production 到 Master)"}
	for i, opt := range options {
		cursor := "  "
		if m.menuCursor == i {
			cursor = "> "
		}
		line := truncateText(fmt.Sprintf("%s%d. %s", cursor, i+1, opt), width)
		if m.menuCursor == i {
			line = styleAccentText.Render(line)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func (m MainModel) renderRepoSelect(width, height int) string {
	var sb strings.Builder
	sb.WriteString(m.repoModel.ViewWithSize(width, height-2))

	selected := m.repoModel.GetSelected()
	if len(selected) > 0 {
		sb.WriteString(styleAccentText.Render(truncateText(fmt.Sprintf("\n已选择 %d 个仓库", len(selected)), width)) + "\n")
	}
	return sb.String()
}

func (m MainModel) renderScanning(width, height int) string {
	var sb strings.Builder
	sb.WriteString(styleInfoText.Render(truncateText("正在扫描仓库...", width)))
	sb.WriteString("\n\n")

	visibleLogs := height - 2
	if visibleLogs < 1 {
		visibleLogs = 1
	}
	start, end := visibleRange(len(m.scanLogs), len(m.scanLogs)-1, visibleLogs)
	for _, log := range m.scanLogs[start:end] {
		sb.WriteString(truncateText(log, width) + "\n")
	}
	return sb.String()
}

func (m MainModel) renderFooter() string {
	return m.renderFooterWithWidth(effectiveWidth(m.width))
}

func (m MainModel) renderFooterWithWidth(width int) string {
	var sb strings.Builder

	sb.WriteString(truncateStyledText(m.renderHelpForWidth(width), width))

	if m.state > StateRepoSelect {
		sb.WriteString("\n")
		sb.WriteString(truncateStyledText(renderHTTPCount(m.opts.Client.RequestCount()), width))
	}

	return sb.String()
}

func (m MainModel) renderHelp() string {
	return m.renderHelpForWidth(effectiveWidth(m.width))
}

func (m MainModel) formatHelpItem(keyText, descText string, keyStyle lipgloss.Style, narrow bool) string {
	if narrow {
		return keyStyle.Render(keyText) + " " + styleHelpDesc.Render(descText)
	}
	return keyStyle.Render(keyText) + styleHelpDesc.Render(":") + " " + styleHelpDesc.Render(descText)
}

func (m MainModel) renderHelpForWidth(width int) string {
	narrow := width > 0 && width < 60
	switch m.state {
	case StateMenu:
		itemMove := m.formatHelpItem("↑/↓", "移动", styleKeyNormal, narrow)
		itemEnter := m.formatHelpItem("ENTER/SPACE", "确认", styleKeyAction, narrow)
		itemQuit := m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)
		if narrow {
			return strings.Join([]string{itemMove, itemEnter, itemQuit}, " · ")
		}
		return strings.Join([]string{itemMove, itemEnter, itemQuit}, "  ")
	case StateRepoSelect:
		itemMove := m.formatHelpItem("↑/↓", "移动", styleKeyNormal, narrow)
		itemSpace := m.formatHelpItem("SPACE", "选择仓库", styleKeyNormal, narrow)
		itemA := m.formatHelpItem("A", "全选/反选", styleKeyNormal, narrow)
		itemD := m.formatHelpItem("D", "开始扫描", styleKeyAction, narrow)
		itemEsc := m.formatHelpItem("ESC", "返回菜单", styleKeyEsc, narrow)
		itemQuit := m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)

		if narrow {
			itemSpaceNarrow := m.formatHelpItem("SPACE", "选择", styleKeyNormal, narrow)
			itemDNarrow := m.formatHelpItem("D", "开始", styleKeyAction, narrow)
			itemEscNarrow := m.formatHelpItem("ESC", "返回", styleKeyEsc, narrow)
			return strings.Join([]string{itemMove, itemSpaceNarrow, itemDNarrow, itemEscNarrow}, " · ")
		}
		if m.mergeMode {
			return strings.Join([]string{itemMove, itemSpace, itemD, itemEsc, itemQuit}, "  ")
		}
		return strings.Join([]string{itemMove, itemSpace, itemA, itemD, itemEsc, itemQuit}, "  ")
	case StateScanning:
		itemQuit := m.formatHelpItem("Q", "强制退出", styleKeyDestructive, narrow)
		return styleHelpDesc.Render("正在处理，请稍候...  ") + itemQuit
	case StateBranchSelect:
		if m.branchModel.mode == ModeConfirm {
			itemD := m.formatHelpItem("D", "确认执行删除", styleKeyDestructive, narrow)
			itemEsc := m.formatHelpItem("ESC", "取消并返回", styleKeyEsc, narrow)
			itemQuit := m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)
			return strings.Join([]string{itemD, itemEsc, itemQuit}, "  ")
		}
		itemMove := m.formatHelpItem("↑/↓", "移动", styleKeyNormal, narrow)
		itemSpace := m.formatHelpItem("SPACE", "选中分支", styleKeyNormal, narrow)
		itemA := m.formatHelpItem("A", "全选/反选", styleKeyNormal, narrow)
		itemD := m.formatHelpItem("D", "执行删除", styleKeyAction, narrow)
		itemEsc := m.formatHelpItem("ESC", "返回菜单", styleKeyEsc, narrow)
		itemQuit := m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)

		if narrow {
			itemSpaceNarrow := m.formatHelpItem("SPACE", "选择", styleKeyNormal, narrow)
			itemDNarrow := m.formatHelpItem("D", "删除", styleKeyAction, narrow)
			itemEscNarrow := m.formatHelpItem("ESC", "返回", styleKeyEsc, narrow)
			return strings.Join([]string{itemMove, itemSpaceNarrow, itemDNarrow, itemEscNarrow}, " · ")
		}
		return strings.Join([]string{itemMove, itemSpace, itemA, itemD, itemEsc, itemQuit}, "  ")
	case StateMerge:
		var itemAction string
		if m.mergeModel.status == MergeStatusDone || m.mergeModel.status == MergeStatusAlreadyMerged {
			itemAction = m.formatHelpItem("S", "扫描分支", styleKeyAction, narrow)
		} else {
			itemAction = m.formatHelpItem("M", "合并/评审", styleKeyAction, narrow)
		}
		itemEsc := m.formatHelpItem("ESC", "返回菜单", styleKeyEsc, narrow)
		itemQuit := m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)
		return strings.Join([]string{itemAction, itemEsc, itemQuit}, "  ")
	default:
		return m.formatHelpItem("Q", "退出", styleKeyDestructive, narrow)
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
	return m.ViewWithSize(defaultTUIWidth, defaultTUIHeight)
}

func (m BranchModel) ViewWithSize(width, height int) string {
	width = effectiveWidth(width)
	if height <= 0 {
		height = defaultTUIHeight
	}

	var s strings.Builder
	s.WriteString(branchTitleStyle.Render(truncateText("=== 可删除的分支 ===", width)))
	s.WriteString("\n\n")

	if len(m.candidates) == 0 {
		s.WriteString(styleMutedText.Render(truncateText("未找到符合条件的分支。", width)))
		return s.String()
	}

	listHeight := height - 5
	if listHeight < 1 {
		listHeight = 1
	}
	start, end := visibleRange(len(m.candidates), m.cursor, listHeight)

	if start > 0 {
		s.WriteString(styleMutedText.Render(truncateText(fmt.Sprintf("↑ 还有 %d 个分支", start), width)))
		s.WriteString("\n")
	}

	for i := start; i < end; i++ {
		candidate := m.candidates[i]
		cursor := "  "
		if m.cursor == i && m.mode == ModeNormal {
			cursor = "> "
		}

		checked := " "
		if m.selected[i] {
			checked = "✓"
		}

		prefix := fmt.Sprintf("%s[%s] ", cursor, checked)
		name := fmt.Sprintf("%s: %s", candidate.RepoName, candidate.BranchName)

		// 构建提交信息后缀
		commitInfo := formatCommitInfo(candidate.CommitAuthor, candidate.CommitTime)

		availableWidth := width - plainLen(prefix)
		if commitInfo != "" {
			// 为提交信息预留空间
			commitInfoWidth := plainLen(commitInfo) + 2 // 2 for "  " separator
			nameWidth := availableWidth - commitInfoWidth
			if nameWidth < 10 {
				nameWidth = availableWidth
				commitInfo = "" // 宽度不足时不显示提交信息
			}
			if commitInfo != "" {
				line := prefix + truncateText(name, nameWidth) + "  " + styleWhiteText.Render(commitInfo)
				if m.cursor == i && m.mode == ModeNormal {
					line = branchLineStyle.Render(prefix+truncateText(name, nameWidth)) + "  " + styleWhiteText.Render(commitInfo)
				}
				s.WriteString(line)
				s.WriteString("\n")
				continue
			}
		}

		line := prefix + truncateText(name, availableWidth)
		if m.cursor == i && m.mode == ModeNormal {
			line = branchLineStyle.Render(line)
		}

		s.WriteString(line)
		s.WriteString("\n")
	}

	if end < len(m.candidates) {
		s.WriteString(styleMutedText.Render(truncateText(fmt.Sprintf("↓ 还有 %d 个分支", len(m.candidates)-end), width)))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(m.renderStatus(width))

	return s.String()
}

func (m BranchModel) renderStatus(width int) string {
	switch m.mode {
	case ModeNormal:
		var s strings.Builder
		selectedCount := 0
		for _, selected := range m.selected {
			if selected {
				selectedCount++
			}
		}
		if selectedCount > 0 {
			s.WriteString(branchCountStyle.Render(truncateText(fmt.Sprintf("已选择 %d 个分支", selectedCount), width)))
			s.WriteString("\n")
		}

		if m.deleteDone {
			s.WriteString(branchDoneStyle.Render(truncateText(fmt.Sprintf("删除完成: 成功 %d, 失败 %d",
				len(m.result.Success), len(m.result.Failed)), width)))
			s.WriteString("\n")
		}
		return s.String()

	case ModeConfirm:
		return branchConfirmStyle.Render(truncateText("确认删除选中的分支？", width)) + "\n"

	case ModeDeleting:
		return m.renderDeletionProgress(width)
	}
	return ""
}

func (m BranchModel) renderDeletionProgress(width int) string {
	if m.deleteTotal <= 0 {
		return branchProgressStyle.Render(truncateText("删除进度: 准备中...", width)) + "\n"
	}

	prefix := "删除进度: ["
	suffix := fmt.Sprintf("] %d/%d", m.deleting, m.deleteTotal)
	barWidth := width - plainLen(prefix) - plainLen(suffix)
	if barWidth < 1 {
		return branchProgressStyle.Render(truncateText(fmt.Sprintf("删除进度: %d/%d", m.deleting, m.deleteTotal), width)) + "\n"
	}
	if barWidth > 40 {
		barWidth = 40
	}

	progress := float64(m.deleting) / float64(m.deleteTotal)
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return branchProgressStyle.Render(prefix+bar+suffix) + "\n"
}
