package codeup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func DisplaySummary(result Result) {
	if len(result.Failed) > 0 {
		fmt.Println("\n=== 删除失败汇总 ===")
		for _, failure := range result.Failed {
			fmt.Printf("  - %s/%s: %v\n", failure.Candidate.RepoName, failure.Candidate.BranchName, failure.Error)
		}
	}
}

func Fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}

func OpenConfigDir() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("获取配置目录失败: %w", err)
	}

	appDir := filepath.Join(configDir, "codeup-checker")

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", appDir).Start()
	case "linux":
		return exec.Command("xdg-open", appDir).Start()
	case "windows":
		return exec.Command("explorer", appDir).Start()
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}
