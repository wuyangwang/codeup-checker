# Codeup Checker

一个用于清理 Codeup 平台已合并分支的命令行工具。

## 功能特性

- 🔍 自动扫描指定仓库的已合并分支
- 🎯 交互式 TUI 界面选择要删除的分支
- ⚡ 并发处理多个仓库和分支
- 🛡️ 自动保护重要分支（master, main, production, release）
- � dryRun 模式预览删除操作
- 🔧 支持环境变量和配置文件

## 安装

```bash
go install codeup-checker/cmd/codeup-checker@latest
```

或者从源码构建：

```bash
git clone https://github.com/your-username/codeup-checker.git
cd codeup-checker
go build -o codeup-checker ./cmd/codeup-checker/
```

## 配置

首次运行会自动创建配置文件 `~/.codeup-checker/config.yaml`：

```yaml
# 组织 ID（可选，优先使用环境变量 CODEUP_ORG_ID）
organizationId: ""

# 访问令牌（可选，优先使用环境变量 CODEUP_ACCESS_TOKEN）
accessToken: ""

# 仓库列表
repositories:
  # 方式一：使用仓库名称
  - name: "your-repo-name"
  
  # 方式二：使用仓库 ID（更稳定）
  # - id: "12345678"
  
  # 方式三：同时指定 ID 和名称
  # - id: "12345678"
  #   name: "my-repo"
```

## 使用方法

### 基本用法

```bash
# 运行程序
./codeup-checker

# 干运行模式（预览删除操作）
DRY_RUN=true ./codeup-checker
```

### TUI 操作

启动后进入交互式界面：

- `↑/k`: 上移光标
- `↓/j`: 下移光标
- `space`: 选择/取消选择分支
- `a`: 全选/反选
- `d`: 确认删除
- `o`: 打开配置目录
- `q`: 退出

### 命令行参数

```bash
# 查看帮助
./codeup-checker -h

# 指定配置文件
./codeup-checker -config /path/to/config.yaml
```

## 开发

### 项目结构

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
├── AGENTS.md               # 项目知识库
├── README.md               # 项目文档
├── go.mod                  # Go 模块定义
└── go.sum                  # 依赖校验
```

### 构建和测试

```bash
# 构建
go build -o codeup-checker ./cmd/codeup-checker/

# 运行测试
go test ./...

# 代码检查
go vet ./...

# 格式化代码
gofmt -w .
```

### 依赖管理

```bash
# 添加依赖
go get package/name

# 更新依赖
go get -u ./...

# 整理依赖
go mod tidy
```

## API 文档

### Codeup OpenAPI

文档：https://help.aliyun.com/zh/yunxiao/developer-reference/getcompare

项目使用 Codeup OpenAPI 进行仓库和分支管理：

- **列出仓库**: `GET /oapi/v1/codeup/organizations/{orgId}/repositories`
- **列出分支**: `GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoId}/branches`
- **比较分支**: `GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoId}/compares`
- **删除分支**: `DELETE /oapi/v1/codeup/organizations/{orgId}/repositories/{repoId}/branches/{branchName}`

### 认证

使用 Header `x-yunxiao-token` 进行认证。

## 并发模型

项目使用 Go 并发模式：

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

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 致谢

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI 框架
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - 样式库
- [Codeup OpenAPI](https://help.aliyun.com/document_detail/codeup-api.html) - API 文档
