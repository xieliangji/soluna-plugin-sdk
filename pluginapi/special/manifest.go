package special

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func ValidID(s string) bool { return identifier.MatchString(s) }
func (d Descriptor) Validate() error {
	if !ValidID(d.ID) || d.Version == "" || len(d.Platforms) == 0 || len(d.ProfileVersions) == 0 || d.ResultGuide == "" {
		return fmt.Errorf("special: incomplete descriptor")
	}
	for _, p := range d.Platforms {
		if p != "android" && p != "ios" {
			return fmt.Errorf("special: invalid platform")
		}
	}
	for _, v := range d.ProfileVersions {
		if v < 1 {
			return fmt.Errorf("special: invalid profile version")
		}
	}
	for _, s := range []json.RawMessage{d.ResultSchema, d.ProfileSchema} {
		var m map[string]any
		if json.Unmarshal(s, &m) != nil || len(m) == 0 {
			return fmt.Errorf("special: schema must be an object")
		}
	}
	return nil
}
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteManifest binds a locally built binary to its descriptor. No downloads.
func WriteManifest(path, binary, pluginID string, d Descriptor) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if !ValidID(pluginID) {
		return fmt.Errorf("special: invalid plugin ID")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	hash, err := HashFile(absolute)
	if err != nil {
		return err
	}
	directory, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(directory, absolute)
	if err != nil {
		return err
	}
	m := Manifest{ProtocolVersion: ProtocolVersion, PluginID: pluginID, Executable: filepath.ToSlash(relative), SHA256: hash, Descriptor: d}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}
