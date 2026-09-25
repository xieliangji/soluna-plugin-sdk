> 已接受的完整目标设计。当前实现范围以 [special-protocol.md](special-protocol.md) 为准。原评审资料保存在 Soluna 主仓库。

# 专项测试插件完整协议：用户评审稿

状态：2026-09-25 用户已接受设计，并明确首个稳定协议发布前保持 V1。当前迁移实现使用 `special/1.0` 加精确草案 revision；本设计中的后续目标不代表已经全部实现。当前行为仍见[现行协议](special-protocol.md)。两份独立设计与新伙伴评审见Soluna 主仓库历史评审记录。本稿是设计评审入口；用户接受后，公共规范、Schema、接口、夹具与开发指引统一进入独立 SDK，主仓保留宿主实现和迁移说明。

## 1. 建议批准的边界

1. **专项拥有业务，Soluna 拥有执行环境。** 插件自行编排轮次、分支、重试、判定、指标和私有报告；宿主管理手机、Appium、日志会话、进程、执行资源和全程证据。
2. **公开契约只在独立 SDK 维护。** 关键字输入／输出、runtime 代理、证据模型、报告接口及测试套件契约均归 SDK；插件仅依赖固定公开版本，不导入主仓。
3. **关键字使用类型化接口，仍由现有引擎执行。** 包括默认值、等待和 settleMs、直接输出、平台限制及失败含义，不能在 SDK 重实现一套动作。
4. **证据是必交付数据，报告可选。** 插件持续补业务事实，宿主记录实际执行；分析直接消费冻结快照，无需报告或插件进程存活。
5. **首版不自动恢复崩溃业务。** 可查询已记录操作状态，不能因没收到结果就重新点击，也不承诺跨崩溃 exactly-once。
6. **官方 stub 使用真实宿主协议核心。** 只替换设备／外部事实执行器和时钟，不维护另一套宽松的模拟协议。

OCR 和日志断言继续使用各自 UI 能力契约，不增加专项生命周期。FFmpeg 仍是宿主基础设施。本轮不接模型、不迁移真实业务代码，也不改变设备仅通过 USB 访问的约定。

## 2. SDK、宿主和生成工程

| 所属 | 内容 |
| --- | --- |
| 独立 SDK | 公共版本化 Schema／类型、双向通信、关键字客户端、runtime 与证据代理、纯校验器、官方测试套件结构、契约夹具、唯一开发指南 |
| Soluna | 能力注册与协商、编译资产绑定、真实 Engine 适配、状态机、设备所有权、持久账本、证据服务、结果终结、官方测试 runner |
| 插件项目 | 元数据、私有配置／结果 Schema、业务 Provider、可选 Reporter、领域规则及测试样本 |

拟议 SDK 包分为 `special`、`keywords/v1`、`runtime/v1`、`evidence/v1`、`plugintest`，实际公共导入根沿 SDK 统一规范。SDK module、wire 协议、能力、私有配置／结果版本独立。

`soluna scaffold special-plugin` 仅生成固定 SDK 依赖、入口、Provider 待实现方法、私有 Schema 占位、构建脚本、README。README 指向固定 SDK 版本指引，不生成 AGENTS、协议副本或蓝牙示例实现。测试套件通过单独的 `soluna plugin test-init special` 创建；空业务返回 `ErrNotImplemented`，不能通过正式集成验收。

## 3. 身份、清单与准入

- `executionId`：宿主在解析配置／加载插件前创建；启动失败也有证据。进入正式运行后关联 `runId`。
- `instanceId`：每个进程唯一；`requestId`：一次通信请求；`operationId`：一次逻辑宿主操作；这些身份不能混用。
- `unitId`：业务单元／轮次；`attemptId`：业务重试；`eventId`、`resourceId`：不可变事实和资源。
- 公共身份由 SDK 生成／校验，使用不含路径分隔符的 ASCII 标识，长度不超过 128 字节；用户可读名称另存。身份在 execution 内唯一，资源文件名由宿主映射。

清单包含插件／专项身份、版本、二进制摘要、支持协议／平台、配置／编译数据／结果 Schema 的版本及摘要、结果语义指南、可选 Reporter 描述、能力需求上界。Schema／指南必须随包可读取，不依赖开发者源码路径。

顺序：建立证据入口 → 校验清单和二进制 → hello／describe → Health → Compile → 解析角色及资产 → 协商本次能力与预算 → 冻结输入 → 准备设备 → 绑定执行进程 → Prepare → Run。

Compile 使用无设备查询进程，输入是冻结 Profile，不依赖执行进程内存。返回私有编译数据、角色引用、设备／存储资产引用、实际能力需求、覆盖与证据计划、最坏预算估算。宿主解析资产后冻结摘要；执行进程确认摘要，不能重新选择另一份配置。

能力协商使用双方声明的准确版本交集，选择结果包含 id、version、契约摘要、模式、平台、限制及依赖。2.0 首版只声明准确版本列表，不引入含糊的版本范围推断。宿主目录由已注册实现导出，SDK 存在类型不表示宿主已有实现。必需能力不满足在设备操作前失败；设备连接后才能判断的条件在 readiness 阶段失败，并完整记录。可选能力缺失只能走已声明的替代分支，不能暗中少测。

## 4. Provider、Reporter 与生命周期

拟议公共接口：

```go
type Provider interface {
    Describe() Descriptor
    Health(context.Context, HealthRequest) (HealthResult, error)
    Template(context.Context, TemplateRequest) (ProfileDocument, error)
    Compile(context.Context, CompileRequest, DocumentWriter) (Compiled, error)
    Prepare(context.Context, Session, Runtime) (Preparation, error)
    Run(context.Context, Session, Runtime) (DomainResult, error)
    Cleanup(context.Context, CleanupRequest, Runtime) (CleanupResult, error)
}
type Reporter interface {
    PrepareReport(context.Context, FrozenInput, ReportIO) (ReportPlan, error)
    RenderReport(context.Context, RenderInput, ReportIO) (ReportOutput, error)
}
```

