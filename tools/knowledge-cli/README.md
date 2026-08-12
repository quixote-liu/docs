# Knowledge CLI

供 AI Agent 读写本仓库知识文档的 Go CLI。仓库根目录就是默认知识库，`tools/` 为保留目录，不参与文档列表和搜索。

## 构建

需要 Go 1.20 或更高版本。在仓库根目录执行：

```powershell
go -C tools\knowledge-cli build -o bin\knowledge.exe .\cmd\knowledge
```

也可以进入工具目录开发和测试：

```powershell
cd tools\knowledge-cli
go test ./...
go vet ./...
```

## 命令

- `categories`：列出已有分类
- `list [--category CATEGORY]`：列出全部或指定分类下的文档
- `read <path>`：读取文档
- `write <path>`：创建、覆盖或追加文档
- `search <query>`：不区分大小写地搜索内容
- `--json`：输出适合 Agent 解析的 JSON

全局参数必须放在子命令之前。以下命令均在仓库根目录执行：

```powershell
# 创建文档，省略 .md 后缀也可以
.\tools\knowledge-cli\bin\knowledge.exe write --content "# Go 测试`n`n优先使用表驱动测试。" engineering/go/testing

# 多行内容推荐通过 stdin 写入
Get-Content -Raw draft.md | .\tools\knowledge-cli\bin\knowledge.exe write --stdin practices/code-review

# 查询
.\tools\knowledge-cli\bin\knowledge.exe --json list --category engineering/go
.\tools\knowledge-cli\bin\knowledge.exe --json read engineering/go/testing
.\tools\knowledge-cli\bin\knowledge.exe --json search --limit 20 "表驱动"
```

写入模式：

- `--mode create`：仅创建，文档已存在时失败（默认）
- `--mode overwrite`：创建或覆盖
- `--mode append`：创建或追加

默认知识库为 CLI 运行时的当前目录。需要操作其他知识库时，在子命令前使用 `--root <目录>`，也可以设置 `KNOWLEDGE_ROOT` 环境变量。

成功时退出码为 `0`，失败时为非零退出码。启用 `--json` 后，错误会以 `{"error":"..."}` 输出到 stderr。工具拒绝绝对路径、父目录跳转、符号链接以及 `.git/`、`tools/` 保留路径。
