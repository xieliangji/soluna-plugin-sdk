# 插件开发指引

本指引面向插件开发者和 Agent。Soluna 只生成最小接入骨架，不规定插件项目的目录、测试框架或团队协作方式；是否创建 AGENTS.md 由开发者决定。

## 开始开发

用 Soluna scaffold 创建日志、OCR 或专项插件 → 阅读对应 SDK 接口和下方分类指引 → 实现业务与私有结果 → 补业务测试 → 构建并接入 Soluna 验证。

SDK 是公共接口、数据类型及协议的唯一来源。插件固定依赖 SDK，无需克隆主仓库；只有 SDK 联调才使用本地 replace。协议、能力、SDK、插件和私有业务 Schema 的版本分别管理；不得通过修改生成项目重定义公共契约。

生成项目的 README 只引用本指引和 SDK 接口，不复制开发规则或协议。默认源码路径只是入口位置，可以按团队需要调整，保持入口与构建脚本一致即可。

## 专项测试

接口：[Provider 与 Host](../pluginapi/special/types.go)；完整契约：[special/1.0](special-protocol.md)。

直接实现 Provider，不重定义接口。入口只调用 `os.Exit(special.Main(provider))`；SDK 处理参数、清单、实例绑定、stdio、请求派发、取消与生命周期。Host 提供设备动作、事件、变量和资源回调，业务不直接依赖宿主设备驱动。

初始骨架在 provider/config.go 声明身份、平台和私有 Schema；provider/run.go 提供 Template、Compile、Prepare、Run、Cleanup；provider/report.go 提供两个报告方法。方法尚未实现时返回 ErrNotImplemented，不会操作设备、提供示例通过或假装清理成功。

开发顺序：

1. 定义私有配置 Schema → 实现 Template → 严格校验并 Compile，声明角色、关键字、预算和私有数据。
2. 实现 Prepare → Run → Cleanup，通过 Host 操作设备；传递取消和执行身份，未知反馈不换操作 ID 重放。
3. 定义私有结果 Schema 与 Descriptor.ResultGuide，说明字段、通过／失败条件、未知结果和证据关联。
4. 实现 PrepareReport → RenderReport，只消费冻结结果与资源链接，不能依赖先前进程中的状态。
5. 用 fake Host 覆盖业务分支、取消、准备失败、清理失败和独立报告；协议帧与生命周期边界由 SDK 测试覆盖。

骨架私有 Schema 为空对象占位，实现业务时自行完善结构和版本。业务结果与报告分开保存，便于独立分析；报告内容不改变实际结论。

完成业务后：把 SOLUNA_SPECIAL_PLUGIN_PATHS 指向 dist → `soluna special list` → `soluna special init <专项ID> --project <资产项目> --app <应用ID> --name check` → 填写配置 → `soluna special validate <配置路径>` → `soluna special run <配置路径> --report-root <输出目录>`。

## 本地 OCR

接口：[Handler](../pluginapi/server.go)、[OCR 类型](../pluginapi/ocr/protocol/protocol.go)；完整契约：[OCR](ocr-contract.md)。

初始业务入口在 internal/backend/backend.go，内部适配器接入 SDK Handler。实现 Health、Execute 和 Shutdown：Health 说明能否接收请求，Execute 执行初始化／识别并保留实际结果及模型身份，Shutdown 释放自身原生资源。

确认平台、模型和动态库 → 实现初始化和识别 → 核对受控图片、ROI、资源大小及摘要 → 产生结构化文字及身份 → 验证取消和关闭。

可以组合 [SDK Worker](../pluginapi/ocr/worker/handler.go) 复用初始化、图片校验、识别输出和关闭，自己提供模型工厂与真实健康探测。初始化前健康表示能接受初始化请求；只有成功识别且没有文字时才允许空结果。

入口使用 --input-root。构建得到 ocr-plugin.json，通过 SOLUNA_LOCAL_OCR_PLUGIN 接入。原生后端、模型及平台资源由插件装配，生成器不复制宿主原生实现。

## 日志断言

接口：[Handler](../pluginapi/server.go)、[消息类型](../pluginapi/protocol.go)；完整契约：[日志断言](log-contract.md)。

初始业务入口在 internal/backend/backend.go，内部适配器接入 SDK Handler。实现 Health、Execute 和 Shutdown，私有规则只读取请求授权的日志，不采集设备或触发业务命令。

明确系统规则 → 定义私有参数和结果 → 准备真实日志夹具 → 实现匹配及分类 → 覆盖平台与规则版本。

日志来源元数据不能替代文件授权；不匹配、读取失败、坏参数和未实现分别处理。把默认值、大小写、正则和平台差异记录到项目自行组织的业务说明，并配套夹具。生成器没有内置 UGREEN 或其他系统的业务规则。

构建脚本生成清单；快速构建可使用 `go build -o app-log-plugin.exe .`。核对 executable、能力、版本和摘要后，通过 Plan pluginRefs 或 SOLUNA_APP_LOG_PLUGIN_DIRS 接入。

## 验证与交付

`gofmt` → `go mod tidy` → `go test -race ./...` → `go vet ./...` → `sh build.sh`（Windows：`./build.ps1`）。这是骨架默认入口，可以由开发者按项目调整。

默认测试离线。补齐成功、业务失败、无效输入、资源错误、取消及清理测试；原生依赖另做目标平台验证，交叉构建不能代替运行验收。构建通过只证明项目可编译，不代表业务实现或真机测试通过。

stdout 只用于协议，诊断写 stderr。保留 context、运行／动作身份、资源摘要与真实原因；RPC 成功、健康就绪和业务通过不可混淆。规则未实现时明确失败，不生成虚假结果。

提交固定 SDK 版本的 go.mod/go.sum，正式发布不保留本机 replace。开发构建脚本只生成程序和摘要清单，正式分发需补齐自身依赖许可证、原生资源与归档验证。目录布局、内部抽象及 Agent 工作规则由插件维护者自己选择。


## V1 迁移草案的联调

专项项目统一名为 `soluna-special`，不同业务使用自己的插件 ID。固定依赖 SDK v0.1.5；Health revision 为 `v1-draft-20260925`。生命周期与资源接口见 [special-protocol.md](special-protocol.md)。若业务包含多轮执行，Compiled 声明 Units / UnitTimeoutMs，每个 Run 处理当前 Unit，避免把 200 轮塞进一个不可观测的调用。

大结果和日志通过资源 ID 分块传输，不读取宿主路径，不在结果里内嵌整份大日志。报告在新进程中处理，通过 ResourceReporter 读取冻结资源，不能再操作设备。Private ResultGuide 说明判定来源、缺失证据及资源关联，为后续独立 AI 分析保留依据。

集成夹具由 [pluginapi/plugintest](../pluginapi/plugintest/types.go) 定义。`soluna plugin test` 的 Host 只匹配显式的操作、观察与事件预期，资源传输可自动保存和回读；report 阶段禁止设备调用。它是离线集成测试，不等于真机验收。