Reporter 单独可选，声明支持就必须实现并通过验收。私有报告不是执行／分析的前提。

| 状态／触发 | 允许的下一步与约束 |
| --- | --- |
| new → negotiated → bound | 描述、健康、输入／能力确认；尚无业务设备调用 |
| bound → preparing | 持久化 prepareAttempted，再进入插件 Prepare |
| Prepare 成功 → prepared | 允许一次 Run |
| Prepare 失败／取消 → draining | 禁止 Run；保留部分准备和已登记义务 |
| running → returned／draining | 返回业务结果或遇到取消／超时／故障；停止新增业务 |
| draining → cleaning | 仅当插件业务方法及宿主实际执行均已收敛后，授予新的 cleanup 代际 |
| cleaning → finalizing | 记录业务清理与宿主兜底结果；独立预算结束即终结 |
| finalizing → closed | 冻结执行快照，关闭进程；未收敛设备保持不可复用 |

Prepare 被尝试且进程仍可用才调用业务 Cleanup；前期编译失败仍执行宿主终结。Run 返回成功或失败都不能跳过清理。CleanupRequest 包含 prepareAttempted／prepareCompleted／runStarted、结束原因、未完成义务和剩余清理预算。

生命周期调用携带稳定 invocationId，重传同身份同摘要返回在途状态或原结果，不重新执行；同身份不同输入报冲突。Cleanup 失败后的业务重试用新 attemptId，但仍受同一清理总预算约束。Shutdown 归 SDK 控制接口，关闭通信和受管任务，不替代业务 Cleanup。

Health 与传输 ping 分开。Health 只检查插件依赖，禁止手机动作，返回 ready／degraded／unready 和具体依赖项。它不占业务锁；插件实现必须并发安全。启动 unready 拒绝执行；运行中健康检查策略在绑定时冻结，只对持续的必需依赖失败启动取消，不因一次健康请求超时直接宣称业务失败。

## 5. 取消、代际隔离和设备回收

每个活动阶段由宿主签发绑定 execution、instance、phase、epoch 的不透明调用令牌。SDK 自动附带，插件不能通过填写 phase=cleanup 自行获得权限。进入 draining 立即撤销旧业务代际；旧 goroutine、排队请求和晚到请求均不能进入设备执行。

取消有三个不同的确认：已接收取消、插件方法已退出、宿主操作／驱动已终止。只有最后两者满足才能开始操作手机的 Cleanup。杀死插件进程不代表 Appium／宿主在途请求终止；unknown 不代表设备空闲。

宿主执行器未收敛时，只做不争用设备的资源回收，记录业务清理未完成并保持设备隔离；不得并行启动清理动作或下一任务。租约重新放行须有原生服务／会话／执行器退出证据或显式的设备恢复流程。首版不恢复业务状态机，不重新启动插件继续未完成轮次。

取消后的晚到反馈作为宿主事实保留，但不能让已取消业务恢复推进；晚到插件事件只能通过受限的收尾通道对已发生事实补齐，不能打开新业务单元或声明新的操作已执行。封存后不得再写旧快照；新增事实进入后续快照。

## 6. 通信与错误

沿用长度前缀 JSON 双向传输。请求、响应、取消、状态查询各有明确 kind；公共字段闭合，未知字段、重复键、多个 JSON 值拒绝。扩展只在声明命名空间及 Schema 的 extension 内出现。

协商／控制、生命周期、设备执行、证据与资源传输分别调度，取消与状态查询有保留槽。生命周期每进程最多一个在途方法；每设备最多一个实际操作。同阶段不同 operationId 的并发设备请求直接返回 concurrent_call，不隐藏排队改变插件轮询时序；同 operationId 重传走状态查询／去重。

错误含稳定 code、category、phase、相关身份、message、causeRefs。区分协议错误、能力不支持、准入拒绝、实际执行失败、业务不满足、资源错误、证据缺口、清理失败和报告失败。查询可重试、原身份可重传、设备可新建业务尝试是三种不同授权，不能用一个 retryable=true 混在一起。

请求正文摘要由 SDK 固定版本规范化函数计算；明确默认值后固定字段、键排序、数值与空值规则，发布黄金向量。身份去重比较该摘要，不能比较不稳定 map 字节。时间 tick 和超出 JSON 精确整数范围的数量用十进制字符串，禁止非有限数；最终编码规则在 Schema 和生成器一起冻结。

## 7. 关键字公共契约与调用

公共执行参数、直接输出、默认值、错误／副作用语义、支持模式由 SDK capability catalog 唯一维护并生成类型化客户端与校验器。Soluna 保留 DSL 外壳、中文别名、插值、资产引用和控制流；公共参数片段从 SDK 生成或确定性映射。初次迁移逐动作验证 DSL → 公共调用 → Engine 等价，不整体搬运编译器到 SDK。

Runtime 提供 Keywords、Observe、Logs、Variables、Clock、Evidence、Resources 窄接口。关键字示意：

```go
result, err := rt.Keywords().RestartApp(ctx, operation,
    keywords.RestartAppParams{AppID: appID, SettleMs: 5000})
text, err := rt.Keywords().GetText(ctx, operation, role, params)
probe, err := rt.Keywords().ProbeElementExists(ctx, operation, role, params)
```

| 调用内容 | 约束 |
| --- | --- |
| operationId、unitId、attempt／parent 关联 | 当前活动单元已持久化；动作意图由宿主记录 |
| capability id／version／digest、mode | 必须在冻结的能力集合中；2.0 不开放任意动态关键字注册 |
| role 或专门的坐标输入 | 角色由宿主解析定位；坐标注明坐标系与来源采样，不传裸驱动 |
| typed params、timeoutMs | 公共 Schema 和实际生产准入双重验证，完整保留等待、重试、settleMs |

