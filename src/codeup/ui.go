package codeup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func DisplaySummary(result Result, dryRun bool) {
	fmt.Println("\n=== 汇总 ===")
	if dryRun {
		fmt.Printf("[DryRun] 将删除 %d 个分支\n", len(result.Success))
		return
	}

	fmt.Printf("成功删除: %d\n", len(result.Success))
	fmt.Printf("删除失败: %d\n", len(result.Failed))

	if len(result.Failed) > 0 {
		fmt.Println("\n删除失败的分支:")
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
