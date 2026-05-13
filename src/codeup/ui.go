package codeup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func displayCandidates(candidates []Candidate) {
	fmt.Println("\n=== 可删除的分支（已合并到 master）===")
	for i, candidate := range candidates {
		fmt.Printf("[%d] %s: %s\n", i+1, candidate.RepoName, candidate.BranchName)
	}
	fmt.Println()
}

func selectBranches(candidates []Candidate) []Candidate {
	fmt.Printf("输入要删除的索引（如 1,3,5-7），'all' 删除全部，'q' 退出: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "" {
		return nil
	}

	if input == "all" {
		return candidates
	}

	indices := parseIndices(input, len(candidates))
	var selected []Candidate
	for _, idx := range indices {
		selected = append(selected, candidates[idx-1])
	}

	return selected
}

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

func parseIndices(input string, max int) []int {
	var result []int
	seen := make(map[int]bool)

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) != 2 {
				continue
			}

			start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err1 != nil || err2 != nil {
				continue
			}

			for i := start; i <= end; i++ {
				if i >= 1 && i <= max && !seen[i] {
					result = append(result, i)
					seen[i] = true
				}
			}
			continue
		}

		idx, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		if idx >= 1 && idx <= max && !seen[idx] {
			result = append(result, idx)
			seen[idx] = true
		}
	}

	return result
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
