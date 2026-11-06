# Linux 触摸事件溢出导致系统变量推送停滞

更新时间：2026-11-06 02:16

场景：排查 FPAD710 上 south-system 进程存活但系统变量不再推送、主线程持续占满一个 CPU 核的问题。设备运行 Linux 6.1 aarch64；代码基于 Release-1.20 及现场测试版本。以下结论综合线程诊断包、SIGABRT 生成的 core、代码检查和 evdev 0.12.1 的缺陷分析。

## 结论

问题不是 evdev 依赖最近升级，而是 2026-07-15 的提交 `5063f29` 将触摸事件读取改为 evdev 异步流，使项目开始经过 evdev 0.12.1 的 `SYN_DROPPED` 状态补偿路径。触摸事件队列溢出且期间绝对轴状态变化时，该版本存在可能陷入高 CPU 状态循环的缺陷。

触发条件需要同时具备：

1. 触摸输入使用 `Device::into_event_stream()`。
2. 设备事件缓冲区溢出并上报 `SYN_DROPPED`。
3. 丢失事件期间 ABS 轴状态发生变化。
4. 设备具有多个绝对轴。现场 TSC2007 触摸屏上报 ABS_X、ABS_Y 和压力轴。
5. 读取任务在承载 Actor 的 Actix Arbiter 上运行，导致事件流中的忙循环占用 Actor 执行资源。

高频触摸增加事件产生速率；运行时短时间重复注册变量会让 BroadcastServer 同步执行多个 monitor 的 `latest_info()`，其中可能涉及磁盘、网络或外部命令查询。这会延迟同一 Arbiter 上的触摸读取，使内核缓冲区更容易溢出，是放大因素而非 evdev 缺陷本身。

## 证据

- 提交 `5063f29` 修改 `src/system/linux/tokio_input.rs`：旧实现使用 libinput + `AsyncFd`，新实现通过 `Device::open(...).into_event_stream()` 读取 evdev 流。
- 当前代码以 `actix::spawn` 启动 udev 监视和设备读取任务；因此它们属于 Actix 当前执行环境，而不是独立阻塞线程。
- `BroadcastServer::Handler<EventMessage>` 在 Actor handler 内同步遍历 monitor 并调用 `latest_info()`，之后才将消息处理工作交给 spawn task。
- 线程采样中 south-system 主线程持续约 96% 至 100% 单核 CPU；其他 Actix runtime / server 线程在 `epoll_wait`。进程仍存活，CPU 时间持续增长。
- SIGABRT core 中主线程 PC 位于 south-system 可执行文件偏移 `0x503fb0`；附近是状态表读取、比较和向前循环分支。其他线程停在 libc 等待路径。发布二进制已剥离符号，所以无法从该 core 还原 Rust 函数名或源码行，也不能仅凭地址证明该循环就是 evdev 状态补偿函数。
- 综合代码路径与 evdev 0.12.1 的缺陷特征，状态补偿忙循环是最符合证据的根因解释；特定地址到具体依赖函数的符号化仍需要匹配版本的未剥离二进制/符号。

## 队列与容量

触摸事件积压指 Linux evdev 为每个打开的设备客户端缓存原始输入事件记录。一个触摸采样可能包含 ABS_X、ABS_Y、压力/触摸状态及 SYN_REPORT 等多条记录；读取暂时跟不上产生速度时，记录会积压，溢出后通过 SYN_DROPPED 告知用户态丢失事件。

不能把不同层级容量混为一谈：

| 层级 | 内容 | 容量说明 |
| --- | --- | --- |
| evdev 客户端缓冲区 | 坐标、压力、按键、同步等原始输入事件 | 标准 Linux 实现通常有固定缓冲区；现场内核可能有厂商改动，需查对应内核源码确认具体值。常见的 64 条说法不能在未核验设备源码时当作现场实测值。 |
| Tokio/Actix 任务 | 每设备一个异步读取循环 | 不是每个触摸事件创建一个 task，因此没有“触摸 task 队列容量”。 |
| 活动通知 mpsc | TouchDown/TouchUp 等简化通知 | 当前实现使用 unbounded channel，不会按固定条数限制。 |
| Actix Actor mailbox | 发往 Actor 的消息 | Actix 默认 mailbox 容量为 16，但它不是 evdev 原始输入事件缓冲区。 |

## 改进方向

1. 修补或升级 evdev 的 `SYN_DROPPED` 状态补偿路径，并用包含多个 ABS 轴的设备事件验证。
2. 将 evdev 设备读取隔离到独立 OS 线程或专用 runtime，通过有界 channel 把少量归一化触摸状态交回 Actix。避免触摸读取的异常状态占用 BroadcastServer 的调度线程。
3. 避免在 Actor handler 中同步执行慢速 monitor 查询；Actor 先快速接收消息，再由独立任务采集并回传结果。
4. 保留 BroadcastServer 调度心跳和外部恢复机制。它用于检测“进程活着但 Actor 不推进”，不能代替修复事件流和阻塞路径。
5. 验证覆盖高频触摸、SYN_DROPPED、多 ABS 轴、短时间重复注册、变量持续推送以及运行时 WebSocket 重连与重新注册。

参考：

- south-system 提交 `5063f29`（2026-07-15）及 `src/system/linux/tokio_input.rs`。
- `src/websocket/server.rs` 中 `BroadcastServer::started`、`Handler<EventMessage>` 和 `Handler<RegisterMessage>`。
- 现场诊断包 `south-system-diag-20261106_141820.tar.gz`、`south-system-cpu-diag-20261106_144840.tar.gz`、`south-system-core-20261106_151857.tar.gz`。
