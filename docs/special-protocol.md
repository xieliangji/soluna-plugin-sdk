# 专项进程插件协议 special/1.0

本协议用于外部专项项目，独立于动作插件 `1.0`。公开 Go SDK 为 `pluginapi/special`（本 SDK），进程树管理复用宿主 `internal/plugin`；执行适配复用现有专项 Runner、Engine 和 Delivery。现有 Audio / Connect 蓝牙仍为内置 Provider，未在本次骨架交付中迁出。

## 创建、发现和执行

```sh
soluna scaffold special-plugin --output /path/to/my-special --module example.com/my-special --plugin-id startup-check --sdk-path /path/to/soluna-plugin-sdk
```

也可用 `--sdk-version` 绑定包含本 SDK 的固定发布版本；未发布时使用显式本地 SDK。工程有自己的 go.mod，首次执行 `go mod tidy` 后使用 `sh build.sh` 或 Windows `./build.ps1`，产出独立程序和 `dist/special-plugin.json`。当前脚本生成可运行目录，不负责正式发行安装、归档或许可证全集装配；发布前仍需按依赖补齐许可。

`SOLUNA_SPECIAL_PLUGIN_PATHS` 指向一个或多个清单文件/成品目录；目录内读取 `special-plugin.json`。分隔符采用系统路径列表规则（macOS/Linux 为 `:`，Windows 为 `;`）。宿主不扫描源码，重复专项 ID（包括与内置冲突）明确拒绝。清单绑定协议、插件身份、领域描述与二进制 SHA256；启动前再次核对摘要，握手描述必须与清单一致。

之后使用现有 `special list/init/validate/run`。Profile 继续位于 `specials/<app-id>/<special-id>/<profile-id>.yaml`。`init` 输出的 JSON 也是合法的单文档 YAML；示例中的元素、设备和存储路径必须替换。`validate` 启动短生命周期插件完成私有配置编译，再由宿主解析并校验资产；不会启动设备。`run` 才占用手机并执行业务。

专项唯一公共接口为 `special.Provider` 与 `special.Host`，请求／响应类型统一位于 `pluginapi/special/types.go`。插件直接实现 Provider，不复制或重定义接口。SDK 的 `Main` 管理参数、清单生成、实例绑定及 stdio；`Serve` 管理帧、派发、运行身份与生命周期边界。协议变化由 SDK 版本化处理。

生成项目只保留 Provider 待实现方法、插件元数据、私有配置／结果 Schema 占位和测试入口，不附带应用重启、控件检查或报告业务样例。未实现方法返回 `ErrNotImplemented`，不能当作执行或清理成功。构建可生成清单，但 init、validate、run 和报告必须等开发者实现对应业务后才能使用。私有业务 Schema 归插件，公共 RPC／清单 Schema 归 SDK。

## 消息与预算

清单和帧的规范分别见 [Manifest Schema](../contracts/special-plugin-manifest.schema.json)、[RPC Schema](../contracts/special-plugin-rpc.schema.json)。方法载荷与字段名以[SDK 类型](../pluginapi/special/types.go)为准；未知字段、多 JSON 值均拒绝。

传输复用四字节大端长度加 UTF-8 JSON 帧；最大帧 8 MiB，SDK单次请求 JSON 不超过4 MiB。消息携带 `version=special/1.0`、随机进程 `instance`、方向前缀请求 ID。生命周期载荷携带 runId 和冻结 Compiled；后续 run/cleanup 不得改动准备时的运行身份或输入。反向请求通过当前进程和活动阶段关联 run，不接受执行阶段以外的设备调用。

每端最多32个待响应请求、32个待写消息、8个入向处理槽。收发循环不被业务运行占用；取消消息直接取消目标上下文。取消后等候至多1秒的处理结束反馈，未收到则关闭连接并终止宿主拥有的插件进程树。未知动作结果不自动重放。启动握手10秒；配置/报告查询30秒；准备30秒；执行由配置声明，最高24小时；清理最长60秒。无调用方 deadline 的 SDK 请求默认15分钟。进程关闭等待至多5秒，未回收则交给宿主现有清理跟踪，阻止下一次运行绕过未完成清理。

## 生命周期与方法

