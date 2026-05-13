package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"codeup-checker/src/codeup"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfgPath, err := codeup.GetConfigPath()
	if err != nil {
		codeup.Fatal("获取配置路径失败: %v", err)
	}
	
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Println("首次运行，开始配置...")
		fmt.Println()
		
		if err := codeup.CreateDefaultConfig(cfgPath); err != nil {
			codeup.Fatal("创建默认配置文件失败: %v", err)
		}
		
		reader := bufio.NewReader(os.Stdin)
		
		fmt.Println("请输入以下配置信息（直接回车跳过）：")
		fmt.Println()
		
		fmt.Print("组织 ID (CODEUP_ORG_ID): ")
		orgId, _ := reader.ReadString('\n')
		orgId = strings.TrimSpace(orgId)
		
		fmt.Print("访问令牌 (CODEUP_ACCESS_TOKEN): ")
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)
		
		fmt.Println("仓库名称（多个仓库用逗号分隔，如: repo1,repo2）：")
		fmt.Print("仓库列表: ")
		reposInput, _ := reader.ReadString('\n')
		reposInput = strings.TrimSpace(reposInput)
		
		var repos []codeup.RepoConfig
		if reposInput != "" {
			for _, name := range strings.Split(reposInput, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					repos = append(repos, codeup.RepoConfig{Name: name})
				}
			}
		}
		
		cfg := &codeup.Config{
			OrganizationId: orgId,
			AccessToken:    token,
			Repositories:   repos,
		}
		
		if err := codeup.SaveConfig(cfgPath, cfg); err != nil {
			codeup.Fatal("保存配置失败: %v", err)
		}
		
		fmt.Println()
		fmt.Println("✓ 配置已保存！")
		fmt.Printf("  配置文件: %s\n", cfgPath)
		fmt.Println("  提示: 也可以使用环境变量 CODEUP_ORG_ID, CODEUP_ACCESS_TOKEN")
		
		time.Sleep(300 * time.Millisecond)
	}
	
	cfg, err := codeup.LoadConfig(cfgPath)
	if err != nil {
		codeup.Fatal("加载配置失败: %v", err)
	}
	if err := codeup.ValidateConfig(cfg); err != nil {
		codeup.Fatal("配置无效: %v", err)
	}

	token := codeup.GetAccessToken(cfg)
	if token == "" {
		codeup.Fatal("缺少访问令牌，请在配置文件或 CODEUP_ACCESS_TOKEN 环境变量中设置")
	}

	orgId := codeup.GetOrganizationId(cfg)
	if orgId == "" {
		codeup.Fatal("缺少组织 ID，请在配置文件或 CODEUP_ORG_ID 环境变量中设置")
	}

	client := codeup.NewCodeupClient(orgId, token)
	if err := codeup.ResolveRepositories(ctx, client, cfg); err != nil {
		codeup.Fatal("解析仓库失败: %v", err)
	}

	fmt.Printf("正在扫描 %d 个仓库的已合并分支...\n\n", len(cfg.Repositories))
	candidates, err := codeup.ScanRepositories(ctx, client, cfg)
	if err != nil {
		codeup.Fatal("扫描仓库失败: %v", err)
	}
	if len(candidates) == 0 {
		fmt.Println("未找到已合并的分支。")
		return
	}

	result, err := codeup.StartTUI(candidates)
	if err != nil {
		codeup.Fatal("TUI 错误: %v", err)
	}

	if len(result.Success) == 0 && len(result.Failed) == 0 {
		fmt.Println("未选择要删除的分支。")
		return
	}

	codeup.DisplaySummary(result, os.Getenv("DRY_RUN") == "true")
}