反馈包含 operationId、执行状态、业务／探测结果、已定义类型的 output、主次失败、intentRef／resultRef、资源引用、采样及耗时。读取文字直接返回文字；截图返回资源；保存矩形返回几何；日志启动返回 sessionRef。不能再让插件从 `case.__special_observation` 或宿主路径猜输出。

执行状态为 not_started／in_progress／completed／interrupted／unknown；completed 不等于业务 passed。Probe 明确 matched／not_matched／error，传输失败不能转不存在。已接纳操作的失败使用结构化反馈；无法取得已接纳结果时标 unknown，并可查询账本。

settle 包含在动作预算内；主动作已完成但 settle 被取消时分别保留两阶段状态，不能宣称主动作未执行。纯时间等待与后置停顿语义分别保留。if／while 等业务控制由插件编排，不作为远程执行任意 DSL 的入口。

一次逻辑操作只准入一次；底层生产引擎合法的显式重试／会话恢复仍按原契约执行，并分别记录 attempt。不能把“通信去重”宣传成物理副作用必然只发生一次。宿主先持久化接纳意图再调用引擎，完成结果持久化后回复；同 ID 同内容返回账本状态，同 ID 异内容冲突。业务重试创建新操作并关联原因，unknown 不自动重放。

## 8. 通用观察、日志、时钟及大资源

- **页面／截图／几何**：返回原始事实、格式、平台、坐标系、方向、来源摘要和采样区间。无法同帧获取时注明 best_effort 及各组件时间；不能把页面树与另次 viewport 假装为原子快照。活动 MAC、卡片选择和日志业务解析留给插件。
- **日志**：Start／Read／Stop 使用不透明 sessionRef。Read 按 cursor、条数和字节上限分页，返回序号范围、nextCursor、丢失／截断、游标调整、过滤条件、时钟来源和状态。Stop 幂等，排空后给资源引用和完整性；停止成功不代表日志完整。插件不能读取宿主临时路径。
- **变量**：只读已冻结输入及公开作用域；动作产生的关键数据直接返回。
- **时钟**：Now 返回宿主 clockId、单调 tick、UTC 与分辨率；Sleep 可取消并计入阶段预算。跨进程／设备时钟不能直接相减，校准须带证据和误差。stub 使用虚拟时钟，纯计算不逐次 RPC 取时。
- **资源**：Begin／Chunk／Commit／Status／Read／Abort。每个上传有稳定逻辑 ID，绑定本运行；分块包含 offset、大小和摘要。首版顺序上传，同块同内容重传确认，相同 offset 异内容冲突。Commit 校验全量摘要并原子入索引后确认；重复 Commit 返回同一 receipt。ACK 丢失可 Status 查询，不重新创建另一资源。未完成上传留 gap，不产生有效引用。

ResourceRef 只引用宿主已确认资源，含摘要、长度、格式、来源、采集范围、完整性和业务关联。Read 有偏移／长度与读取预算；私有报告／分析只读快照可见资源。大日志和大结果分片入资源，不能扩大帧后一次 base64 发送。

## 9. 插件业务证据与宿主事实

宿主从入口到最后交付自动记录基础设施和真实操作。插件持续提交业务意义，两类记录标明 producer，插件不能冒名写 host.operation.result。

| 必交付记录 | 内容与引用 |
| --- | --- |
| 目标／覆盖计划 | 目标来源，预期轮次／有界动态策略、必需检查及可选分支 |
| 单元开始／结束 | 根单元、父子关系、轮次／重试、目标、起止及终态；prepare／cleanup 也有对应单元 |
| 分支选择 | 条件规则、输入证据、选中路径、其它路径未执行原因 |
| 检查 | 规则版本、预期及来源、实际及来源、判定、影响业务结论与否 |
| 样本／指标 | 数值或不可用原因、单位、原始片段、样本总体、纳入／排除理由、公式／算法版本与时钟 |
| 清理义务 | 资源或业务恢复目标、所有者、登记时机、状态和证据 |
| 缺口 | 缺失／截断／中断／时钟未校准等原因、范围、影响的检查、丢失数量或明确未知 |
| 插件结束清单 | 领域结果引用、已生产最后序号、单元／检查／指标／资源及已知缺口 |

检查的预期来自输入或规则，不允许把当前观测当预期；无数值预期的规则按结构化谓词表达。零样本／不可用不能填零。未执行分支不是已覆盖；是否要求遍历全部分支由本次目标声明，不能要求每次条件运行都走互斥两支。

公共记录携带 eventId、sourceSeq、kind、单元／操作／因果引用、发生时间及来源。宿主分配 receiptSeq，但接收次序不等于跨生产者真实发生次序。多条记录可批量提交；SDK统一序列分配，关键单元开始／分支决策／重试原因须在后续副作用前获确认。

Append 只有持久化完成才 ACK；同事件同内容返回原 receipt，异内容冲突，允许查询已持久化高水位及指定 receipt。ACK 丢失不等于事实丢失；终结时按插件已生产序号与宿主持久索引对账，不只相信最后收到的 ACK。未确认事件不凭插件声称补成完整；宿主保留已收到超出插件最后 ACK 的真实记录。写入失败或业务配额耗尽停止新业务，保留清理和终态空间。

## 10. 终态、完整性与分析快照

宿主统一结果分别表达：

| 维度 | 值 |
| --- | --- |
| completion | completed／cancelled／timed_out／crashed／startup_failed／rejected |
| businessVerdict | passed／failed／inconclusive／not_run |
| contract | valid／invalid／legacy |
| coreEvidence | complete／incomplete |
| supplementalEvidence | complete／incomplete／not_requested |
| cleanup | succeeded／failed／incomplete／not_needed |
| report、analysis | succeeded／failed／not_requested；在进行中快照另有 pending |