| 方向/方法 | 载荷 | 结果与限制 |
| --- | --- | --- |
| 宿主 `describe` | 空对象 | Descriptor；平台、配置版本、Profile/结果 Schema、结果解释 |
| 宿主 `template` | TemplateRequest | 单个 JSON 配置；当前支持 default 模板 |
| 宿主 `compile` | CompileRequest | Compiled；私有数据、角色/目录引用、设备/存储引用、能力和预算；不操作设备 |
| 宿主 `prepare` | RunRequest | 准备一次，允许宿主能力回调 |
| 宿主 `run` | 相同 RunRequest | Result；领域判定 passed/failed，失败必须说明原因；宿主验证私有结果 Schema |
| 宿主 `cleanup` | 相同 RunRequest | 清理业务；成功后重复调用无副作用；不能和在途方法并行 |
| 宿主 `report.prepare` | ReportRequest | Report 数据文件、必需资源 ID、私有渲染状态；无设备回调 |
| 宿主 `report.render` | RenderRequest | HTML 字节（JSON 中编码为 base64）；消费冻结输入和最终链接 |
| 插件 `host.call` | Call | Feedback；do/probe/observe，经既有引擎执行并保存事实 |
| 插件 `host.event` | Event | 确认接收；序号从1连续递增，最多10000条；最后一条相同内容重传可确认 |
| 插件 `host.variable` | VariableRequest | 读取当前作用域变量；不允许插件直接篡改宿主变量 |
| 插件 `host.resource` | Resource | 返回宿主登记后的资源ID/摘要/大小；单资源不超过2 MiB |

方法异常使调用失败，宿主用 `special.plugin_call_failed` 等阶段分类保留原因。结构化业务失败在 Result / Feedback 的 Failure 中，包含 code、message 和 operationId；不能用 RPC 返回成功替代业务通过。

一期桥接动作包括点击、长按、滑动、输入、等待、重启/清理应用、读取文字、保存矩形、元素/属性/页面断言、截图和应用日志开始/结束。插件必须在 Compiled.Keywords 声明实际需要的规范关键字。`probe` 当前只支持元素存在；`observe` 支持 getText 与 specialPageSource。页面源观察最大2 MiB，带采样时间，可在插件侧进行私有页面解析。完整矩形快照及逐批原生日志读取还未形成专项公开接口，后续蓝牙迁移前补齐。

同一逻辑操作 ID 的相同内容返回已知反馈，不再执行；不同内容报 operation_conflict。宿主在调用设备前登记 unknown，再保存最终反馈。每次运行最多10000个操作，缓存预算64 MiB，超限失败而非删除去重记录。这里的去重不是崩溃恢复机制；进程重启不会续跑业务。探测明确区分 matched/not_matched/error，传输错误和附加失败不能降为“不存在”。

## 结果、报告、证据和分析

每次运行一个执行进程；运行结束先停止插件/引擎活动再释放设备资源。插件进程也加入现有清理跟踪，不能只 kill 后就宣称清理完成。宿主可清理进程、会话等资源，但插件崩溃导致业务恢复未完成时保留明确失败。

报告使用新的短生命周期插件进程，仅依赖冻结数据，不依赖原 Suite 实例。宿主先将 `special-result.json` 与 `special-analysis-input.json` 写入运行目录，再请求私有报告；报告失败时两份输入仍可读取。后者包含插件及二进制身份、Profile摘要、编译输入、实际引擎结果、设备、资源、私有结果 Schema/解释和可用的证据目录入口。

私有文件进入现有 Delivery 限额和名称/资源校验；宿主上传后提供链接映射，插件渲染自己的 HTML。必需资源缺失报交付错误，不替换成 UI 报告。当前每次报告调用都可在新进程运行；尚无单独的 CLI“重新生成报告”命令。

事实分为宿主记录的动作意图/反馈、阶段与基础设施结果，以及插件声明的业务事件/领域结果。stderr诊断最多保留64 KiB并写入证据。启动、配置、执行、清理、交付继续使用现有完整证据包；未收到结果保留 unknown，不把它转换为 passed。结果文件在报告请求前冻结，后续交付事实由外层证据包追加并封存。独立模型分析及自动报告嵌入仍未实现。

## 当前范围和验证

本轮完成外部专项 SDK、发现/配置/执行桥、私有报告与独立分析输入，以及可在仓库外构建的项目骨架。验证包含真实独立模块构建、双向子进程调用、取消、操作去重/冲突、实际引擎的模拟设备运行和独立报告生成。真机、Windows原生运行、现有两个蓝牙提供方迁出、通用观察能力补齐及正式发行装配不能用骨架测试代替。

整体迁移设计与后续验收见[专项插件化设计](https://github.com/xieliangji/soluna-dsl/blob/soluna/workbench-rebuild/docs/specials/plugin-architecture.md)。
