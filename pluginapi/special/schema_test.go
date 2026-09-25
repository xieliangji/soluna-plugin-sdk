package special

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestPublishedSchemaListsMigrationMethods(t *testing.T) {
	raw, err := os.ReadFile("../../contracts/special-plugin-rpc.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"describe", "health", "compile", "prepare", "run", "cleanup", "snapshot", "report.prepare", "report.render", "host.call", "host.observe", "host.event", "host.resource", "host.resource.read"} {
		if !slices.Contains(schema.Properties["method"].Enum, method) {
			t.Fatalf("missing schema method %s", method)
		}
	}
}
