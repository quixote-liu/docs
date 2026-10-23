# SQLite 时间主键、rowid 与默认值

更新时间：2026-10-23 16:12

持续采集的时间主键若存为 Unix 毫秒整数，SQLite 表声明为 `INTEGER PRIMARY KEY` 时，该列是 `rowid` 的别名。表按该整数键组织，通常不再有额外的主键唯一索引。项目中配置的 `ULINT` 会映射到 SQLite `INTEGER`；SQLite 实际整数范围仍是**有符号 64 位**。

| SQLite 声明 | 主键结构 | 省略主键值时 |
| --- | --- | --- |
| `ts INTEGER PRIMARY KEY` | `ts` 是 `rowid` 别名，表 B-Tree 即主键 | SQLite 自动分配 rowid |
| `ts DATETIME PRIMARY KEY` | rowid 表 + 主键自动索引 B-Tree | 使用配置的默认值，或按约束报错 |
| `ts BIGINT PRIMARY KEY` | 不构成 rowid 别名，通常有额外主键索引 | 可按普通列默认值处理 |

因此，给 `INTEGER PRIMARY KEY` 写 `DEFAULT (…毫秒表达式…)`，不能指望省略字段时执行该表达式。需要 Unix 毫秒时，调用方应显式提供 `time.Now().UnixMilli()`；同一毫秒写入多条仍可能主键冲突。这是**毫秒时间戳**与**唯一 ID**的不同语义。

`CURRENT_TIMESTAMP` 在 SQLite 中通常是秒级 UTC 字符串，不能直接满足毫秒主键需求。SQLite 内置 `strftime` 可以生成毫秒字符串，但 `DATETIME PRIMARY KEY` 仍需维护额外主键索引；与 Go 注册的 SQLite 自定义函数不同，内置函数不会在每行查询时回调 Go。具体性能差异应在实际写入、查询路径测量。

排查索引时，用 `PRAGMA table_info(表名)`、`PRAGMA index_list(表名)`、`EXPLAIN QUERY PLAN ...` 区分主键自动索引与业务额外索引。2026-10-22 模拟工程中，普通表 `tt1/tt2` 的 DATETIME 主键有 `sqlite_autoindex_*`；独立文件表 `test1.Timestamp` 为 INTEGER 主键，没有额外主键索引。

来源：[历史数据和数据集支持分段存储](../../notes/0001-历史数据和数据集支持分段存储.md)；Codex 2026-10-22 会话 `01a16136-...` 的只读 SQLite 元数据检查。该会话后半段关于新默认值的提案属于后续讨论，不等同于 2026-10-19 功能提交的最终设计。