保留 pluginClaimedVerdict、宿主诊断和主次失败，不覆盖原始结论。必需证据不足不能无条件显示 passed；已有业务失败可保留 failed 与 incomplete。补充日志不足不得一概推翻当前蓝牙的 UI 判定；哪些证据影响判定在 Compile 时冻结，插件不能执行后降级必需程度。

通用校验检查身份、版本、引用、序列、终态、覆盖和样本集合闭合。自定义算法正确性由领域黄金夹具验证；AI不能代替契约校验。平台无法采集的补充证据标原因，不伪造成功或空资源。

结果终结服务位于宿主执行层，所有退出路径均调用，独立于 Reporter：

1. 启动／编译失败也形成带原因的快照，领域结果可缺失，不要求 runId 或插件存活。
2. 执行、取证及清理终结后冻结 execution-final，包含 identity、输入／能力摘要、结果各维度、截止序号、事件／资源索引、覆盖、缺口、私有结果及其 Schema／规则说明。
3. 报告与分析各自读取该快照。报告／上传／分析过程追加宿主外层事实，后续冻结 delivery-final 并引用前快照，不改写旧摘要。
4. 分析输出独立保存 inputSnapshotId／digest、分析器／模型版本、发现、事实与推断、证据引用、缺失信息和建议。分析失败不改变业务结论。

冻结要求生产者停止或被明确标中断、接收高水位固定、所有有效引用均指向截止时已 commit 资源、索引及摘要一致。未闭合单元由宿主标 interrupted/gap，不伪造插件正常结束。磁盘彻底不可写时只能尽力保存故障并明确持久化失败，不能保证总能产出完整文件。

## 11. 私有报告

Reporter 用独立进程，使用 FrozenInput 和 ReportIO（只读 EvidenceReader 加用途受限 DocumentWriter），不提供 Runtime。PrepareReport 输出文件资源、必需引用、带 Schema／摘要的 renderState；RenderReport 使用冻结输入和最终链接映射，可包含可选的已完成分析引用，输出私有 HTML／附件。

ReportIO.Inputs 只读；ReportIO.Outputs 提供Begin／Chunk／Commit／Status／Abort，权限绑定reportInvocationId、阶段、允许的文档用途、预算和输出命名空间。Prepare只提交报告文件及renderState，Render只提交HTML及附件；不能冒名写执行证据、修改输入快照或覆盖已有资源。所有输出须commit后才能在ReportPlan／ReportOutput引用，ACK丢失按稳定文档key查询。宿主接受ReportPlan后将其输出pin至该报告终结与约定保留期，并加入后续Render进程的只读输入集合；Render不是回写原execution快照。Compile的DocumentWriter同理只允许本compileInvocationId的私有编译数据与evidencePlan，不能操作设备或提交业务事实；有效Compiled被宿主接收后，其文档才pin并进入Bind输入。失败／取消调用的未commit文档清理，已commit但未被接受引用的文档在该调用对账结束后回收。

两阶段可用不同新进程，不依赖 Suite 内存、临时路径、原手机或运行变量。缺资源明确报错，不能改用普通 UI 报告。重渲染须使用冻结二进制或明确兼容的报告版本；程序缺失仍可读通用结果与独立分析。

不形成报告等待分析、分析等待报告的循环：分析使用指定执行快照，报告可引用完成的分析；需要分析交付失败时基于后续快照发起新分析。重渲染产生新报告身份，不改写已冻结执行结果。

## 12. 预算与背压

预算作用域分 perOperation、perUnit、businessTotal、cleanupReserve、finalizationReserve。Compile 返回最坏估算，包括失败取证／重试／清理，不能只估成功平均路径。宿主在设备启动前明确接受或拒绝；接受后冻结，不静默降低配置。

| 项目 | 2.0 建议基线 |
| --- | --- |
| 帧／控制请求／资源块 | 最大帧 8 MiB；控制请求 1 MiB；资源块 256 KiB |
| 并发 | 每方向最多32个在途请求；生命周期1；设备实际操作1；控制保留槽 |
| 启动／Health | 默认10秒／5秒；状态查询独立于业务锁 |
| Template／Compile | 默认30秒，允许声明更高并在启动前冻结 |
| Prepare | 默认30秒，须支持配置至300秒；更大需求由宿主策略明确准入 |
| Run | 默认15分钟，显式配置独立于AI编写时间；宿主可配置上限，不以24小时无条件截断既有合法专项 |
| 单操作 | 不超过阶段余量；默认上限10分钟，等待／重试／settle 均计入 |
| Cleanup | 默认60秒，至少支持既有300秒；逐轮业务清理与进程最终清理分别计费 |
| 取消／关闭 | 控制立即处理；默认5秒观察插件退出，超时终止进程；宿主实际执行未收敛时不放设备 |
| 报告 | 每阶段默认60秒，独立总预算；失败不影响执行快照 |
| 事件／账本／资源总量 | 按 Compile 规模估算和宿主容量准入，分段落盘，有界内存索引；不设整场10000次固定上限 |

Audio 当前最多200轮，每轮上限9000调用；Connect 200轮，每轮5000调用，清理至300000ms。迁移前须验证相应规模和原配置时间上界，不能默认能力降级。持续日志按分片和索引保存，不为每行日志发一条独立 RPC。

业务配额不能消耗清理／终态预留。数据队列背压可取消；控制不排在大资源写入后。可选日志截断须带范围与 gap；动作／检查等核心事实不得静默丢弃。持久账本分页保存全量去重身份，不能因内存淘汰导致旧操作再次可执行。

基线不是已测性能承诺。实现第一阶段须提交预算字段 Schema、默认／最大策略及200轮规模基准；未通过容量门禁不得切换旧 Provider。

## 13. 官方集成测试与真实运行同源

`soluna plugin test-init special` 生成可编辑套件；`soluna plugin test` 启动真正插件子进程。套件契约在 SDK，runner 在 Soluna。

