# Soluna 插件 SDK

独立 Go module，只依赖标准库。插件开发入口是 Soluna 的 `scaffold` 命令；SDK 不包含宿主、设备驱动、识别算法或日志业务规则。

- `pluginapi`：UI 自动化能力协议 1.0、帧、Server、健康／执行／关闭与资源类型。
- `pluginapi/ocr/protocol`：OCR 能力 1.0.0 的输入输出、身份和成品清单。
- `pluginapi/ocr/worker`：可选 OCR Worker，复用模型初始化、受控图片校验、识别结果与关闭；识别工厂由插件提供。
- `ocr`：OCR 值类型与错误分类；不依赖主项目的 visual 模块。
- `pluginapi/special`：专项协议 special/1.0、双向 Host 调用、生命周期、独立结果与私有报告。

UI 能力和专项协议分别版本化，不能以统一 Execute 替代专项生命周期。新增类别扩展自己的协议与能力契约，不修改现有类别的含义。

[维护规则](AGENTS.md) → [协议入口](docs/README.md)。公共接口、契约与[开发指引](docs/development.md)统一在 SDK；生成项目只提供最小业务骨架与 README，不生成 AGENTS.md 或开发文档副本；业务实现不需要克隆 Soluna 主仓库。

## 验证

```sh
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
```

SDK 版本与进程协议、能力版本、插件版本分别管理。v0.1.0 抽取已有协议实现，保持线上字节和语义；正式消费者固定版本，联调才显式添加 SDK replace。SDK 为 Public，获取依赖无需 GitHub 登录或 GOPRIVATE。

当前推荐版本为 **v0.1.3**：提供专项统一进程入口 `special.Main` 和明确的未实现错误，公共接口仍为 `special.Provider`。协议保持 special/1.0。

V1 草案此次补充 Health revision、阶段 epoch、顺序业务单元、Wait、通用观察、分块资源和独立资源报告。迁移细节见 [专项协议](docs/special-protocol.md)。
