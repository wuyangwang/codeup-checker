package main

import (
	"fmt"
	"os"
)

func main() {
	cfgPath, err := getConfigPath()
	if err != nil {
		fatal("获取配置路径失败: %v", err)
	}
	
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Printf("首次运行，创建默认配置文件: %s\n", cfgPath)
		if err := createDefaultConfig(cfgPath); err != nil {
			fatal("创建默认配置文件失败: %v", err)
		}
		fmt.Println("请编辑配置文件后重新运行程序。")
		return
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
	if err := resolveRepositories(client, cfg); err != nil {
		fatal("解析仓库失败: %v", err)
	}

	fmt.Printf("正在扫描 %d 个仓库的已合并分支...\n\n", len(cfg.Repositories))
	candidates := scanRepositories(client, cfg)
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
