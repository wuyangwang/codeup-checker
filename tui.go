package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeConfirm
	ModeResult
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Space    key.Binding
	Select   key.Binding
	Quit     key.Binding
	Confirm  key.Binding
	Execute  key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "上移"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "下移"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "选择/取消"),
	),
	Select: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "全选/反选"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "退出"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "确认删除"),
	),
	Execute: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "执行删除"),
	),
}

type Model struct {
	candidates []Candidate
	selected   map[int]bool
	cursor     int
	mode       Mode
	result     Result
	quitting   bool
	allSelected bool
}

func NewModel(candidates []Candidate) Model {
	return Model{
		candidates: candidates,
		selected:   make(map[int]bool),
		cursor:     0,
		mode:       ModeNormal,
		allSelected: false,
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
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.candidates)-1 {
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
				return m.executeDeletion()
			}
		}
	}

	return m, nil
}

func (m Model) executeDeletion() (tea.Model, tea.Cmd) {
	var toDelete []Candidate
	for i, selected := range m.selected {
		if selected {
			toDelete = append(toDelete, m.candidates[i])
		}
	}

	// TODO: 集成实际的删除函数
	result := Result{}
	for _, candidate := range toDelete {
		result.Success = append(result.Success, candidate)
	}

	m.result = result
	m.mode = ModeResult

	newCandidates := make([]Candidate, 0)
	newSelected := make(map[int]bool)
	j := 0
	for i, candidate := range m.candidates {
		if !m.selected[i] {
			newCandidates = append(newCandidates, candidate)
			if m.selected[i] {
				newSelected[j] = true
			}
			j++
		}
	}

	m.candidates = newCandidates
	m.selected = newSelected
	m.cursor = 0
	m.allSelected = false

	return m, nil
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
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if m.selected[i] {
			checked = "x"
		}

		line := fmt.Sprintf("%s [%s] %s: %s", cursor, checked, candidate.RepoName, candidate.BranchName)
		
		if m.cursor == i {
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
		s.WriteString(helpStyle.Render("  ↑/k: 上移  ↓/j: 下移  space: 选择/取消"))
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("  a: 全选/反选  d: 确认删除  q: 退出"))
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

	case ModeConfirm:
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			MarginTop(1)
		s.WriteString(confirmStyle.Render("确认删除选中的分支？"))
		s.WriteString("\n")
		s.WriteString(confirmStyle.Render("按 d 执行删除，按其他键取消"))
		s.WriteString("\n")

	case ModeResult:
		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			MarginTop(1)
		s.WriteString(resultStyle.Render("删除结果:"))
		s.WriteString("\n")
		s.WriteString(resultStyle.Render(fmt.Sprintf("成功删除: %d 个分支", len(m.result.Success))))
		s.WriteString("\n")
		
		if len(m.result.Failed) > 0 {
			s.WriteString(resultStyle.Render(fmt.Sprintf("删除失败: %d 个分支", len(m.result.Failed))))
			s.WriteString("\n")
		}
		
		if len(m.candidates) == 0 {
			s.WriteString(resultStyle.Render("\n所有分支已删除完毕"))
			s.WriteString("\n")
			s.WriteString(resultStyle.Render("按 q 退出"))
			s.WriteString("\n")
		} else {
			s.WriteString(resultStyle.Render("\n按任意键返回继续选择"))
			s.WriteString("\n")
		}
	}

	return s.String()
}

func startTUI(candidates []Candidate) (Result, error) {
	p := tea.NewProgram(NewModel(candidates))
	
	finalModel, err := p.Run()
	if err != nil {
		return Result{}, fmt.Errorf("运行 TUI 失败: %w", err)
	}
	
	m := finalModel.(Model)
	return m.result, nil
}