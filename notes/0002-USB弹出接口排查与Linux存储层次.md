# USB 弹出接口排查与 Linux 存储层次

更新时间：2026-11-03 10:36

场景：排查文件浏览器 USB 弹出失败，随后通过设备上的 lsblk 输出解释一个 U 盘显示两个分区，以及磁盘、文件系统、挂载点与块设备命名的关系。本文整理 2026-11-02 至 2026-11-03 的会话与现场输出。

## 接口标识与运行时变量路径

文件浏览器 GET /api/files/ 的 USB 分区 id 原来返回运行时系统变量 UUID，前端把该值用于写变量接口，而写入侧需要运行时变量路径。修复复用 GetSystemVariablePathByID，只转换四个 USB 槽位的响应标识；分区状态读取仍用 UUID 经 GetSystemVariableIndex 查索引。

源码依据（本次检查的 web-gui-server 工作区）：

- internal/service/filebrowser/partition.go：mountUsbDiskPartition 读取四个槽位，RefreshUsbDisk 按 mount 状态维护 disk0 至 disk3；ReadPartitionList 把 partition.id 写入响应。
- internal/view_application.go：initVariablePathLookup 注入 GetSystemVariablePathByID，查系统变量索引后返回 variable.Path()；找不到时返回空串。
- 文件读写按分区名和文件路径定位，不依赖 USB 响应 id。因此标识转换本身不会多创建分区。
- mountDiskPartition 对已存在分区只刷新容量等字段，不刷新 id；若路径为空时先建立了分区，后续刷新可能保留空标识。这是代码边界分析，不是已复现故障。

验证边界：gofmt 与 git diff --check 曾通过；Go 定向测试受本机 pkg-config / CGO 依赖限制，未完成编译，不属于测试断言失败。临时新增的测试文件后按用户要求删除。用户之后报告弹出一个条目，另一分区也会一起弹出；未采集写接口请求及系统卸载日志，不能仅据此断言底层具体实现。

## 一个 U 盘为什么显示两个 disk

现场 lsblk 的关键部分：

```text
NAME         TYPE  SIZE MOUNTPOINT
sda          disk 57.3G
└─sda1       part 57.3G
sdb          disk 14.7G
├─sdb1       part 12.8G /mnt/disk0
└─sdb4       part  1.9G /mnt/disk1
mmcblk0      disk 29.1G
├─mmcblk0p7  part  7.8G /
└─mmcblk0p9  part 20.9G /userdata
mmcblk0boot0 disk    4M
mmcblk0boot1 disk    4M
```

进一步执行 lsblk -f /dev/sdb，确认 sdb1 和 sdb4 的 FSTYPE 都为 vfat；sdb4 卷标为 XB_PE，sdb1 卷标显示乱码。卷标可能提示用途，但不能证明其为启动盘，也不能仅凭乱码判断文件系统损坏。

**已证实：同一 sdb 下两个分区分别挂在 disk0、disk1，两个条目不是 UUID 改路径产生的重复挂载。** 用户判断 sda 是另一只已弹出的 U 盘；命令只证明它当时没有显示挂载点，不能单凭这一点证明已经安全移除。

## 从设备到目录的层次

```text
物理 U 盘 /dev/sdb
├─ 分区 /dev/sdb1 → vfat 文件系统 → 挂载点 /mnt/disk0
└─ 分区 /dev/sdb4 → vfat 文件系统 → 挂载点 /mnt/disk1
```

磁盘提供可按块访问的空间，分区表描述各分区范围，文件系统组织目录、文件及空闲空间，挂载把文件系统接到目录树。此为常见布局，不是强制顺序：也可直接在整个块设备上建文件系统，或者在分区与文件系统之间引入加密、逻辑卷等层。

挂在 / 的文件系统提供根目录树；挂在 /userdata 的另一个文件系统覆盖该目录入口，路径查找到这里时进入后者。挂载不会删除底层目录原有文件，只会在该挂载视图中遮住它们；不能因子分区未挂载而误把写入根分区的目录当成正常用户数据区。

## 命名与硬件类别

- sd 是 Linux SCSI 磁盘驱动命名体系；USB 大容量存储常经该体系呈现，不代表 SD 存储卡。sda、sdb 的字母是枚举结果，不是永久身份。
- sdb1、sdb4 的末尾数字是分区号。编号不连续不证明存在未显示的分区；不能仅凭这些编号判断分区表类型。
- mmcblk0 是 MMC 子系统块设备；mmcblk0p7 中的 p 分隔设备编号和分区号。
- SD/microSD 常用于可移除存储；eMMC 通常内置并由自身控制器管理闪存。带 boot0/boot1 的现场布局高度符合 eMMC，硬件类型仍可另行确认。
- mmcblk0boot0/boot1 是同一 eMMC 的启动硬件区域，不是另外两颗存储芯片，也不是用户区分区表里的 pN 分区；不得随意格式化或写入。
- 块设备不一定是物理磁盘：loop、device-mapper、软件 RAID、zram 等都可提供逻辑块设备。

## 排查经验

先用 lsblk 看父子关系、大小及挂载点，再用 lsblk -f 看文件系统与卷标；把系统挂载事实与运行时槽位状态对照。只有挂载记录不足以解释时，再考虑槽位重复、状态残留及前端缓存。

RefreshUsbDisk 在槽位数据缺失时直接 continue，不主动清理历史条目，因此存在残留风险；但本次两个条目均有真实挂载点，不应把该风险误作本次原因。

相关总结：[Linux块设备、分区与挂载](../summary/other/Linux块设备、分区与挂载.md)、[接口标识与运行时变量路径](../summary/other/接口标识与运行时变量路径.md)。
