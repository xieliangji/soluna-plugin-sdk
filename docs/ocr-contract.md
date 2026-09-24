# OCR 插件契约

固定插件身份 `soluna.local-ocr`，传输协议 `1.0`，能力 `ocr.initialize / ocr.recognize` 版本 `1.0.0`。单并发；初始化非幂等，识别幂等。握手、生命周期与错误均由协议约束。

## 独立开发的实现契约

公共接口为 [pluginapi.Handler](../pluginapi/server.go)，消息类型见 [protocol.go](../pluginapi/protocol.go)。Handler 包括 Handshake、Health、Execute、Shutdown。生成项目的内部适配器只负责连接业务后端；它不是另一套公共协议。默认骨架的实现位置和开发步骤见 [SDK 开发指引](development.md)。

SDK 负责帧、调度和消息契约，业务后端负责真实能力、资源和结果；插件无需克隆或导入主仓库。

### Health：是否能接收请求

- `Ready=true` 表示后端能接受协议请求，不表示某次动作或断言已经成功。
- 已知依赖不可用返回 `Ready=false` 和具体 `DegradedReason`；可用 `Metrics` 返回数值指标。
- 健康探测本身异常返回 error；不把探测失败转换成 ready。Health 应短时、可取消，不执行识别或业务断言。
- 适配层透传原 context、完整健康结果和 error；不会因为 Backend 非 nil 就返回健康。预先取消的调用不进入后端。

### Execute：输入、结果与证据

| 字段 | 后端约定 |
| --- | --- |
| `Capability / CapabilityVersion` | 按本页能力表分派，拒绝不支持的名称和版本 |
| `RunID / ActionID` | 宿主提供的执行身份，不能自行改写；诊断需能关联本次调用 |
| `Arguments` | 按各能力解析 JSON 参数，错误输入明确失败 |
| `RuntimeVariables` | 本次运行变量，不得跨调用或跨运行污染 |
| `Resources` | 宿主授予的资源；保留 ID、URI、媒体类型、字节数、摘要和访问方式，按能力检查后使用 |
| `Status` | 业务完成后返回 `passed` 或 `failed`；RPC 正常返回不代表 passed |
| `FailureCode / Message / Retryable` | 稳定分类、实际失败原因及是否可重试；后端不能自动重放 |
| `Value / ProducedResources` | 小型结构化值或带身份及摘要的输出资源；不依赖 HTML 报告才能解读 |
| `Diagnostics / VariableUpdates` | 事实诊断与明确的变量更新，不能掺入未经验证的 AI 推断 |

可分类的业务／依赖失败用 ExecuteResult 表达；取消、截止时间和无法生成有效结果的异常保留 error。适配层不吞错、不改写结果字段。后端必须持续检查 context；不仅在函数入口检查一次。

### 生命周期与验证

适配层负责握手身份；通用 Server 负责帧、请求取消与单并发调度，业务后端负责自己的资源。Shutdown 在同一适配器内只调用后端一次，保留首次关闭错误；即使入口 defer 与协议关闭都触发，也不会重复释放。关闭失败不能声称回收成功。

生成项目的适配器测试只验证接线。业务开发按 [SDK 开发指引](development.md) 补齐真实输入、成功／失败、资源错误和运行中取消测试；这些测试不代表真机或原生平台验收。

### OCR 能力的输入与输出

- `ocr.initialize`：Arguments 对应 SDK `ocrprotocol.Config`，包括 `bundlePath`、`libraryPath`、`librarySha256`、`intraOpThreads`、`interOpThreads`。严格解析单个 JSON 对象；成功后复用模型，不重复初始化。成功 Value 返回 `pluginVersion / bundleId / bundleSha256 / detectorSha256 / recognizerSha256 / librarySha256` 身份。
- `ocr.recognize`：Arguments 为 `{}` 或带 `roi` 的对象；ROI 为 `x / y / width / height`。只接收一个 ID 为 `image`、只读、非空且不超过 16 MiB 的图片资源，核对受控输入目录、文件类型、字节数及 SHA256。
- 识别成功返回 ID 为 `result` 的 JSON 输出资源，内容为 `items` 与上述 `identity`。每项包含 `value`、`bounds`，可含 `polygon`、`confidence`。输出资源提供实际路径、大小、媒体类型 `application/json` 和 SHA256；沿用 Worker 的 `output/result.json` 输出位置，宿主在下一次调用前读取。
- 未初始化、模型／动态库缺失、坏图片及识别失败分别保留对应 `visual.*` 分类。只有实际成功识别且没有文字时允许 items 为空。
- OCR 的 Health 在初始化前可以 ready：表示已具备接收初始化的条件；不能要求先加载模型才允许宿主发送 initialize。初始化指定资源的可用性由该次 Execute 判定。

SDK Worker 已满足 Backend 接口，后续可通过 `adapter.New(version, &ocrworker.Handler{Version: version, Root: root, Open: factory})` 接入。需要更完整的模型健康探测时，后端组合 Worker 并实现自己的 Health，继续复用其执行和资源校验。
