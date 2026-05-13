package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfgPath, err := getConfigPath()
	if err != nil {
		fatal("获取配置路径失败: %v", err)
	}
	
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Printf("首次运行，创建默认配置文件: %s\n", cfgPath)
		if err := createDefaultConfig(cfgPath); err != nil {
			fatal("创建默认配置文件失败: %v", err)
		}
		fmt.Println("配置文件已创建，请编辑配置文件或设置环境变量：")
		fmt.Printf("  配置文件: %s\n", cfgPath)
		fmt.Println("  环境变量: CODEUP_ORG_ID, CODEUP_ACCESS_TOKEN")
		fmt.Println("\n按回车键继续...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
	
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fatal("加载配置失败: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		fatal("配置无效: %v", err)
	}

	token := getAccessToken(cfg)
	if token == "" {
		fatal("缺少访问令牌，请在配置文件或 CODEUP_ACCESS_TOKEN 环境变量中设置")
	}

	orgId := getOrganizationId(cfg)
	if orgId == "" {
		fatal("缺少组织 ID，请在配置文件或 CODEUP_ORG_ID 环境变量中设置")
	}

	client := newCodeupClient(orgId, token)
	if err := resolveRepositories(ctx, client, cfg); err != nil {
		fatal("解析仓库失败: %v", err)
	}

	fmt.Printf("正在扫描 %d 个仓库的已合并分支...\n\n", len(cfg.Repositories))
	candidates, err := scanRepositories(ctx, client, cfg)
	if err != nil {
		fatal("扫描仓库失败: %v", err)
	}
	if len(candidates) == 0 {
		fmt.Println("未找到已合并的分支。")
		return
	}

	result, err := startTUI(candidates)
	if err != nil {
		fatal("TUI 错误: %v", err)
	}

	if len(result.Success) == 0 && len(result.Failed) == 0 {
		fmt.Println("未选择要删除的分支。")
		return
	}

	displaySummary(result, os.Getenv("DRY_RUN") == "true")
}

func openConfigDir() error {
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
