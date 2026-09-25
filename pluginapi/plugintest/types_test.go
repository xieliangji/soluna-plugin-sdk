package plugintest

import (
	"encoding/json"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi/special"
	"testing"
)

func TestFixtureBoundaries(t *testing.T) {
	valid := Suite{SchemaVersion: "1.0", Kind: "app-log", Cases: []Case{{Name: "one", TimeoutMs: 1000, Steps: []Step{{Name: "one", TimeoutMs: 100, Request: pluginapi.ExecuteParams{Capability: "check", CapabilityVersion: "1.0", Arguments: json.RawMessage(`{}`)}, Expect: Expectation{Result: json.RawMessage(`{"status":"passed"}`)}}}}}}
	b, _ := json.Marshal(valid)
	for _, mutate := range []func(*Suite){
		func(s *Suite) { s.SchemaVersion = "2.0" }, func(s *Suite) { s.Cases = nil }, func(s *Suite) { s.Cases[0].TimeoutMs = 0 },
		func(s *Suite) { s.Cases = append(s.Cases, s.Cases[0]) }, func(s *Suite) { s.Cases[0].Steps[0].Expect = Expectation{Result: json.RawMessage(`{}`)} },
		func(s *Suite) { s.Cases[0].Steps[0].Expect.ErrorContains = "error" }, func(s *Suite) { s.Kind = "special" },
	} {
		var s Suite
		_ = json.Unmarshal(b, &s)
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Fatal("accepted invalid fixture", s)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	var s Suite
	if err := special.Decode([]byte(`{"schemaVersion":"1.0","kind":"app-log","cases":[],"unknown":true}`), &s); err == nil {
		t.Fatal("accepted unknown field")
	}
}
func TestHostExpectationsCannotBeEmptyWildcards(t *testing.T) {
	s := Suite{SchemaVersion: "1.0", Kind: "special", Cases: []Case{{Name: "one", TimeoutMs: 1000, Profile: json.RawMessage(`{}`), Expect: &Expectation{Result: json.RawMessage(`{"status":"passed"}`)}, Host: []HostCall{{Phase: "run", Method: "host.call", Params: json.RawMessage(`{ }`), Result: json.RawMessage(`{}`)}}}}}
	if err := s.Validate(); err == nil {
		t.Fatal("accepted empty params")
	}
	s.Cases[0].Host[0].Params = json.RawMessage(`{"id":"check"}`)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
