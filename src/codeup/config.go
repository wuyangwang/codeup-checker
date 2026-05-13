package codeup

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件: %w", err)
	}

	return &cfg, nil
}

func ValidateConfig(cfg *Config) error {
	for _, repo := range cfg.Repositories {
		if repo.ID == "" && repo.Name == "" {
			return fmt.Errorf("repositories 中存在缺少 id/name 的仓库配置")
		}
	}

	return nil
}

func GetAccessToken(cfg *Config) string {
	if token := os.Getenv("CODEUP_ACCESS_TOKEN"); token != "" {
		return token
	}
	return cfg.AccessToken
}

func GetOrganizationId(cfg *Config) string {
	if orgId := os.Getenv("CODEUP_ORG_ID"); orgId != "" {
		return orgId
	}
	return cfg.OrganizationId
}

func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取配置目录失败: %w", err)
	}
	
	appDir := filepath.Join(configDir, "codeup-checker")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	return filepath.Join(appDir, "config.yaml"), nil
}

func CreateDefaultConfig(path string) error {
	defaultConfig := `# Codeup 分支清理工具配置文件
# 建议将敏感信息设置为环境变量：
# export CODEUP_ORG_ID=your_org_id
# export CODEUP_ACCESS_TOKEN=your_token_here

# 组织 ID（可选，优先使用环境变量 CODEUP_ORG_ID）
organizationId: ""

# 访问令牌（可选，优先使用环境变量 CODEUP_ACCESS_TOKEN）
accessToken: ""

# 目标分支（默认 master，用于判断分支是否已合并）
targetBranch: "master"

# 排除的分支模式（支持通配符 *）
# 匹配的分支不会被扫描和删除
excludePatterns:
  - "feature/*"

# 仓库列表
repositories:
  - name: your-repo-name
`
	
	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("创建默认配置文件失败: %w", err)
	}
	
	return nil
}

func SaveConfig(path string, cfg *Config) error {
	tmpl := `# Codeup 分支清理工具配置文件
# 建议将敏感信息设置为环境变量：
# export CODEUP_ORG_ID=your_org_id
# export CODEUP_ACCESS_TOKEN=your_token_here

# 组织 ID（可选，优先使用环境变量 CODEUP_ORG_ID）
organizationId: %q

# 访问令牌（可选，优先使用环境变量 CODEUP_ACCESS_TOKEN）
accessToken: %q

# 目标分支（默认 master，用于判断分支是否已合并）
targetBranch: %q

# 排除的分支模式（支持通配符 *）
# 匹配的分支不会被扫描和删除
excludePatterns:
%s
# 仓库列表
repositories:
%s`

	var excludePatterns string
	if len(cfg.ExcludePatterns) == 0 {
		excludePatterns = "  []\n"
	} else {
		for _, p := range cfg.ExcludePatterns {
			excludePatterns += fmt.Sprintf("  - %q\n", p)
		}
	}

	var repositories string
	if len(cfg.Repositories) == 0 {
		repositories = "  []\n"
	} else {
		for _, repo := range cfg.Repositories {
			if repo.ID != "" && repo.Name != "" {
				repositories += fmt.Sprintf("  - id: %q\n    name: %q\n", repo.ID, repo.Name)
			} else if repo.ID != "" {
				repositories += fmt.Sprintf("  - id: %q\n", repo.ID)
			} else {
				repositories += fmt.Sprintf("  - name: %q\n", repo.Name)
			}
		}
	}

	content := fmt.Sprintf(tmpl, cfg.OrganizationId, cfg.AccessToken, cfg.GetTargetBranch(), excludePatterns, repositories)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}
