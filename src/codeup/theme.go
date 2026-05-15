package codeup

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
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
	styleMutedText   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	styleTitleText   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorTitle))
	styleInfoText    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorInfo))
	styleAccentText  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	styleSuccessText = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorSuccess))
	styleWarnText    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWarn))
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
