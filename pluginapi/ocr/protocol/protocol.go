// Package ocrprotocol defines the independently versioned local OCR capability.
package ocrprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	visual "github.com/xieliangji/soluna-plugin-sdk/ocr"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
)

const (
	PluginID          = "soluna.local-ocr"
	Version           = "1.0.0"
	Initialize        = "ocr.initialize"
	Recognize         = "ocr.recognize"
	CapabilityVersion = "1.0.0"
	MaxImageBytes     = 16 << 20
)

type Config struct {
	BundlePath     string `json:"bundlePath"`
	LibraryPath    string `json:"libraryPath"`
	LibrarySHA256  string `json:"librarySha256"`
	IntraOpThreads int    `json:"intraOpThreads"`
	InterOpThreads int    `json:"interOpThreads"`
}

type Request struct {
	ROI *visual.Region `json:"roi,omitempty"`
}
type Identity struct {
	PluginVersion    string `json:"pluginVersion"`
	BundleID         string `json:"bundleId"`
	BundleSHA256     string `json:"bundleSha256"`
	DetectorSHA256   string `json:"detectorSha256"`
	RecognizerSHA256 string `json:"recognizerSha256"`
	LibrarySHA256    string `json:"librarySha256"`
}

type Result struct {
	Items    []visual.Text `json:"items"`
	Identity Identity      `json:"identity"`
}

func Capabilities() []pluginapi.ManifestCapability {
	return []pluginapi.ManifestCapability{
		{Name: Initialize, Version: CapabilityVersion, Kind: "action"},
		{Name: Recognize, Version: CapabilityVersion, Kind: "action", Idempotent: true},
	}
}

func Decode(raw []byte, target any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("OCR payload is required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("OCR payload must contain one JSON value")
	}
	return nil
}
