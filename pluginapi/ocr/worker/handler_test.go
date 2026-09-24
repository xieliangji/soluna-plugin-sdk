package ocrworker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
)

func TestImageDescriptorScopeAndDigest(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	b := []byte("image bytes")
	name := filepath.Join(dir, "input")
	os.WriteFile(name, b, 0600)
	h := &Handler{Root: root}
	r := pluginapi.ResourceDescriptor{ID: "image", AccessMode: "read", SizeBytes: int64(len(b)), SHA256: fmt.Sprintf("%x", sha256.Sum256(b)), URI: name}
	if _, err = h.readImage(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*pluginapi.ResourceDescriptor){func(r *pluginapi.ResourceDescriptor) { r.URI = filepath.Join(t.TempDir(), "input") }, func(r *pluginapi.ResourceDescriptor) { r.SizeBytes++ }, func(r *pluginapi.ResourceDescriptor) {
		r.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}} {
		copy := r
		mutate(&copy)
		if _, err = h.readImage(context.Background(), copy); err == nil {
			t.Fatal("invalid resource accepted")
		}
	}
}
