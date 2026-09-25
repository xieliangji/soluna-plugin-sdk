# 专项协议 V1：当前迁移草案

进程版本仍为 `special/1.0`，兼容标识为 `v1-draft-20260925`。首个稳定版本前统一保持 V1；SDK Go module 版本单独递增。宿主启动时检查 Describe、Health.ready 和 Health.revision；草案不一致必须在占用设备之前拒绝。

## SDK 接口与项目边界

公共接口、类型和传输只有 SDK 一份定义：[types.go](../pluginapi/special/types.go)、[runtime.go](../pluginapi/special/runtime.go)、[server.go](../pluginapi/special/server.go)。插件实现 Provider，可额外实现 HealthProvider、SnapshotProvider、ResourceReporter。项目不得导入 soluna-dsl 或复制协议实现。

Provider 负责 Describe、Template、Compile、Prepare、Run、Cleanup、PrepareReport、RenderReport。SDK 服务端负责状态转换、取消、进程消息和回调 epoch；业务模块负责完整配置校验、流程、结果及报告。准备失败后禁止 Run；Cleanup 即使准备失败仍允许调用。业务失败通过 Result.Status / Failure 表示，不用 RPC 成功代表业务通过。

Compiled 冻结业务配置、角色/目录、设备与存储引用、平台、关键字、单元数量与预算。Runtime 只经 Host 注入，不暴露宿主设备对象或本地文件系统。SubjectName/SubjectID、ProductModel、ApplicationName 为业务显示元数据，不授权设备操作。

## 生命周期

每次运行：Describe → Health → Compile → Prepare → Run(unit 1..N) → Cleanup → Snapshot（可选）。RunID 与 Compiled 在 Prepare 后不可更改。单元顺序严格递增；每个阶段使用新 epoch。旧阶段 Host 在结束后失效，不能把旧执行回调送进清理阶段。清理使用独立有界上下文，插件仍必须主动响应取消。

报告运行在新的进程：report.prepare → report.render，只消费冻结结果与资源。ResourceReporter 可以读取宿主登记资源，提交 report- 前缀的报告输出；宿主不得提供设备调用。报告失败保留执行结果，执行结果不能依赖 HTML。

## Host 契约

| 方法 | 含义 |
| --- | --- |
| Call | 声明过的关键字、模式、角色、JSON 参数、Wait 和预算；返回动作状态、主次失败、探测状态及时间 |
| Observe | 原始页面源、元素矩形或页面源加矩形；解析业务含义仍由插件完成 |
| Variable | 读取宿主作用域变量，插件不得直接写宿主变量 |
| Event | 连续序号、业务单元、阶段、事件类别及结构化私有数据 |
| Resource | 分块提交字节，返回资源 ID、SHA256 和大小；完成前不可供报告读取 |
| ReadResource | 按登记资源 ID、偏移与限额读取，不接受宿主路径 |

Call 的 ID 属于当前阶段。同身份同内容只返回已知结果，改变内容必须冲突；unknown 不自动重放。probe 的未匹配不是驱动失败；取消、连接丢失和隐式等待恢复失败不得降级为不存在。关键字的 Wait 与 Timeout 语义由实际宿主声明及执行器决定，不在插件中重新实现关键字。

资源推荐使用 PutResource / ReadResourceTo：每块 256 KiB，单资源最多 256 MiB；读取核对偏移、稳定回执、大小与摘要，写入核对最终回执。业务不得依赖宿主 Path；例如日志描述符中的资源先读入插件自身临时文件，再交给私有分析器。

帧为 4 字节大端长度加 JSON，最大帧 8 MiB，方法载荷最多 4 MiB。32 个待响应请求、32 个待写消息、8 个入向处理槽。取消直接投递目标上下文；无调用方期限的 SDK 请求默认 15 分钟。未知结果应保留证据缺口并终止自动重放。

## 结果与证据

Result 与私有 Schema/ResultGuide 一起定义业务判定。大结果通过不可变资源回执引用，每轮详情独立保存；主结果不能只留下 HTML 或统计而丢失底层事实。必须能从轮次定位动作、状态转移、UI 结果、日志完整性与原始资源。Host 的执行状态、领域判定、清理和报告交付状态分别保留。

宿主应独立保存从启动到结束的证据包，包含动作意图/反馈、阶段身份、插件/二进制/配置摘要、领域结果和资源索引。事实来自宿主还是插件必须可区分。未来 AI 分析直接消费这些数据，报告仅引用分析结果。

完整目标设计见 [V1 设计](special-design-v1.md)。类型化能力目录协商、通用业务证据强校验、显式 drain/seal 和断点恢复状态查询尚未全部落地，不能把本次迁移草案标成稳定完整版。

## 插件集成测试

离线夹具定义在 [plugintest](../pluginapi/plugintest/types.go) 和 [Schema](../contracts/plugin-test-suite.schema.json)。使用 Soluna 的 `plugin test --manifest ... --suite ... --output ...` 验证真实插件进程，关键字与业务事件按期望顺序核对。资源分块传输由 stub 提供，报告只能用资源；stub 不连接手机。夹具允许 run 多单元、cleanup 及 report 阶段，业务返回状态必须显式断言。

插件本身运行 `GOWORK=off go test -race ./...` 与 `go vet ./...`，再做真实设备验收。SDK 测试、stub 测试、生产引擎假设备和真机分别记录，不能互相代替。

Descriptor.Templates 声明可选模板 ID 列表；未声明时只有 default。TemplateRequest.TemplateID 选择其中一项，宿主拒绝未声明项。这样迁移保留既有配置版本模板，不把所有调用静默改成最新模板。
