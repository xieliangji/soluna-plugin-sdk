# 外部插件协议

本文维护进程协议 `1.0`。外部业务扩展是 runner 监督的可执行程序；本地 OCR 也复用本协议，能力及独立发行见 [OCR 插件](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/docs/guides/local-ocr-plugin.md)；专项测试另用 [special/1.0](special-protocol.md)；Driver 与交付不属于本能力协议。字段以 [v1 Schema](../contracts/plugin-manifest.schema.json) 为准，调用动作见 [应用日志](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/docs/actions/app-log.md)。

## 进程与传输

| 通道 | 用途 |
| --- | --- |
| stdin | runner → 插件协议帧 |
| stdout | 插件 → runner 协议帧，禁止混入日志 |
| stderr | 有大小和速率限制的 UTF-8 运行日志 |

runner 创建并持有管道，启动后立即关闭自身的子端副本；独立读写 owner 完成后才释放父端。唯一 Wait 不得提前关闭响应管道；进程树终止后再排空后代继承的描述符。

每帧为 **4 字节大端无符号长度 + UTF-8 JSON**，默认上限 8 MiB，策略只能收紧。零长度、超限、非法编码/JSON、半帧、重复或未知 ID 都终止连接。完全未读到当前帧字节的 EOF 属于 `plugin.unavailable`；读取任何字节后的截断属于协议错误。大图片、视频和日志只传资源描述符。

客户端只有一个 writer pump 和固定有界队列。入队前冻结、编码并检查完整 envelope：本地拒绝返回 `plugin.response_invalid`，不写管道，存活进程仍可复用。绝对截止时间覆盖排队与完整写入：

- 写入前取消：丢弃该帧。
- 前台帧开始后取消：调用者保留取消/超时错误，客户端关闭 stdin 并冻结 unavailable；该调用不得重发。
- 入队后的写 I/O 错误：`plugin.unavailable`。
- 周期 health 超时：退役 ID，writer 完成当前帧并丢弃晚到响应；由连续两次失约策略判断健康，不按前台中断破坏传输。

独立 reader 持续路由协商并发范围内的乱序响应，每个 ID 只交付一次。已交付但消费方尚未校验的响应进入有界账本；完成即释放 payload。零字节 EOF 等待这些校验，以免把先到的 `plugin.response_invalid` 降级为 unavailable；半帧、非法帧和 ID 错误立即失败，不等待账本。

## Envelope 与方法

请求包含 `protocolVersion`、进程生命周期内唯一 `id`、`method`、绝对 `deadline`、`params`。响应必须原样返回 ID，并且 `result`、`error` 二选一。错误含稳定 `code`、`message`、`retryable` 及可选已校验 `details`。v1 拒绝未知顶层字段；envelope 和 result 都只能有一个 JSON 值，尾随第二个值无效。

| 方法 | 契约 |
| --- | --- |
| `handshake` | 首个请求；交换 runner 版本、实例 ID、协议、功能和限额；返回插件身份/语义版本、选定协议、能力及版本、最大并发、资源 scheme、可选健康间隔。身份和能力必须匹配，主版本不兼容立即失败 |
| `health` | 返回 ready、降级原因和可选指标；握手后首次 ready 成功才可执行 |
| `execute` | 输入协商能力、动作身份、JSON 参数、选定变量和资源；输出值、消息、变量更新、资源、诊断，整体校验后才应用 |
| `cancel` | 取消指定 ID，不延长原期限 |
| `shutdown` | 停止接单、取消调用、排空有界状态、响应并退出；宽限期后 runner 终止并回收进程树 |

诊断键、变量名、资源 ID 必须是规范 ASCII 标识符且不超过 64 字节。插件不得直接修改运行上下文。

默认并发为 1，只有握手与主机策略都允许才提高；设备动作仍由引擎串行执行。并发 1 时 health、execute、shutdown 共用可取消槽位，cancel 绕过槽位但仍经过单 writer。前台调用超时后保留槽位直到原响应收敛；异步 cancel 不能阻塞已路由原响应的校验，宽限期内无法收敛则禁用进程。周期 health 超时是例外：退役 ID、直接释放槽位、不创建 settle 任务。前台占槽时跳过周期探测，不计失约。

