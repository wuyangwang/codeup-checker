package codeup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RepoSelectorModel struct {
	repos       []RepoConfig
	selected    map[int]bool
	cursor      int
	quitting    bool
	allSelected bool
	confirmed   bool
}

func NewRepoSelectorModel(repos []RepoConfig) RepoSelectorModel {
	return RepoSelectorModel{
		repos:       repos,
		selected:    make(map[int]bool),
		allSelected: false,
	}
}

func (m RepoSelectorModel) Init() tea.Cmd {
	return nil
}

func (m RepoSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.repos)-1 {
				m.cursor++
			}

		case key.Matches(msg, keys.Space):
			m.selected[m.cursor] = !m.selected[m.cursor]

		case key.Matches(msg, keys.Select):
			if m.allSelected {
				m.selected = make(map[int]bool)
				m.allSelected = false
			} else {
				for i := range m.repos {
					m.selected[i] = true
				}
				m.allSelected = true
			}

		case key.Matches(msg, keys.Confirm):
			hasSelected := false
			for _, selected := range m.selected {
				if selected {
					hasSelected = true
					break
				}
			}
			if hasSelected {
				m.confirmed = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m RepoSelectorModel) GetSelected() []RepoConfig {
	var selectedRepos []RepoConfig
	for i, selected := range m.selected {
		if selected {
			selectedRepos = append(selectedRepos, m.repos[i])
		}
	}
	return selectedRepos
}

func (m RepoSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorTitle)).
		MarginBottom(1)
	s.WriteString(titleStyle.Render("=== 请选择要扫描的仓库 ==="))
	s.WriteString("\n\n")

	for i, repo := range m.repos {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		checked := " "
		if m.selected[i] {
			checked = "✓"
		}

		line := fmt.Sprintf("%s[%s] %s", cursor, checked, repo.DisplayName())

		if m.cursor == i {
			lineStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTitle)).
				Bold(true)
			line = lineStyle.Render(line)
		}

		s.WriteString(line)
		s.WriteString("\n")
	}

	return s.String()
}

func StartRepoSelectorTUI(repos []RepoConfig) ([]RepoConfig, error) {
	p := tea.NewProgram(NewRepoSelectorModel(repos), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("运行仓库选择 TUI 失败: %w", err)
	}

	m := finalModel.(RepoSelectorModel)
	if m.quitting {
		return nil, fmt.Errorf("用户退出")
	}

	var selectedRepos []RepoConfig
	for i, selected := range m.selected {
		if selected {
			selectedRepos = append(selectedRepos, m.repos[i])
		}
	}
	return selectedRepos, nil
}
