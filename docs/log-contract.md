# 日志断言插件契约

插件身份 `example-log`，传输协议 `1.0`，能力 `exampleAppLogAssertion` 版本 `1.0`，类型 assertion，幂等，单并发。

## 独立开发的实现契约

实现入口为本项目的 [`adapter.Backend`](README.md)。它由适配层消费，业务实现必须提供全部三个方法：

```go
type Backend interface {
    Health(context.Context) (pluginapi.HealthResult, error)
    Execute(context.Context, pluginapi.ExecuteParams) (pluginapi.ExecuteResult, error)
    Shutdown(context.Context) error
}
```

在本项目 `internal/` 下实现后端，在程序入口调用 `adapter.New(version, backend)`，并增加 `var _ adapter.Backend = (*Backend)(nil)` 编译检查。当前入口注入 backend.Implementation，业务尚未实现，明确返回未就绪和失败。帧、Server 和通用值类型仍使用固定版本的公开 SDK，不复制一份通信结构；业务实现不需要导入宿主 internal 或修改宿主源码。

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

本项目 [`handler_test.go`](README.md) 提供最小假后端和执行证据透传样例，[`health_test.go`](README.md) 覆盖健康、降级、探测错误、原上下文及取消。它们用于验证接线，不是生产业务实现。单独在本项目运行开发指南中的测试即可；后续业务必须补真实输入的成功、失败、资源错误及运行中取消用例。

## 日志能力

Soluna 传入的 Arguments 为 args、source、readLimit；args 是断言私有参数，source 是日志来源元数据，readLimit 是读取预算。真正的日志访问以 Resources 授权文件为准。不能直接信任 source 中的任意路径。SDK 通用 ResourceDescriptor 使用 id、uri、mediaType、sizeBytes、sha256、accessMode。

后端只读取已授权日志，不采集设备、不触发业务命令。具体系统的匹配规则、默认值、大小写、正则、平台分支及私有参数须在本项目定义，并附成功／不匹配／无效输入夹具。

业务不匹配返回 failed / action.assertion_mismatch；缺文件、坏参数、无法读取或异常退出不能当成断言 false。规则尚未实现返回 failed / plugin.not_implemented。生成项目不包含 UGREEN 或任何外部系统的默认匹配规则。
