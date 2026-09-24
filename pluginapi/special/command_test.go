package special

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type commandProvider struct{ Provider }

func (commandProvider) Describe() Descriptor {
	return Descriptor{ID: "test", Version: "0.1.0", Platforms: []string{"android"}, ProfileVersions: []int{1}, ProfileSchema: json.RawMessage(`{"type":"object"}`), ResultSchema: json.RawMessage(`{"type":"object"}`), ResultGuide: "test"}
}
func TestCommandManifestAndArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := command(context.Background(), commandProvider{}, []string{"--manifest", path}, io.Discard); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	executable, _ := os.Executable()
	hash, err := HashFile(executable)
	if err != nil || m.SHA256 != hash || m.PluginID != "test" || m.ProtocolVersion != ProtocolVersion {
		t.Fatalf("manifest %+v %v", m, err)
	}
	for _, args := range [][]string{{"--unknown"}, {"extra"}, {"--manifest"}} {
		if err := command(context.Background(), commandProvider{}, args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := command(context.Background(), nil, []string{"--manifest", path}, io.Discard); err == nil {
		t.Fatal("accepted nil provider")
	}
	if err := command(context.Background(), nil, []string{"--help"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOLUNA_SPECIAL_INSTANCE", "")
	if err := command(context.Background(), commandProvider{}, nil, io.Discard); err == nil {
		t.Fatal("accepted missing instance")
	}
}
