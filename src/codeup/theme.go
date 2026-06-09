package codeup

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	colorMuted   = "241"
	colorTitle   = "205"
	colorInfo    = "45"
	colorAccent  = "81"
	colorSuccess = "10"
	colorWarn    = "214"
	colorError   = "196"
)

var (
	styleMutedText      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	styleWhiteText      = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	styleTitleText      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorTitle))
	styleInfoText       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorInfo))
	styleAccentText     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	styleSuccessText    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorSuccess))
	styleWarnText       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWarn))
	styleKeyNormal      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Bold(true)
	styleKeyAction      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSuccess)).Bold(true)
	styleKeyDestructive = lipgloss.NewStyle().Foreground(lipgloss.Color(colorError)).Bold(true)
	styleKeyEsc         = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWarn)).Bold(true)
	styleHelpDesc       = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
)

func renderHTTPCount(count int64) string {
	return styleInfoText.Render(fmt.Sprintf("HTTP 请求: %d", count))
}

func RenderScanStart(total int) string {
	return styleInfoText.Render(fmt.Sprintf("正在扫描 %d 个仓库的已合并分支...", total))
}

func renderRepoChecking(repoName string) string {
	return styleInfoText.Render(fmt.Sprintf("正在检查仓库: %s", repoName))
}

func renderScanError(branch string, err error) string {
	return styleWarnText.Render(fmt.Sprintf("检查 %s 时出错: %v", branch, err))
}

func renderMergedBranch(branch string) string {
	return styleSuccessText.Render(fmt.Sprintf("发现已合并分支: %s", branch))
}

// formatCommitInfo 格式化提交信息，用于分支列表展示
func formatCommitInfo(author, commitTime string) string {
	relTime := RelativeTime(commitTime)
	if author != "" && relTime != "" {
		return fmt.Sprintf("(%s, %s)", author, relTime)
	}
	if author != "" {
		return fmt.Sprintf("(%s)", author)
	}
	if relTime != "" {
		return fmt.Sprintf("(%s)", relTime)
	}
	return ""
}

const (
	defaultTUIWidth  = 80
	defaultTUIHeight = 24
)

func effectiveWidth(width int) int {
	if width > 0 {
		return width
	}
	return defaultTUIWidth
}

func effectiveHeight(height int) int {
	if height > 0 {
		return height
	}
	return defaultTUIHeight
}

func plainLen(s string) int {
	return lipgloss.Width(s)
}

func truncateText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}

	limit := maxWidth - 1
	width := 0
	var out []rune
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if width+rw > limit {
			break
		}
		out = append(out, r)
		width += rw
	}
	return string(out) + "…"
}

func truncateLines(lines []string, maxWidth int) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = truncateStyledText(line, maxWidth)
	}
	return out
}

func truncateStyledText(s string, maxWidth int) string {
	return lipgloss.NewStyle().MaxWidth(maxWidth).Inline(true).Render(s)
}

func visibleRange(total, cursor, visible int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}

	start := cursor - visible + 1
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > total {
		end = total
		start = end - visible
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
