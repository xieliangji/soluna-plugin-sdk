package special

import (
	"encoding/json"
	"testing"
)

func TestDescriptorTemplateNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		templates []string
		valid     bool
	}{
		{"legacy default", nil, true},
		{"two profiles", []string{"default", "v2.5.x"}, true},
		{"duplicate", []string{"default", "default"}, false},
		{"empty name", []string{""}, false},
		{"path", []string{"../default"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Descriptor{ID: "fixture", Version: "1.0.0", Platforms: []string{"android"}, ProfileVersions: []int{1}, ResultGuide: "fixture", ProfileSchema: json.RawMessage(`{"type":"object"}`), ResultSchema: json.RawMessage(`{"type":"object"}`), Templates: tc.templates}
			if err := d.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