两层离线验收：

- **协议 stub**：生产通信、Session、准入、代际、账本、证据／资源服务、finalizer 与 report 链；替换 CapabilityExecutor、Clock 和外部样本来源。
- **引擎模拟**：同插件经过真实 Engine 与动作实现，底层使用 fake driver／日志服务，检验默认、settle、反馈投影和重试。不能只让脚本回传成功而声称验证了关键字。

插件业务套件包含 Profile、角色、能力、样本、严格顺序／显式偏序期望、反馈、虚拟时间、领域结果及证据断言。意外调用、未消费必需期望、非法反馈或未释放资源均失败。正常业务失败可以是预期测试结果，不能一概让测试失败；协议失败不能被 expected error 字符串匹配掩盖。

官方坏插件和故障注入夹具验证宿主通用协议；无需真实插件实现“崩溃按钮”。业务插件负责自有算法黄金样本。报告中列出两类验收的版本、已执行案例、未覆盖范围及全部输入摘要，不把任意用户套件通过当完整认证。

| 验收组 | 关键反例 |
| --- | --- |
| 准入 | 版本／摘要冲突、缺能力／角色、unready、无效编译产物；早期快照仍存在 |
| 生命周期 | Prepare部分失败不得Run；重复生命周期调用；取消旧代际；业务退出但宿主驱动未退出 |
| 关键字 | 直接输出、参数默认与settle、probe三态、合法引擎重试、通信重传不重新准入 |
| 证据 | ACK丢失、旧记录重传、序列冲突、缺引用、伪造宿主事实、未闭合单元、执行后降低必需性 |
| 资源／日志 | 分块重传、Commit ACK丢失、坏摘要、部分上传、cursor调整、截断、配额／磁盘错误 |
| 终结 | 动作前／中／后崩溃，宿主重启只恢复读取，不续跑；未知操作与设备隔离 |
| 报告／分析 | 无Reporter、报告失败／新进程、领域结果缺失、冻结后交付失败，仍能独立读取对应快照 |
| 领域等价 | Android/iOS页面和日志、各轮／重试分母、日志不足但UI成功、P95零样本、首失败及清理次失败 |
| 规模 | 200轮、最坏调用估算、长日志、固定内存、清理和终态预留 |

输出明确 mode=stub／engine-simulation／real-device。离线通过不能替代USB真机、Windows原生或模型分析质量验收。

## 14. 迁移顺序与完成条件

1. 用户确认本设计的所有权和边界；在 SDK 落定全部类型／Schema／错误／编码向量／状态机测试，发布候选版本。
2. 从生产动作抽取公共调用契约，形成宿主生成映射与等价夹具；新增通用几何、页面源、分页日志、资源读取和类型化直接输出。
3. 实现共享 Session、代际／账本、持续证据和报告之外的 finalizer，同步官方 stub 与引擎模拟验收。
4. 发布固定 SDK 与支持的 Soluna，更新最小生成器，用无 workspace／replace 的独立目录实际构建并执行 CLI 集成测试。
5. 按用户最新决定，先迁 Audio；Connect 本轮保留内置；私有业务观察算法随插件迁出，公共宿主只留事实接口。完成离线回归后移除旧内置 Audio Provider，再与用户进行 USB 真机验收。
6. 下一轮接独立 AI 分析及报告展示，使用相同快照／AnalysisResult，不重新设计证据来源。

special/1.0 保持准确旧语义，2.0 显式独立入口与解码器。旧插件可按旧适配运行并标 legacy，不能授予新完整性认证；不自动修改或补造旧数据。何时移除旧协议另行明确，不引入多版本安装管理。

## 15. 与当前实现的差距

目前仍有 Host.Call 原始 JSON、固定 allowlist、仅页面源特判输出、准备失败后进入 prepared、内存去重、2 MiB 内联资源、最后事件重传及 Reporter.Prepare 内写分析输入等限制。现有 stub 也未与生产所有校验共享。这些都是待实现项，本稿与同行评审不代表已修复或测试通过。

本轮仅文档与评审，保留已有未提交实现；不发布 SDK、不提交代码、不操作手机。正式契约及指南最终归 SDK，本评审包只保留决策依据。

## 16. 规范附件 A：进程模式、资产绑定与发布入口

这些附件是整合评审意见后的约束，正文概述不能覆盖附件中的具体规则。JSON 形状为拟议契约，不是可向当前1.0发送的载荷。

| 进程模式 | 允许宿主方法 | 允许插件回调 |
| --- | --- | --- |
| query | hello、describe、health、template、compile、shutdown | 只读本次显式配置资料；Compile专用DocumentWriter提交私有编译数据／evidencePlan；无设备／日志／业务事件 |
| execute，未准备 | hello、describe、health、bind、shutdown | 无业务能力 |
| execute，活动阶段 | prepare 或 run 或 cleanup；并行ping／health／cancel／status | 对应活动epoch的能力及证据；生命周期方法不得并行 |
| execute，drain | status、cancel、drain、shutdown | 原阶段事实补交／上传收尾、插件SDK向宿主发evidence.seal；不准入设备变更 |
| report | hello、describe、health、report.prepare、report.render、shutdown | 输入快照白名单资源只读；DocumentWriter仅写本reportInvocationId的报告文件／renderState；无Variables／Logs／设备能力 |

采用现有 `SOLUNA_SPECIAL_PLUGIN_PATHS` 显式清单／成品目录入口，不扫描源码。清单沿用顶层 `protocolVersion` 字段（现有值为 `special/1.0`，草案修订通过 Health revision 精确匹配），先检查严格单 JSON 与协议版本，再核对草案 revision，不兼容时明确拒绝；同specialId出现多份包（包括内置冲突）拒绝，不按路径或版本号偷偷选最高版本。V1 Manifest 包含 executable 相对包根路径、二进制SHA256、Descriptor与随包Schema／指南资源清单及摘要。相对路径解析与平台可执行校验归宿主。

