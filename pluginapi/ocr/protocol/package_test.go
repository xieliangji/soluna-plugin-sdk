package ocrprotocol

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPackageRejectsHostVersionAndTraversal(t *testing.T) {
	p := Package{"1.0", "1.0", CapabilityVersion, "1.0.0", "darwin", "arm64", "soluna-ocr", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := p.Validate("darwin", "arm64"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Package){func(p *Package) { p.Executable = "../ocr" }, func(p *Package) { p.Version = "bad" }, func(p *Package) { p.ProtocolVersion = "2.0" }, func(p *Package) { p.Arch = "amd64" }, func(p *Package) { p.SHA256 = "bad" }} {
		copy := p
		mutate(&copy)
		if copy.Validate("darwin", "arm64") == nil {
			t.Fatal("invalid package accepted", copy)
		}
	}
}
func TestInventoryPrebuiltBinaryAndRejectTampering(t *testing.T) {
	if !((runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") || (runtime.GOOS == "windows" && runtime.GOARCH == "amd64")) {
		t.Skip("OCR binary inventory requires a supported release host")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binary := filepath.Join(root, "ocr.exe")
	os.WriteFile(binary, raw, 0700)
	manifest := filepath.Join(root, "ocr-plugin.json")
	if err = WritePackage(context.Background(), binary, Version, runtime.GOOS, runtime.GOARCH, manifest); err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoadPackage(context.Background(), manifest, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(binary, []byte("modified"), 0700)
	if _, _, err = LoadPackage(context.Background(), manifest, runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("tampered executable accepted")
	}
}
