package ocrprotocol

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xieliangji/soluna-plugin-sdk/internal/semver"
)

// Package is independent of the Soluna release version. Paths are package-relative.
type Package struct {
	SchemaVersion     string `json:"schemaVersion"`
	ProtocolVersion   string `json:"protocolVersion"`
	CapabilityVersion string `json:"capabilityVersion"`
	Version           string `json:"version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	Executable        string `json:"executable"`
	SHA256            string `json:"sha256"`
}

func LoadPackage(ctx context.Context, filename, goos, arch string) (Package, string, error) {
	var p Package
	f, err := os.Open(filename)
	if err != nil {
		return p, "", err
	}
	raw, err := io.ReadAll(io.LimitReader(f, 65537))
	closeErr := f.Close()
	if err != nil {
		return p, "", err
	}
	if closeErr != nil {
		return p, "", closeErr
	}
	if len(raw) > 65536 {
		return p, "", fmt.Errorf("OCR package manifest exceeds limit")
	}
	if err = Decode(raw, &p); err != nil {
		return p, "", err
	}
	if err = p.Validate(goos, arch); err != nil {
		return p, "", err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return p, "", err
	}
	defer root.Close()
	file, err := root.Open(p.Executable)
	if err != nil {
		return p, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return p, "", err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 512<<20 {
		return p, "", fmt.Errorf("invalid OCR executable")
	}
	if goos != "windows" && info.Mode().Perm()&0111 == 0 {
		return p, "", fmt.Errorf("OCR executable lacks executable permission")
	}
	digest, err := hashReader(ctx, file)
	if err != nil {
		return p, "", err
	}
	if digest != p.SHA256 {
		return p, "", fmt.Errorf("OCR executable checksum mismatch")
	}
	build, err := buildinfo.Read(file)
	if err != nil {
		return p, "", err
	}
	settings := map[string]string{}
	for _, s := range build.Settings {
		settings[s.Key] = s.Value
	}
	if settings["GOOS"] != goos || settings["GOARCH"] != arch {
		return p, "", fmt.Errorf("OCR executable build host differs from manifest")
	}
	executable, err := filepath.Abs(filepath.Join(filepath.Dir(filename), p.Executable))
	return p, executable, err
}
func (p Package) Validate(goos, arch string) error {
	if p.SchemaVersion != "1.0" || p.ProtocolVersion != "1.0" || p.CapabilityVersion != CapabilityVersion {
		return fmt.Errorf("unsupported local OCR package contract")
	}
	if _, err := semver.Parse(p.Version); err != nil {
		return fmt.Errorf("invalid OCR plugin version: %w", err)
	}
	if p.OS != goos || p.Arch != arch || !((goos == "darwin" && arch == "arm64") || (goos == "windows" && arch == "amd64")) {
		return fmt.Errorf("OCR package host mismatch: %s/%s, need %s/%s", p.OS, p.Arch, goos, arch)
	}
	if p.Executable == "" || p.Executable == "." || p.Executable == ".." || strings.ContainsAny(p.Executable, "/\\:") {
		return fmt.Errorf("OCR executable must be a package-local filename")
	}
	if goos == "windows" && !strings.HasSuffix(p.Executable, ".exe") {
		return fmt.Errorf("Windows OCR executable must end in .exe")
	}
	if len(p.SHA256) != 64 || strings.Trim(p.SHA256, "0123456789abcdef") != "" {
		return fmt.Errorf("invalid OCR executable checksum")
	}
	return nil
}

// WritePackage inventories a prebuilt plugin without running the target binary.
func WritePackage(ctx context.Context, binary, version, goos, arch, output string) error {
	absolute, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	dest, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if filepath.Dir(absolute) != filepath.Dir(dest) {
		return fmt.Errorf("OCR manifest and executable must share a directory")
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return err
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if settings["GOOS"] != goos || settings["GOARCH"] != arch {
		return fmt.Errorf("OCR executable build host does not match package host")
	}
	digest, err := HashFile(ctx, binary)
	if err != nil {
		return err
	}
	p := Package{"1.0", "1.0", CapabilityVersion, version, goos, arch, filepath.Base(binary), digest}
	if err = p.Validate(goos, arch); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, append(raw, '\n'), 0644)
}
func HashFile(ctx context.Context, filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(ctx, f)
}
func hashReader(ctx context.Context, r io.Reader) (string, error) {
	h := sha256.New()
	b := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := r.Read(b)
		h.Write(b[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
