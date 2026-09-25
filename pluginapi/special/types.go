// Package special is the versioned SDK for independently built special tests.
// A provider owns its workflow and reports; Host owns device operations.
package special

import (
	"context"
	"encoding/json"
	"time"
)

const ProtocolVersion = "special/1.0"
const MaxFrame = 8 << 20
const MaxCalls = 10000

type Descriptor struct {
	ID              string          `json:"id"`
	Version         string          `json:"version"`
	Platforms       []string        `json:"platforms"`
	ProfileVersions []int           `json:"profileVersions"`
	ResultSchema    json.RawMessage `json:"resultSchema"`
	ProfileSchema   json.RawMessage `json:"profileSchema"`
	ResultGuide     string          `json:"resultGuide"`
}
type Manifest struct {
	ProtocolVersion string     `json:"protocolVersion"`
	PluginID        string     `json:"pluginId"`
	Executable      string     `json:"executable"`
	SHA256          string     `json:"sha256"`
	Descriptor      Descriptor `json:"descriptor"`
}
type TemplateRequest struct {
	AppID     string `json:"appId"`
	ProfileID string `json:"profileId"`
}
type CompileRequest struct {
	Profile json.RawMessage `json:"profile"`
}
type Catalog struct {
	ID   string `json:"id"`
	File string `json:"file"`
}
type Role struct {
	Ref      string         `json:"ref"`
	Bindings map[string]any `json:"bindings,omitempty"`
}
type Compiled struct {
	ProductModel    string          `json:"productModel,omitempty"`
	ApplicationName string          `json:"applicationName,omitempty"`
	SubjectName     string          `json:"subjectName,omitempty"`
	SubjectID       string          `json:"subjectId,omitempty"`
	Units           int             `json:"units,omitempty"`
	UnitTimeoutMs   int64           `json:"unitTimeoutMs,omitempty"`
	PrepareMs       int64           `json:"prepareMs,omitempty"`
	MaxUnitCalls    int             `json:"maxUnitCalls,omitempty"`
	ImplicitWaitMs  *int64          `json:"implicitWaitMs,omitempty"`
	Capabilities    map[string]any  `json:"capabilities,omitempty"`
	ExternalReport  string          `json:"externalReport,omitempty"`
	ProfileID       string          `json:"profileId"`
	ProfileVersion  int             `json:"profileVersion"`
	Name            string          `json:"name"`
	AppID           string          `json:"appId"`
	Platform        string          `json:"platform"`
	DeviceConfig    string          `json:"deviceConfig"`
	ArtifactStore   string          `json:"artifactStore"`
	Catalogs        []Catalog       `json:"catalogs,omitempty"`
	Roles           map[string]Role `json:"roles,omitempty"`
	Keywords        []string        `json:"keywords"`
	TimeoutMs       int64           `json:"timeoutMs"`
	CleanupMs       int64           `json:"cleanupMs"`
	Data            json.RawMessage `json:"data"`
}
type RunRequest struct {
	Epoch    string   `json:"epoch,omitempty"`
	Unit     int      `json:"unit,omitempty"`
	RunID    string   `json:"runId"`
	Compiled Compiled `json:"compiled"`
}
type Failure struct {
	Retryable   bool              `json:"retryable,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	OperationID string            `json:"operationId,omitempty"`
}
type Result struct {
	Status            string          `json:"status"`
	Data              json.RawMessage `json:"data"`
	PrimaryFailure    *Failure        `json:"primaryFailure,omitempty"`
	SecondaryFailures []Failure       `json:"secondaryFailures,omitempty"`
}
type Call struct {
	Wait      *WaitPolicy     `json:"wait,omitempty"`
	ID        string          `json:"id"`
	Mode      string          `json:"mode"` // do, probe, observe
	Keyword   string          `json:"keyword"`
	Role      string          `json:"role,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
	TimeoutMs int64           `json:"timeoutMs"`
}
type Feedback struct {
	Message           string          `json:"message,omitempty"`
	Attempts          int             `json:"attempts,omitempty"`
	StartedAt         *time.Time      `json:"startedAt,omitempty"`
	FinishedAt        *time.Time      `json:"finishedAt,omitempty"`
	Status            string          `json:"status"`
	ProbeState        string          `json:"probeState,omitempty"`
	Failure           *Failure        `json:"failure,omitempty"`
	SecondaryFailures []Failure       `json:"secondaryFailures,omitempty"`
	ObservedAt        time.Time       `json:"observedAt"`
	Value             json.RawMessage `json:"value,omitempty"`
}
type Event struct {
	Sequence int64           `json:"sequence"`
	UnitID   string          `json:"unitId,omitempty"`
	Phase    string          `json:"phase"`
	Kind     string          `json:"kind"`
	Data     json.RawMessage `json:"data"`
}
type VariableRequest struct {
	Scope string `json:"scope"`
	Name  string `json:"name"`
}
type Variable struct {
	Found bool            `json:"found"`
	Value json.RawMessage `json:"value,omitempty"`
}
type Resource struct {
	Offset      int64  `json:"offset,omitempty"`
	Total       int64  `json:"total,omitempty"`
	Commit      bool   `json:"commit,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Data        []byte `json:"data"`
}
type ResourceReceipt struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}
type File struct {
	Resource    *ResourceReceipt `json:"resource,omitempty"`
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	ContentType string           `json:"contentType"`
	Data        []byte           `json:"data"`
}
type ReportRequest struct {
	Result  Result          `json:"result"`
	Summary json.RawMessage `json:"summary"`
}
type Report struct {
	Files             []File          `json:"files"`
	RequiredResources []string        `json:"requiredResources,omitempty"`
	State             json.RawMessage `json:"state"`
}
type RenderRequest struct {
	Report Report            `json:"report"`
	Links  map[string]string `json:"links"`
}

// Provider is the sole special-plugin implementation contract. Projects implement
// it directly; they must not redeclare a local copy of this interface.
// Provider methods may return errors; execution failure is distinct from a
// completed test whose Result.Status is failed. Report methods must be stateless.
type Provider interface {
	Describe() Descriptor
	Template(context.Context, TemplateRequest) (json.RawMessage, error)
	Compile(context.Context, CompileRequest) (Compiled, error)
	Prepare(context.Context, RunRequest, Host) error
	Run(context.Context, RunRequest, Host) (Result, error)
	Cleanup(context.Context, RunRequest, Host) error
	PrepareReport(context.Context, ReportRequest) (Report, error)
	RenderReport(context.Context, RenderRequest) ([]byte, error)
}
type Host interface {
	ReadResource(context.Context, ResourceRead) (ResourceChunk, error)
	Observe(context.Context, ObservationRequest) (Observation, error)
	Call(context.Context, Call) (Feedback, error)
	Event(context.Context, Event) error
	Variable(context.Context, VariableRequest) (Variable, error)
	Resource(context.Context, Resource) (ResourceReceipt, error)
}
