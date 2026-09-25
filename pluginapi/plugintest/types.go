// Package plugintest defines fixture contracts for Soluna's offline integration
// runner. The SDK contains no test runner or device implementation.
package plugintest

import (
	"encoding/json"
	"fmt"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
)

const Version = "1.0"

type Suite struct {
	SchemaVersion string `json:"schemaVersion"`
	Kind          string `json:"kind"` // app-log, ocr, special
	Cases         []Case `json:"cases"`
}
type Case struct {
	Name      string `json:"name"`
	TimeoutMs int64  `json:"timeoutMs"`
	// UI plugins execute these calls in one process, allowing OCR initialization.
	Steps []Step `json:"steps,omitempty"`
	// Special plugins compile this profile, then prepare/run/cleanup/report.
	Profile json.RawMessage `json:"profile,omitempty"`
	Expect  *Expectation    `json:"expect,omitempty"`
	Host    []HostCall      `json:"host,omitempty"`
}
type Step struct {
	Name      string                  `json:"name"`
	TimeoutMs int64                   `json:"timeoutMs"`
	Request   pluginapi.ExecuteParams `json:"request"`
	Files     []InputFile             `json:"files,omitempty"`
	Expect    Expectation             `json:"expect"`
	// Optional assertions against claimed output JSON files, addressed by resource ID.
	Outputs map[string]json.RawMessage `json:"outputs,omitempty"`
}
type InputFile struct {
	ID        string `json:"id"`
	Path      string `json:"path"` // relative to suite; copied into a runner-owned input root
	MediaType string `json:"mediaType"`
}
type Expectation struct {
	// Object subset: every expected field must match; arrays match in full.
	Result        json.RawMessage `json:"result,omitempty"`
	ErrorContains string          `json:"errorContains,omitempty"`
}
type HostCall struct {
	Phase  string          `json:"phase"`  // prepare, run, cleanup
	Method string          `json:"method"` // host.call, host.event, host.variable, host.resource
	Params json.RawMessage `json:"params"` // object subset; no silent wildcard
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func object(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return len(raw) > 0 && json.Unmarshal(raw, &value) == nil && value != nil
}
func (e Expectation) Validate(requireStatus bool) error {
	if e.ErrorContains != "" {
		if len(e.Result) > 0 {
			return fmt.Errorf("result and errorContains are mutually exclusive")
		}
		return nil
	}
	if !object(e.Result) {
		return fmt.Errorf("expected result must be an object")
	}
	if requireStatus {
		var value struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(e.Result, &value)
		if value.Status != "passed" && value.Status != "failed" {
			return fmt.Errorf("expected result must specify passed or failed status")
		}
	}
	return nil
}
func (s Suite) Validate() error {
	if s.SchemaVersion != Version || (s.Kind != "app-log" && s.Kind != "ocr" && s.Kind != "special") {
		return fmt.Errorf("unsupported fixture version or kind")
	}
	if len(s.Cases) == 0 || len(s.Cases) > 100 {
		return fmt.Errorf("suite requires 1..100 cases")
	}
	names := map[string]bool{}
	for _, c := range s.Cases {
		if c.Name == "" || names[c.Name] || c.TimeoutMs < 1 || c.TimeoutMs > 300000 {
			return fmt.Errorf("case name must be unique; timeoutMs must be 1..300000")
		}
		names[c.Name] = true
		if s.Kind == "special" {
			if len(c.Steps) > 0 || !object(c.Profile) || c.Expect == nil || c.Expect.ErrorContains != "" {
				return fmt.Errorf("special case requires profile and expected business result, without UI steps")
			}
			if err := c.Expect.Validate(true); err != nil {
				return err
			}
			if len(c.Host) > 1000 {
				return fmt.Errorf("too many host calls")
			}
			for _, h := range c.Host {
				if h.Phase != "prepare" && h.Phase != "run" && h.Phase != "cleanup" && h.Phase != "report" {
					return fmt.Errorf("invalid host phase")
				}
				if h.Method != "host.call" && h.Method != "host.event" && h.Method != "host.variable" && h.Method != "host.resource" && h.Method != "host.resource.read" && h.Method != "host.observe" {
					return fmt.Errorf("invalid host method")
				}
				if !object(h.Params) || !nonemptyObject(h.Params) || (h.Error == "" && !json.Valid(h.Result)) || (h.Error != "" && len(h.Result) > 0) {
					return fmt.Errorf("host stub needs matching params and exactly one result or error")
				}
			}
		} else {
			if len(c.Profile) > 0 || c.Expect != nil || len(c.Host) > 0 || len(c.Steps) == 0 || len(c.Steps) > 100 {
				return fmt.Errorf("UI case requires 1..100 steps only")
			}
			seen := map[string]bool{}
			for _, step := range c.Steps {
				if step.Name == "" || seen[step.Name] || step.TimeoutMs < 1 || step.TimeoutMs > c.TimeoutMs {
					return fmt.Errorf("invalid step name or timeout")
				}
				seen[step.Name] = true
				if step.Request.Capability == "" || step.Request.CapabilityVersion == "" || !object(step.Request.Arguments) || len(step.Request.Resources) > 0 {
					return fmt.Errorf("step needs capability/version/arguments; use files instead of resource descriptors")
				}
				if err := step.Expect.Validate(true); err != nil {
					return err
				}
				if s.Kind == "ocr" && step.Request.Capability != "ocr.initialize" && step.Request.Capability != "ocr.recognize" {
					return fmt.Errorf("invalid OCR capability")
				}
				if len(step.Files) > 32 || len(step.Outputs) > 32 {
					return fmt.Errorf("too many resources")
				}
				ids := map[string]bool{}
				for _, f := range step.Files {
					if f.ID == "" || ids[f.ID] || f.Path == "" || f.MediaType == "" {
						return fmt.Errorf("invalid input file")
					}
					ids[f.ID] = true
				}
				for id, expected := range step.Outputs {
					if id == "" || !json.Valid(expected) {
						return fmt.Errorf("invalid output expectation")
					}
				}
			}
		}
	}
	return nil
}

func nonemptyObject(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && len(value) > 0
}
