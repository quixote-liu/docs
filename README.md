# 知识库

这是一个以阅读和沉淀知识为主的 Git 文档库。知识文档直接存放在仓库根目录下，使用普通目录分类，使用 Markdown 编写；不需要放进额外的 `knowledge/` 目录。

## 文档分类

目前尚未添加知识分类。可以按主题直接创建目录，例如：

```text
engineering/          工程与技术知识
practices/            AI Agent 开发实践
troubleshooting/      问题排查记录
projects/             项目相关经验
```

目录可以继续细分，例如 `engineering/go/testing.md`。建议一篇文档只描述一个主题，并以一级标题开头。

## 查阅方式

日常查看时直接从本页面进入各分类目录，GitHub 会原生渲染 Markdown。也可以使用编辑器的文件树或全文搜索浏览文档。

仓库内附带一个供 AI Agent 使用的 CLI，源码和详细说明集中在 [`tools/knowledge-cli`](tools/knowledge-cli/README.md)。工具代码不会出现在知识文档的列表及搜索结果中。

常用查询示例：

```powershell
# 在仓库根目录执行
.\tools\knowledge-cli\bin\knowledge.exe categories
.\tools\knowledge-cli\bin\knowledge.exe list
.\tools\knowledge-cli\bin\knowledge.exe read engineering/go/testing
.\tools\knowledge-cli\bin\knowledge.exe search "表驱动"

# AI Agent 优先使用 JSON 输出
.\tools\knowledge-cli\bin\knowledge.exe --json list
```

## 文档约定

- 使用 `.md` 或 `.markdown` 文件。
- 目录名表达分类，文件名简洁描述主题。
- 首个一级标题作为文档标题。
- 内容尽量说明背景、结论、适用条件和实际示例。
- CLI 的 `tools/` 目录是保留目录，不用于存放知识文档。
