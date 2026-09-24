// Package pluginapi 包含受监管 Soluna 进程插件使用的版本化线协议契约与 Go SDK。
package pluginapi

import (
	"encoding/json"
	"time"
)

const (
	ProtocolVersion = "1.0"
	DefaultMaxFrame = 8 << 20
)

// Manifest 是启动前加载的 Schema 版本化进程声明。
// 路径在 Runner 实体化前均相对于 Manifest。
type Manifest struct {
	SchemaVersion        string               `json:"schemaVersion"`
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Version              string               `json:"version"`
	ProtocolVersion      string               `json:"protocolVersion"`
	Executable           string               `json:"executable"`
	Arguments            []string             `json:"arguments,omitempty"`
	WorkingDirectory     string               `json:"workingDirectory,omitempty"`
	EnvironmentAllowlist []string             `json:"environmentAllowlist,omitempty"`
	ChecksumSHA256       string               `json:"checksumSha256,omitempty"`
	Capabilities         []ManifestCapability `json:"capabilities"`
	Resources            ResourcePolicy       `json:"resources"`
	Restart              RestartPolicy        `json:"restart,omitempty"`
}

type ManifestCapability struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Kind       string `json:"kind"`
	Idempotent bool   `json:"idempotent,omitempty"`
}

type ResourcePolicy struct {
	ReadRoots        []string `json:"readRoots,omitempty"`
	OutputDirectory  string   `json:"outputDirectory"`
	MaxResourceBytes int64    `json:"maxResourceBytes,omitempty"`
}

type RestartPolicy struct {
	MaxRestarts  int   `json:"maxRestarts,omitempty"`
	WindowMillis int64 `json:"windowMs,omitempty"`
}

// Request 与 Response 是唯一的顶层线协议信封。
type Request struct {
	ProtocolVersion string          `json:"protocolVersion"`
	ID              string          `json:"id"`
	Method          string          `json:"method"`
	Deadline        time.Time       `json:"deadline"`
	Params          json.RawMessage `json:"params"`
}

type Response struct {
	ProtocolVersion string          `json:"protocolVersion"`
	ID              string          `json:"id"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Retryable bool              `json:"retryable"`
	Details   map[string]string `json:"details,omitempty"`
}

type HandshakeParams struct {
	ProtocolVersions []string `json:"protocolVersions"`
	RunnerVersion    string   `json:"runnerVersion"`
	InstanceID       string   `json:"instanceId"`
	Features         []string `json:"features,omitempty"`
	Limits           Limits   `json:"limits"`
}

type Limits struct {
	MaxPayloadBytes  int   `json:"maxPayloadBytes"`
	MaxResourceBytes int64 `json:"maxResourceBytes,omitempty"`
}

type HandshakeResult struct {
	PluginID         string       `json:"pluginId"`
	PluginName       string       `json:"pluginName"`
	PluginVersion    string       `json:"pluginVersion"`
	ProtocolVersion  string       `json:"protocolVersion"`
	Capabilities     []Capability `json:"capabilities"`
	MaxConcurrency   int          `json:"maxConcurrency"`
	ResourceSchemes  []string     `json:"resourceSchemes"`
	HealthIntervalMS int64        `json:"healthIntervalMs,omitempty"`
}

type Capability struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Kind        string          `json:"kind"`
	Idempotent  bool            `json:"idempotent,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type HealthResult struct {
	Ready          bool               `json:"ready"`
	DegradedReason string             `json:"degradedReason,omitempty"`
	Metrics        map[string]float64 `json:"metrics,omitempty"`
}

type ExecuteParams struct {
	Capability        string                     `json:"capability"`
	CapabilityVersion string                     `json:"capabilityVersion"`
	RunID             string                     `json:"runId"`
	ActionID          string                     `json:"actionId"`
	Arguments         json.RawMessage            `json:"arguments"`
	RuntimeVariables  map[string]json.RawMessage `json:"runtimeVariables,omitempty"`
	Resources         []ResourceDescriptor       `json:"resources,omitempty"`
}

type ExecuteResult struct {
	Status            string                     `json:"status"`
	Value             json.RawMessage            `json:"value,omitempty"`
	Message           string                     `json:"message,omitempty"`
	FailureCode       string                     `json:"failureCode,omitempty"`
	Retryable         bool                       `json:"retryable,omitempty"`
	VariableUpdates   map[string]json.RawMessage `json:"variableUpdates,omitempty"`
	ProducedResources []ResourceDescriptor       `json:"producedResources,omitempty"`
	Diagnostics       map[string]string          `json:"diagnostics,omitempty"`
}

type CancelParams struct {
	RequestID string `json:"requestId"`
}

type ShutdownParams struct {
	Reason string `json:"reason,omitempty"`
}

type ShutdownResult struct {
	Accepted bool `json:"accepted"`
}

type ResourceDescriptor struct {
	ID         string `json:"id"`
	MediaType  string `json:"mediaType"`
	SizeBytes  int64  `json:"sizeBytes"`
	SHA256     string `json:"sha256,omitempty"`
	AccessMode string `json:"accessMode"`
	URI        string `json:"uri"`
}
