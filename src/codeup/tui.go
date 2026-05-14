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

type Model struct {
	candidates  []Candidate
	selected    map[int]bool
	cursor      int
	mode        Mode
	result      Result
	quitting    bool
	allSelected bool
	opts        TUIOptions
	deleting    int
	deleteTotal int
	deleteDone  bool
	msgChan     chan tea.Msg
}

func NewModel(candidates []Candidate, opts TUIOptions) Model {
	return Model{
		candidates:  candidates,
		selected:    make(map[int]bool),
		cursor:      0,
		mode:        ModeNormal,
		allSelected: false,
		opts:        opts,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			if m.mode != ModeDeleting {
				m.quitting = true
				return m, tea.Quit
			}

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
			}
		case key.Matches(msg, keys.Open):
			OpenConfigDir()
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

func (m Model) listen() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m Model) startDeletion() (tea.Model, tea.Cmd) {
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

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)
	s.WriteString(titleStyle.Render("=== 可删除的分支（已合并到 master）==="))
	s.WriteString("\n\n")

	for i, candidate := range m.candidates {
		cursor := " "
		if m.cursor == i && m.mode == ModeNormal {
			cursor = ">"
		}

		checked := " "
		if m.selected[i] {
			checked = "x"
		}

		line := fmt.Sprintf("%s [%s] %s: %s", cursor, checked, candidate.RepoName, candidate.BranchName)

		if m.cursor == i && m.mode == ModeNormal {
			lineStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)
			line = lineStyle.Render(line)
		}

		s.WriteString(line)
		s.WriteString("\n")
	}

	s.WriteString("\n")

	switch m.mode {
	case ModeNormal:
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		s.WriteString(helpStyle.Render("操作说明:"))
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("  ↑/K: 上移  ↓/J: 下移  SPACE: 选择/取消"))
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("  A: 全选/反选  D: 确认删除  O: 打开配置目录  Q: 退出"))
		s.WriteString("\n")

		selectedCount := 0
		for _, selected := range m.selected {
			if selected {
				selectedCount++
			}
		}
		if selectedCount > 0 {
			countStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)
			s.WriteString(countStyle.Render(fmt.Sprintf("\n已选择 %d 个分支", selectedCount)))
			s.WriteString("\n")
		}

		if m.deleteDone {
			doneStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)
			s.WriteString(doneStyle.Render(fmt.Sprintf("\n删除完成: 成功 %d, 失败 %d",
				len(m.result.Success), len(m.result.Failed))))
			s.WriteString("\n")
		}

	case ModeConfirm:
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			MarginTop(1)
		s.WriteString(confirmStyle.Render("确认删除选中的分支？"))
		s.WriteString("\n")
		s.WriteString(confirmStyle.Render("按 D 执行删除，按 ESC 取消"))
		s.WriteString("\n")

	case ModeDeleting:
		progressStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
		barWidth := 40
		progress := float64(m.deleting) / float64(m.deleteTotal)
		filled := int(progress * float64(barWidth))

		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		s.WriteString(progressStyle.Render(fmt.Sprintf("删除进度: [%s] %d/%d", bar, m.deleting, m.deleteTotal)))
		s.WriteString("\n")
	}

	reqStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	s.WriteString(reqStyle.Render(fmt.Sprintf("HTTP 请求: %d", m.opts.Client.RequestCount())))
	s.WriteString("\n")

	return s.String()
}

func StartTUI(candidates []Candidate, opts TUIOptions) (Result, error) {
	p := tea.NewProgram(NewModel(candidates, opts))

	finalModel, err := p.Run()
	if err != nil {
		return Result{}, fmt.Errorf("运行 TUI 失败: %w", err)
	}

	m := finalModel.(Model)
	return m.result, nil
}
