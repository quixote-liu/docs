# 知识库

这是一个用 Git 管理的开发知识库。一次工作产生的记录放在 `notes/`，从笔记中提炼的可复用知识放在 `summary/`，书籍和阅读资料放在 `books/`。

详细维护规则见 [`AGENTS.md`](AGENTS.md)。

AI 整理并写入内容后会询问是否提交并推送；得到同意后，AI 自行编写 commit 信息并推送，默认分支为 `main`，默认远程为 `origin`。

## 目录

```text
notes/                 只新增的任务、问题和技术讨论笔记
summary/
  databases/           数据库
  program/             编程语言、运行机制与并发
  network/             网络
  other/               其他通用知识
books/                 PDF 等书籍资料
tools/knowledge-cli/   供 Agent 使用的知识库 CLI
```

`notes/` 的文件名概括当时解决的问题、开发任务或讨论的知识点；旧笔记保留原样，更正内容写入新笔记。`summary/` 按技术主题使用文件区分内容，不在四个分类下继续建目录。总结可持续更新，并链接来源笔记；一个任务涉及 SQLite、Go 和网络时，可以分别更新对应分类的总结。

每篇笔记简短记录当时场景和写入时间；总结只要求标明更新时间。正文按知识内容组织，可用示例、代码块、图片等方式讲清机制，避免冗长的纯文字描述。业务问题可以作为例子，但重点是可复用的结论、适用条件和验证方法。PDF 直接放在 `books/`，并在 [`books/README.md`](books/README.md) 登记日期和收录场景。

## 查阅方式

可以通过文件树、GitHub Markdown 页面或 `rg` 查阅。

仓库附带的 [Knowledge CLI](tools/knowledge-cli/README.md) 可列出、读取、搜索和写入 Markdown；它不检索 PDF 正文，也不会强制 `notes/` 只增不改的规则。

以下命令在仓库根目录运行；先按 CLI 的 README 构建可执行文件：

```powershell
.\tools\knowledge-cli\bin\knowledge.exe categories
.\tools\knowledge-cli\bin\knowledge.exe list --category notes
.\tools\knowledge-cli\bin\knowledge.exe list --category summary/program
.\tools\knowledge-cli\bin\knowledge.exe search "SQLite"
.\tools\knowledge-cli\bin\knowledge.exe --json list
```
