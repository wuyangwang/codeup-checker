# Codeup Checker - 项目知识库

## 项目概述

Codeup Checker 是一个用于清理 Codeup 平台已合并分支的命令行工具。它通过 API 获取仓库分支信息，识别已合并到 master 的分支，并提供交互式 TUI 界面让用户选择删除。

## 项目结构

```
codeup-checker/
├── cmd/
│   └── codeup-checker/     # 可执行文件入口
│       └── main.go
├── src/
│   └── codeup/             # 业务逻辑包
│       ├── client.go       # Codeup API 客户端
│       ├── config.go       # 配置管理
│       ├── repository.go   # 仓库解析逻辑
│       ├── scanner.go      # 分支扫描和删除
│       ├── tui.go          # TUI 界面
│       ├── types.go        # 类型定义
│       ├── ui.go           # UI 辅助函数
│       └── util.go         # 工具函数
├── AGENTS.md               # 项目知识库（本文件）
├── README.md               # 项目文档
├── go.mod                  # Go 模块定义
└── go.sum                  # 依赖校验
```

## 技术栈

- **语言**: Go 1.21+
- **TUI 框架**: Bubble Tea (charmbracelet/bubbletea)
- **配置管理**: YAML (gopkg.in/yaml.v3)
- **API**: Codeup OpenAPI

## 核心模块

### 1. 配置管理 (src/codeup/config.go)
- `GetConfigPath()`: 获取配置文件路径
- `LoadConfig()`: 加载 YAML 配置
- `ValidateConfig()`: 验证配置有效性
- `CreateDefaultConfig()`: 创建默认配置文件

### 2. API 客户端 (src/codeup/client.go)
- `CodeupClient`: Codeup API 客户端结构
- `ListRepositories()`: 列出仓库
- `ListBranches()`: 列出分支
- `GetCompareDetail()`: 获取分支比较详情
- `DeleteBranch()`: 删除分支

### 3. 仓库解析 (src/codeup/repository.go)
- `ResolveRepositories()`: 解析仓库配置
- `findRepositoryByName()`: 按名称查找仓库

### 4. 分支扫描 (src/codeup/scanner.go)
- `ScanRepositories()`: 并发扫描仓库分支
- `listMergedBranches()`: 列出已合并分支
- `executeDeletions()`: 执行分支删除

### 5. TUI 界面 (src/codeup/tui.go)
- `Model`: TUI 数据模型
- `StartTUI()`: 启动交互式界面
- 支持键盘操作：选择、全选、删除确认

### 6. 类型定义 (src/codeup/types.go)
- `Config`: 配置结构
- `RepoConfig`: 仓库配置
- `Candidate`: 候选删除分支
- `Result`: 操作结果

## 开发指南

### 构建命令
```bash
# 构建可执行文件
go build -o codeup-checker ./cmd/codeup-checker/

# 运行测试
go test ./...

# 代码检查
go vet ./...
```

### 环境变量
- `CODEUP_ORG_ID`: 组织 ID
- `CODEUP_ACCESS_TOKEN`: 访问令牌
- `DRY_RUN`: 设置为 "true" 启用干运行模式

### 配置文件
配置文件位置：`~/.config/codeup-checker/config.yaml`

```yaml
organization_id: "your_org_id"
access_token: "your_token"
repositories:
  - name: "repo-name"
  - id: "12345678"
```

## 并发模式

项目使用 Go 并发模式处理多个仓库和分支：

1. **仓库级并发**: 使用 `sync.WaitGroup` 并发扫描多个仓库
2. **分支级并发**: 使用 goroutine 并发检查分支合并状态
3. **删除并发**: 使用 semaphore 限制并发删除数量（默认 5）
4. **上下文传播**: 所有操作支持 `context.Context` 取消

## 错误处理

- 使用 `fmt.Errorf` 包装错误，保留错误链
- API 错误包含 HTTP 状态码和错误消息
- 批量操作收集所有错误，不因单个失败而中断

## 安全考虑

- 敏感信息（token）优先使用环境变量
- 配置文件权限设置为 0644
- 支持干运行模式预览删除操作
- 保护分支（master, main, production, release）自动排除

## 扩展点

1. **添加新 API 端点**: 在 `client.go` 中添加新方法
2. **自定义保护分支**: 修改 `scanner.go` 中的 `isProtectedBranch`
3. **UI 定制**: 修改 `tui.go` 中的样式和键绑定
4. **配置扩展**: 在 `types.go` 中添加新配置字段

## 常见问题

### Q: 如何添加新的保护分支？
A: 修改 `src/codeup/scanner.go` 中的 `protectedNames` map。

### Q: 如何修改并发数？
A: 修改 `src/codeup/scanner.go` 中 `executeDeletions` 函数的 semaphore 缓冲区大小。

### Q: 配置文件在哪里？
A: 运行程序会自动创建，或手动创建在 `~/.config/codeup-checker/config.yaml`。

## 相关链接

- [Codeup OpenAPI 文档](https://help.aliyun.com/document_detail/codeup-api.html)
- [Bubble Tea 文档](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss 文档](https://github.com/charmbracelet/lipgloss)