Compile 输出的公共形状：

```text
Compiled {
 profileId, profileVersion, appId, platform, name,
 privateData: DocumentValue,
 assets: {
   catalogs:[{id, source:AssetRef}],
   roles:{roleName:{catalogId, elementRef, bindings:{parameter:JSONValue}}},
   device:AssetRef, artifactStore:AssetRef | absent
 },
 requirements:[CapabilityRequirement],
 budgets:BudgetRequest, evidencePlan:DocumentValue
}
AssetRef {base:project|profile, path:relative-path}
DocumentValue = {inline:JSONValue,schemaId,schemaVersion,digest}
              | {resourceRef:ResourceRef,schemaId,schemaVersion,digest}
```

path 以声明的项目根或 Profile 所在目录为基准，不以插件进程cwd或安装目录为基准；禁止通过路径逃出相应资产根。目录alias、elementRef由宿主现有resolver映射；bindings只交参数值，不交原始Locator。输入参数必须在Compile冻结；需要运行时不同目标时，先声明有类型的动态绑定能力与其校验规则，2.0首版不以任意字符串模板绕过冻结绑定。

例如 Compile 声明 catalog `common` → `{base:"project",path:"elements/common.yaml"}`，role `target` → `{catalogId:"common",elementRef:"device.target",bindings:{targetMac:"AA:BB:CC:DD:EE:FF"}}`；device → `{base:"project",path:"devices/usb.yaml"}`。这些是开发指南的样例，不生成到插件业务骨架。

宿主解析后 BindRequest 包含 execution／run／instance、profileDigest、compiledDigest、bindingsDigest、已协商能力、有效默认配置、预算、角色名到不透明句柄的映射、DocumentValue资料；BindResult逐项回显摘要并确认。角色句柄仅此run有效。失败分别为 asset.not_found／asset.invalid_binding／asset.platform_unavailable／binding.digest_mismatch，任何一种都禁止Prepare。查询产生的大编译资料通过受控文档资源交付，query仅能提交声明的编译产物，不能借此写业务证据。

DocumentValue采用二选一闭合结构，内联至多64KiB；私有领域结果、样本集、ReportPlan.renderState及大compiledData超出阈值必须资源化，控制帧不携带完整200轮结果。报告大文件也走同一资源事务。缺失领域结果写原因，不给无效引用。

发布门：先发布可实际下载的固定SDK module tag，再切生成器默认版本；special/1.0不意味着Go module必须叫v2，module是否升major按Go公共API实际兼容决定。专项、OCR、日志三类分别在 `GOWORK=off`、无本机replace、无主仓源码的目录构建并执行各自契约测试；空骨架应按预期报未实现，已实现测试插件应通过。公开版本下载与源码checkout联调两项结果分别记录。

## 17. 规范附件 B：等待、直接输出与操作摘要

```text
CallOptions {
 timeoutMs:positive-integer | absent,
 wait:{timeoutMs:nonnegative-integer | absent,
       intervalMs:positive-integer | absent} | absent,
 settleMs:nonnegative-integer | absent
}
```

公共SDK通过Optional／指针等类型保留字段存在性，null一律拒绝；是否支持wait／settle由各能力Schema决定，不把所有字段强加给所有动作。输入省略值在Bind冻结的有效默认中解析；不能运行途中读取另一份配置。`wait.timeoutMs=0`表示一次尝试、不追加轮询；`intervalMs=0`非法。`settleMs=0`关闭动作后停顿；省略使用该能力在本次绑定中的有效值。`wait.condition`不开放表达式：兼容DSL的空对象在适配时移除，非空仍拒绝。

外层timeoutMs是整个逻辑调用预算，独立于wait轮询窗口。有效截止为调用预算、阶段余量和上下文deadline中最早者；wait不得延长外层预算；轮询共享同一窗口，interval不重置窗口，settle在动作成功后执行一次并包含在外层预算。新RPC重传不重新计时。统一客户端不新增“副作用失败重做N次”字段；生产关键字既有重试／会话恢复在能力契约单独列出，每次实际attempt留痕；插件业务重试用新operation。

| 首期能力组 | 模式与必要输出 | 特别约束 |
| --- | --- | --- |
| tap、tapPosition、longPress、swipe、input、wait、restartApp、clearAppData | do；完成状态，适用时main／settle子状态 | 各动作原参数和平台规则；坐标动作使用比例、明确空间和来源采样 |
| getText、saveElementRect | observe或已注册do；text／geometry有类型输出 | 未产出规定输出是invalid_output，不当空字符串／零矩形 |
| assertElementExists | do断言、probe三态 | 轮询截止的确定不匹配不同于外层取消／设备故障 |
| assertElementAttrEquals／RegexMatch、assertSourceRegexMatch | do；检查实际、规则、判定及证据 | 预期／pattern必需；不自动给所有断言开放probe |
| screenshot、captureAppLogStart／End | do或目录明确的observe；资源／sessionRef | 旧关键字日志入口与Logs服务使用同一宿主会话管理，不创建两套会话 |
| pageSnapshot、elementGeometry、logs.read、resources.read、clock | 独立通用runtime能力 | 能力版本明确，不沿用蓝牙私有关键字 |

首期不声明其它视觉／录屏／日志断言能力已可远程调用。每个后续能力只有SDK契约、宿主注册、完整输出映射、依赖准入及两层验收都具备才进入目录。已声明能力的完整参数必须从现行Schema逐项抽取，不能只提供表中简化字段。

反馈以联合状态约束：not_started必须有准入原因且无副作用；in_progress无终结verdict；completed可为passed／failed；interrupted注明main状态及settle状态（not_started／completed／cancelled／unknown／not_applicable）；unknown保留已知子阶段、原因和证据，禁止整体passed。ProbeState只属于probe；只有completed且有效观察才可为matched或not_matched。Schema拒绝相互矛盾组合。

