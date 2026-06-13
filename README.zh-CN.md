# codewhy

[English](README.md) | [简体中文](README.zh-CN.md)

`codewhy` 是一款“代码考古”命令行工具。它通过 Git 和 GitHub 追踪指定代码行的来源，帮助你回答：**这段代码为什么存在？**

```text
$ codewhy src/auth.ts:42
Code: src/auth.ts:42
Target: if refreshing { return existingRequest }
Introduced: 4f28c31 on 2024-08-12 by Alice
Commit: Prevent duplicate token refresh requests
Commit URL: https://github.com/acme/app/commit/4f28c31...
Pull Request: #418 Prevent concurrent token refresh
PR URL: https://github.com/acme/app/pull/418
Related Issue: #391 Refresh races invalidate tokens
Issue URL: https://github.com/acme/app/issues/391
Reason: Prevent concurrent token refresh. Concurrent requests could invalidate each other's tokens.
Confidence: high
```

## 安装

从 [GitHub Releases](https://github.com/0stick/CodeWhy/releases) 下载对应平台的二进制文件，或者从源码安装：

```sh
go install github.com/0stick/CodeWhy/cmd/codewhy@latest
```

也可以在项目根目录手动构建：

```sh
go build -o bin/codewhy ./cmd/codewhy
```

Windows PowerShell：

```powershell
go build -o bin\codewhy.exe .\cmd\codewhy
```

## 使用方法

在任意 Git 仓库目录中运行 `codewhy`：

```sh
codewhy src/auth.ts:42
codewhy explain "src/path with spaces/auth.ts:42"
codewhy commit 4f28c31
```

支持 Windows 路径：

```powershell
codewhy 'C:\work\app\src\auth.ts:42'
```

可用选项：

```text
--json              输出机器可读的 JSON
--no-color          禁用终端颜色
--offline           仅使用本地 Git 信息
--repo <path>       无需切换目录即可分析指定仓库
--remote <name>     指定 Git remote，默认为 origin
--github-host       指定 GitHub 或 GitHub Enterprise 主机名
--context <number>  显示目标行附近的代码
--history           追踪目标行的完整历史
--function          分析目标行所属的 Go 命名函数
--include-diff      在 JSON 中包含提交 diff，默认开启
--max-diff-size     设置 diff 最大字节数
--no-cache          禁用 GitHub API 响应缓存
--cache-ttl         设置 GitHub API 缓存有效期
--verbose           在 stderr 中显示分析过程
```

查看完整命令说明：

```sh
codewhy --help
```

```sh
codewhy --history src/auth.ts:42
codewhy --function internal/git/client.go:60
codewhy --repo ../another-project src/main.go:10
codewhy --include-diff=false src/auth.ts:42
codewhy --max-diff-size 262144 src/auth.ts:42
codewhy --github-host github.example.com src/auth.ts:42
```

### 使用示例

分析某一行代码，并显示上下各两行：

```sh
codewhy --context 2 src/auth.ts:42
```

仅使用本地 Git 信息，不访问 GitHub：

```sh
codewhy --offline src/auth.ts:42
```

输出 JSON：

```sh
codewhy --json src/auth.ts:42
```

分析指定提交：

```sh
codewhy commit 4f28c31
```

## JSON 输出

JSON 数据结构带有版本号。集合字段在没有内容时输出空数组 `[]`，而不是 `null`，便于脚本和 AI Agent 稳定调用。正式 Schema 位于 [`docs/schema/codewhy-result.schema.json`](docs/schema/codewhy-result.schema.json)。

```json
{
  "schema_version": "1",
  "target": {
    "file": "src/auth.ts",
    "line": 42,
    "source_file": "src/auth.ts",
    "source_line": 42,
    "code": "if (refreshing) return existingRequest"
  },
  "commit": {
    "sha": "4f28c31...",
    "author": "Alice",
    "date": "2024-08-12T10:30:00Z",
    "message": "Prevent duplicate token refresh requests",
    "diff": "...",
    "diff_included": true,
    "diff_truncated": false,
    "files": ["src/auth.ts"],
    "url": "https://github.com/acme/app/commit/4f28c31..."
  },
  "pull_request": {
    "number": 418,
    "title": "Prevent concurrent token refresh",
    "body": "Concurrent requests could invalidate each other's tokens.",
    "url": "https://github.com/acme/app/pull/418",
    "state": "closed"
  },
  "pull_requests": [
    {
      "number": 418,
      "title": "Prevent concurrent token refresh",
      "body": "Concurrent requests could invalidate each other's tokens.",
      "url": "https://github.com/acme/app/pull/418",
      "state": "closed",
      "kind": "pull_request",
      "merged": true,
      "base_branch": "main",
      "base_default": true
    }
  ],
  "issues": [],
  "history": [],
  "reason": "Prevent concurrent token refresh. Concurrent requests could invalidate each other's tokens.",
  "confidence": "high",
  "warnings": []
}
```

## GitHub Token

分析公共仓库时可以匿名访问 GitHub API，但匿名请求的速率限制较低。`codewhy` 会按照以下顺序读取 Token：

1. `CODEWHY_GITHUB_TOKEN`
2. `GITHUB_TOKEN`
3. 如果已安装 GitHub CLI，则执行 `gh auth token`
4. 匿名访问 GitHub API

PowerShell 设置示例：

```powershell
$env:CODEWHY_GITHUB_TOKEN = "你的 GitHub Token"
codewhy src/auth.ts:42
```

Bash 设置示例：

```sh
export CODEWHY_GITHUB_TOKEN="你的 GitHub Token"
codewhy src/auth.ts:42
```

Token 只会被放入 GitHub 请求的认证头中，绝不会被输出或写入日志。

GitHub Enterprise 用户可以传入 `--github-host github.example.com`，API 地址会自动使用 `https://github.example.com/api/v3`。

## 缓存

成功的 GitHub API 响应默认缓存在操作系统的用户缓存目录中，有效期为 15 分钟。使用 `--no-cache` 可以关闭缓存，使用 `--cache-ttl 1h` 可以调整有效期。缓存文件不会包含认证 Token。

## 工作原理

1. 检查当前目录是否位于 Git 仓库中。
2. 对目标行执行 `git blame --line-porcelain -w -M -C`。
3. 使用稳定的 Git 输出格式读取提交信息、修改文件和 diff。
4. 解析指定 remote，识别 GitHub 仓库的 owner 和 repo。
5. 查询与提交关联的 Pull Request，以及正文或提交信息中引用的 Issue。
6. 按照 PR、Issue、Commit message 的优先级生成确定性的原因摘要。

置信度含义：

- `high`：存在 PR 或 Issue 提供的明确说明。
- `medium`：只有清晰的 commit message 可以作为依据。
- `low`：没有找到明确记录的原因。

`codewhy` 不会把根据 diff 得出的猜测包装成事实。如果没有足够文档，会明确显示 `No documented reason found`。

## 隐私说明

本地源代码、diff 和 commit message 会保留在你的机器上。网络请求只包含 GitHub API 查询所必需的仓库标识、commit SHA、PR 编号和 Issue 编号。

`codewhy`：

- 不会调用任何付费 AI API。
- 不会将本地源代码发送给 GitHub。
- 不会输出或记录 GitHub Token。
- 不会在本地创建数据库。

使用 `--offline` 可以完全关闭 remote 检查和 GitHub 网络请求。

## 当前限制

- 函数分析目前支持 Go 命名函数和方法。
- 远程信息目前仅支持 GitHub；其他托管平台仍可使用本地 Git 分析。
- PR 关联依赖 GitHub 的 commit-to-PR API。
- Reason 是确定性的文本摘要，不是 AI 语义总结。
- 目标行存在未提交修改时，需要先提交或暂存这些修改。

## Roadmap

- 函数字面量和代码块级追踪
- GitLab 支持
- PR review comment 分析
- ADR 和文档搜索
- 可选的本地或远程 LLM 摘要
- VS Code 扩展
- MCP Server

## 开发与测试

要求：

- Go 1.24 或更高版本
- Git

运行测试：

```sh
go test ./...
```

执行静态检查：

```sh
go vet ./...
```

构建二进制：

```sh
go build ./cmd/codewhy
```

## 参与贡献

欢迎提交 Issue 和 Pull Request。开发环境、测试要求和提交规范请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源许可证

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。