## 资源与信任

描述符携带运行内 ID、媒体类型、大小、已知校验和、权限及受控路径。v1 接受本机路径和 `file:` URL；Windows 盘符、UNC 及对应 file URL 先规范化再校验。拒绝其他 scheme、query/fragment、盘符相对路径、Windows 设备命名空间，以及非 Windows 主机上的 file authority。

runner 在启动前固定插件输出目录身份。返回资源必须位于该目录，禁止链接绕行，并满足类型、大小与声明校验和。在插件 entry 锁内按 wire 顺序把完整批次复制到调用专属的 runner 文件；部分失败丢弃整批，动作不得重开插件路径。

| 限制 | 语义 |
| --- | --- |
| `sizeBytes: 0` | 由 runner 探测；非零必须精确匹配 |
| 每次结果 | 最多 32 个输出、合计 1 GiB |
| 单资源 | 取调用方与 manifest 正值的较小值；manifest 缺省 64 MiB，任何调用方均不得超过 1 GiB |
| 接受后 | 统一补全实测大小和 SHA-256 |

manifest 维护身份、协议、可执行路径、参数、工作目录、环境变量名称白名单、可选 executable SHA-256、能力 kind/idempotent、读根、输出目录及资源/重启预算。路径相对 manifest 解析；进程配置与序列化 CompiledPlan 分别维护。正式运行可显式授予本次 run root 读权限。Windows 要求普通 `.exe`；Unix 还要求可执行位。仓库和脚手架在两平台统一使用 `.exe` 文件名。

外部文本先按协议 1024 字节上限验证，再裁剪为最多 512 字节预览加省略号。

## 失败与重启

意外退出使活动调用失败为 `plugin.unavailable`。一旦 execute 帧开始，同一调用绝不重启重发。后续**引擎授权且能力声明幂等**的调用才可使用有界重启预算，并重新握手建立新实例。

唯一的同次替换场景是：幂等调用等到槽位时发现旧进程已 unavailable，且自身帧从未开始；可在新进程发送一次。生命周期决策使用客户端不可变的首个终止错误；协议和结果校验错误禁用本次运行中的插件，不得被竞态返回降级为可重启故障。

shutdown 和启动中止先中断 writer，再有界终止、Wait、排空 reader。调用方超时不等于资源所有权已经释放。

## SDK、动作映射与兼容

仓库插件按类别集中在 [plugins/](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/plugins/README.md)，构建入口由 `plugins/build.mk` 管理。共用监督器与公开 SDK 保持独立职责，目录调整不改变本协议或现有插件身份。

源码位置不属于运行契约：插件可以同仓库或独立仓库开发，宿主按 manifest 接入成品。创建和版本绑定见[插件开发指南](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/docs/guides/plugin-development.md)。

公共 `pluginapi` 提供类型、`ReadFrame/WriteFrame` 和 `Server`；实现方只实现 Handshake、Health、Execute、Shutdown，服务端负责有界分发与取消。`soluna scaffold app-log-plugin` 生成独立项目及 manifest；`local-ocr-plugin` 生成使用公开 `pluginapi/ocr` 的独立 OCR 工程与打包脚本。

`customAssertAppLog.plugin` 必须对应 assertion 能力；execute 参数为 args、source、readLimit。只有完整规范资源（含字符串 id/path/contentType，顶层或受认可包装）才被快照、哈希并授予读取；不完整资源形状仍是兼容数据，不触碰文件。完整调用参数在进入可替换 executor 前计算 envelope 余量并检查帧限额。

动作先整体校验/认领结果，再按变量名顺序更新、按 wire 顺序注册资源；日志动作的重放与 Close 账本见 [应用日志](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/docs/actions/app-log.md)。账本只在本进程有效，崩溃残留服从 run root 保留/清理规则，不授权跨进程重放。

可选字段增加须协议次版本与功能协商；删字段、改语义或 framing 须新主版本；能力版本独立于传输。验收覆盖半帧、超限、尾随 JSON、乱序、超时、取消、进程退出、输入阻塞、并发 1 控制调用和退出回收。