规范化摘要 `call-digest/v1` 包含execution、operation、原phaseEpoch、能力版本／摘要、bindingsDigest、解析默认后的参数、请求预算和关联单元；排除requestId、实际剩余时间及重传时间。首先按Schema拒绝未知字段／重复键／null／不合法数字，应用冻结默认；对象键按Unicode码点排序、数组保序、UTF-8不做Unicode归一化；字符串只转义双引号／反斜线及控制字符（统一小写 `\u00xx`）；数字转无指数十进制，去多余前导／小数尾零，负零为0，禁止非有限数，数值展开最多128位。能力中的高精度tick本来就是十进制字符串。规范化字节SHA256配合版本形成摘要；发布跨语言黄金向量，不能依赖某Go map序列化习惯。

必须验收的有效调用：Tap显式settle=0与省略后默认=0摘要相同；Probe wait=1000／interval=100实际按100ms观察；GetText返回text；RestartApp main完成后settle=5000被取消仍保留main完成；坐标系错误在执行前拒绝；Logs.Stop重传返回同一资源。无效调用包括null、interval=0、超出能力支持字段、同operation不同有效参数／epoch。

## 18. 规范附件 C：日志记录、游标与资源保留

日志公共记录至少为：

```text
LogEntry {seq, platform, source, capturedAt:{clockId,tick,utc},
 deviceTime?:{raw,clockId,parsed?,resolution?},
 level?,tag?,process?,pid?,message,raw,
 contentRef?, truncated:boolean, originalBytes?, gapRefs:[]}
ReadResult {sessionRef,sessionStatus,requestedCursor,effectiveCursor,nextCursor,
 minSeq,nextSeq,entries:[],cursorAdjusted,droppedTotal,
 gaps:[],hasMore,eof}
```

seq从0起，表示本会话通过冻结采集过滤条件后进入规范流的记录序号；不宣称覆盖设备所有日志。cursor始终是“下一条待读seq”，客户端直接使用nextCursor，不再+1。nextSeq是下一条尚未产生的序号；minSeq是最早仍可读序号。丢失／截断计数分别注明源端已知丢失和宿主保留丢失，无法得知的数量明确unknown。公共字段不暴露原生启动命令或驱动句柄。

读取不接受改变采集过滤器；需要二次业务过滤由插件处理。cursor落后minSeq时返回effectiveCursor=minSeq、cursorAdjusted和缺口；cursor超出nextSeq报log.cursor_invalid，不静默倒退。limit为正条数，maxBytes为正响应预算；运行中空页是暂时无数据，eof=false。只有会话已终结且nextCursor达到最终nextSeq才eof=true；hasMore表示该次快照内是否还有未读记录。

单条记录过大时返回有界metadata+contentRef并推进cursor；若maxBytes连metadata也容不下，返回log.read_budget_too_small和所需最小值，不空转。原始大行按字节资源保存，截断时给明确范围／原因。NDJSON采用同一个LogEntry Schema，每行一个JSON，字段和时间语义不能另造一套。Android/iOS原生日志时间无法解析或缺时钟校准时保留原文及unknown，不冒充host时间。

Stop以operation/session身份幂等。关闭采集→有界drain→冻结最终nextSeq/缺口→commit不可变日志资源后返回receipt。Stop超时返回incomplete及已确认的部分资源，不能报完整；之后的真实迟到条目保存在后继诊断资源，不改冻结日志。停止后仍能分页读取保留数据，句柄可关闭但资源不随之删除。

资源owner为execution；临时上传计入in-flight配额，Abort和终结回收未commit字节。Begin按execution+clientResourceKey幂等，重传不重复扣费；同key不同声明冲突。已commit未引用资源至少保留至该execution最终对账完成；已进入冻结快照／ReportPlan的资源随对应保留策略pin，不能因会话关闭或临时目录清理消失。内容去重可共享blob，但引用计费／引用计数必须可对账。删除独立快照是显式保留管理，不由报告删除隐式触发。

## 19. 规范附件 D：证据封口与耐久提交

### 阶段水位和最终seal

Run返回的watermark只结束业务阶段；CleanupResult另含清理阶段生产水位、义务状态及资源集合，均不直接代表全流封存。Cleanup返回后，宿主发SDK控制请求 `drain` 并授予收尾令牌。SDK先停止／等待所有注册生产任务，用独立evidence-drain预算补交已发生记录／完成资源，再由插件侧SDK向宿主调用 `evidence.seal`；收到SealAck后回复drain。drain不是新的Provider业务方法，不允许设备回调，封口无法完成时回复明确缺口或由宿主超时终结。

`ProducerSeal{producerId,lastProducedSeq,phaseWatermarks,resourceKeys,pendingEventIds,pendingUploads,knownGaps}`。seal使用收尾令牌，不授予新业务权限。宿主对照持久化索引，返回 `SealAck{lastPersistedSeq,missingRanges,pendingUploads,closed,integrity}`。有缺口仍可closed=true且incomplete，不无限等待；已seal的生产者新事件拒绝，宿主自身清理／交付流仍可在后续快照记事实。进程崩溃无seal时宿主以已持久高水位生成中断记录，不伪造插件seal。

Append批次采用**连续前缀提交**：ACK明确lastPersistedSeq和逐事件receipt，不能笼统说批次成功；前缀落盘后失败仍可Status查询。后续重传可覆盖已存在前缀，同摘要不重复计费，然后提交剩余连续部分；不能跳号。收到乱序输入报sequence_gap并给期望序号；崩溃或取消留下缺口通过宿主衍生记录说明，不为缺失seq捏造业务事件。

独立drain预算默认10秒，可声明更高并单独准入；它仅补事实／上传，不延长业务或清理设备操作。所有生产者sealed或已明确abandoned后，才能发布execution-final。无法收敛设备可以保持quarantined并终结不完整快照，不无限占用用户请求。

