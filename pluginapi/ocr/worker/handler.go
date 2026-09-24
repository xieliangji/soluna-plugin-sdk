// Package ocrworker serves local OCR through the existing supervised plugin protocol.
package ocrworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	visual "github.com/xieliangji/soluna-plugin-sdk/ocr"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
	"github.com/xieliangji/soluna-plugin-sdk/pluginapi/ocr/protocol"
)

type Recognizer interface {
	visual.TextRecognizer
	Close() error
}
type Factory func(context.Context, ocrprotocol.Config) (Recognizer, ocrprotocol.Identity, error)
type Handler struct {
	Version    string
	Root       *os.Root
	Open       Factory
	mu         sync.Mutex
	recognizer Recognizer
	identity   ocrprotocol.Identity
}

func (h *Handler) Handshake(ctx context.Context, p pluginapi.HandshakeParams) (pluginapi.HandshakeResult, error) {
	capabilities := []pluginapi.Capability{}
	for _, c := range ocrprotocol.Capabilities() {
		capabilities = append(capabilities, pluginapi.Capability{Name: c.Name, Version: c.Version, Kind: c.Kind, Idempotent: c.Idempotent})
	}
	return pluginapi.HandshakeResult{PluginID: ocrprotocol.PluginID, PluginName: "Local Paddle OCR", PluginVersion: h.Version, ProtocolVersion: pluginapi.ProtocolVersion, Capabilities: capabilities, MaxConcurrency: 1, ResourceSchemes: []string{"file"}}, nil
}
func (h *Handler) Health(context.Context) (pluginapi.HealthResult, error) {
	return pluginapi.HealthResult{Ready: h.Root != nil && h.Open != nil}, nil
}
func (h *Handler) Execute(ctx context.Context, p pluginapi.ExecuteParams) (pluginapi.ExecuteResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return pluginapi.ExecuteResult{}, err
	}
	if p.CapabilityVersion != ocrprotocol.CapabilityVersion {
		return failed(visual.CodeResponseInvalid, errors.New("unsupported OCR capability version")), nil
	}
	if p.Capability == ocrprotocol.Initialize {
		if h.recognizer != nil {
			return failed(visual.CodeResponseInvalid, errors.New("OCR already initialized")), nil
		}
		var config ocrprotocol.Config
		if err := ocrprotocol.Decode(p.Arguments, &config); err != nil {
			return failed(visual.CodeModelUnavailable, err), nil
		}
		r, identity, err := h.Open(ctx, config)
		if r != nil {
			h.recognizer = r
		}
		if err != nil {
			return failed(visual.CodeNativeUnavailable, err), nil
		}
		h.identity = identity
		h.identity.PluginVersion = h.Version
		return success(h.identity)
	}
	if p.Capability != ocrprotocol.Recognize || h.recognizer == nil {
		return failed(visual.CodeNativeUnavailable, errors.New("OCR is not initialized")), nil
	}
	var request ocrprotocol.Request
	if err := ocrprotocol.Decode(p.Arguments, &request); err != nil {
		return failed(visual.CodeResponseInvalid, err), nil
	}
	if len(p.Resources) != 1 {
		return failed(visual.CodeImageInvalid, errors.New("one image resource required")), nil
	}
	encoded, err := h.readImage(ctx, p.Resources[0])
	if err != nil {
		return failed(visual.CodeImageInvalid, err), nil
	}
	items, err := h.recognizer.Recognize(ctx, encoded, request.ROI)
	if err != nil {
		if ctx.Err() != nil {
			return pluginapi.ExecuteResult{}, ctx.Err()
		}
		return failed(visual.CodeRecognizerFailed, err), nil
	}
	return h.result(ocrprotocol.Result{Items: items, Identity: h.identity})
}
func (h *Handler) readImage(ctx context.Context, r pluginapi.ResourceDescriptor) ([]byte, error) {
	if r.ID != "image" || r.AccessMode != "read" || r.SizeBytes <= 0 || r.SizeBytes > ocrprotocol.MaxImageBytes || len(r.SHA256) != 64 || !filepath.IsAbs(r.URI) {
		return nil, errors.New("invalid OCR image descriptor")
	}
	// The parent grants only its private input directory, never arbitrary paths.
	if filepath.Clean(filepath.Dir(r.URI)) != filepath.Clean(h.Root.Name()) {
		return nil, errors.New("OCR image is outside the input directory")
	}
	f, err := h.Root.Open(filepath.Base(r.URI))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != r.SizeBytes {
		return nil, errors.New("OCR image size/type mismatch")
	}
	b, err := io.ReadAll(io.LimitReader(f, ocrprotocol.MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if int64(len(b)) != r.SizeBytes || hex.EncodeToString(sum[:]) != r.SHA256 {
		return nil, errors.New("OCR image checksum mismatch")
	}
	return b, ctx.Err()
}
func (h *Handler) Shutdown(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.recognizer != nil {
		err := h.recognizer.Close()
		if err != nil {
			return err
		}
		h.recognizer = nil
	}
	return nil
}
func success(v any) (pluginapi.ExecuteResult, error) {
	raw, err := json.Marshal(v)
	return pluginapi.ExecuteResult{Status: "passed", Value: raw}, err
}
func failed(code string, err error) pluginapi.ExecuteResult {
	var v *visual.Error
	retry := false
	if errors.As(err, &v) {
		code = v.Code
		retry = v.Retryable
	}
	message := fmt.Sprint(err)
	if len(message) > 512 {
		message = message[:512]
	}
	return pluginapi.ExecuteResult{Status: "failed", FailureCode: code, Message: message, Retryable: retry}
}

func (h *Handler) result(v ocrprotocol.Result) (pluginapi.ExecuteResult, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return pluginapi.ExecuteResult{}, err
	}
	if len(raw) > 16<<20 {
		return failed(visual.CodeResponseInvalid, errors.New("OCR result exceeds limit")), nil
	}
	name := filepath.Join(h.Root.Name(), "output", "result.json")
	f, err := h.Root.OpenFile("output/result.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return pluginapi.ExecuteResult{}, err
	}
	_, err = f.Write(raw)
	err = errors.Join(err, f.Close())
	if err != nil {
		return pluginapi.ExecuteResult{}, err
	}
	sum := sha256.Sum256(raw)
	return pluginapi.ExecuteResult{Status: "passed", ProducedResources: []pluginapi.ResourceDescriptor{{ID: "result", URI: name, MediaType: "application/json", SizeBytes: int64(len(raw)), SHA256: hex.EncodeToString(sum[:]), AccessMode: "read"}}}, nil
}