### durable ACK故障模型

承诺覆盖进程异常退出及操作系统崩溃后、在本地文件系统正确实现同步原语条件下的恢复；不承诺损坏磁盘或故障硬件仍保全数据。Append仅write成功不够。允许group commit以降低成本，必须在对应持久屏障完成后ACK，不能仅在Close才Sync。

提交顺序：资源临时字节写入并同步 → 原子提交不可变blob并同步目录 → 资源索引journal追加并同步 → 才ACK Commit；引用该资源的事件随后追加并同步才ACK Append。索引是权威，内存／辅助索引均可重建。操作accepted意图先同步，再进入执行器；结果同步后再返回。生命周期与phase令牌变更也需留持久记录。

快照生成只引用已确认的blob及journal水位：写临时manifest→同步文件→原子重命名→同步目录→发布snapshot receipt。摘要不含自身digest字段，以内容清单确定；快照与其依赖都能恢复才算发布。操作系统不支持所需持久模式时启动准入明确失败或显式降级，降级结果不得宣称满足durable验收。

恢复保留可读journal完整前缀，损坏尾部隔离且记gap；不把孤儿blob猜成有效引用。已同步blob但未入索引的孤儿可回收；索引已确认却缺blob是证据损坏，保留诊断而非假完整。宿主重启不续跑业务，恢复状态／证据只读查询并执行有界资源清理。已有通用Bundle／Journal在原模块上升级，共用新写入能力，不再平行新造一套证据系统；旧格式保持旧读取语义。

## 20. 规范附件 E：故障时序与预期终态

| 故障点 | 操作／证据判断 | 恢复与设备归属 |
| --- | --- | --- |
| accepted持久化前退出 | 未获得接纳保证，不宣称设备执行；新宿主只报告未终结尝试 | 无业务自动重试；清理已有宿主资源 |
| accepted后、驱动前退出 | accepted但无结果，恢复视为unknown，不能凭空猜一定没执行 | 不再次准入该ID；检查宿主执行者／设备，必要时隔离 |
| 驱动返回后、结果持久化前退出 | 效果可能发生，unknown | 先证明原执行停止，再允许声明的只读检查或幂等补偿；不自动重放 |
| 结果已持久、回复丢失 | 状态查询／同ID重传得原结果 | 不重新调用执行器 |
| cancel与新调用竞争 | 原子撤销phase后不准入；执行前再核对epoch | 已接纳操作取消／收敛，旧令牌永不切成cleanup |
| 插件退出、宿主驱动仍挂起 | pluginExited不等于quiescent | 不做设备型cleanup；有界终结、quarantined；后续回收另记事实 |
| event部分批次／ACK丢失 | 查询持久前缀，原ID补发，不重复计数 | 无新副作用；超drain预算记gap并封口 |
| Begin／Commit ACK丢失 | 稳定key查upload／receipt，返回同一对象 | 不重复扣配额，不丢已commit内容 |
| 清理尾事件迟到／上传未完成 | seal列缺口，已seal拒绝新增插件事实 | 可冻结incomplete；不替插件声称清理完成 |
| 快照后报告／上传失败 | 旧快照不变；后续快照包含交付失败 | 业务verdict不被覆盖，单独分析后续快照 |
| 业务配额耗尽 | 停新业务，保留账本及事实 | cleanupReserve／finalizationReserve可用；容量不足不得先接受 |

默认设备解除隔离方式：宿主恢复入口必须证明原worker、会话及托管进程停止，并完成新会话readiness检查；业务环境是否恢复单独检查。人工只能显式承认无法自动验证的业务环境恢复，不能靠点击确认覆盖仍活着的执行者。后续新运行使用新execution与租约代际，旧调用无法进入。

控制传输选择保持单物理双向流，采用独立reader、优先队列、保留pending／分派槽、256KiB资源块和有界写入；不承诺控制帧可以插入半帧。写入阻塞默认5秒超时，关闭连接并用带外进程监督回收；持久化worker挂起不阻塞reader接收cancel，但ACK仍等待真实提交。验收包括半帧、对端不读、journal挂起与宿主执行不响应context，不能只测试队列拥堵。

## 21. 冻结前必须交付的夹具与门禁

设计评审通过只批准边界；SDK协议冻结还必须提供以下机器可检验材料，不能用自然语言文档代替：

- Manifest／Compile／Bind／模式准入的正反JSON和预期错误码；完整角色参数／平台缺失案例。
- 首期catalog全参数及输出Schema；字段省略／0／null、wait轮询、settle中断、调用摘要黄金向量。
- LogEntry／NDJSON／cursor多页、空页、超长行、Stop ACK丢失、源时间缺失和日志不完整但UI通过样本。
- 每个持久屏障前后强制退出后的恢复断言；已ACK事实可读、未ACK尾部无伪造、资源引用闭合。
- 状态机／phase epoch／清理seal／半帧堵塞／quarantine与恢复的固定坏插件和宿主故障注入。
- 超过控制帧的大DomainResult、ReportPlan及200轮规模；超过24小时的虚拟时间合法配置不被默认策略截断。
- 协议stub、生产Engine模拟、领域黄金和USB实测分别报告；正式SDK三类骨架干净消费验收。

稳定错误码至少包括 protocol.invalid_message／unsupported_version、capability.unsupported／invalid_arguments／invalid_output、lifecycle.invalid_state、phase.revoked、operation.conflict、operation.concurrent_call、ownership.unresolved、evidence.sequence_gap／conflict／missing_reference／sealed／persistence_failed、resource.conflict／digest_mismatch／not_committed、log.cursor_invalid／read_budget_too_small、budget.exhausted。插件私有错误使用插件命名空间。每个错误对应本稿故障案例与预期状态，不只断言字符串存在。
