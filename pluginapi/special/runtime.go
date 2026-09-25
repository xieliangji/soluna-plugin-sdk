package special

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// V1 remains pre-stable. Revision is an exact compatibility gate, not a new protocol version.
const ContractRevision = "v1-draft-20260925"
const ChunkBytes = 256 << 10
const MaxResourceBytes = 256 << 20

type WaitPolicy struct {
	TimeoutMs  int64 `json:"timeoutMs"`
	IntervalMs int64 `json:"intervalMs,omitempty"`
}
type Callback struct {
	Epoch  string          `json:"epoch"`
	Params json.RawMessage `json:"params"`
}
type Health struct {
	Ready    bool   `json:"ready"`
	Revision string `json:"revision"`
	Message  string `json:"message,omitempty"`
}
type HealthProvider interface {
	Health(context.Context) (Health, error)
}
type SnapshotProvider interface {
	Snapshot(context.Context) (Result, error)
}

// ResourceReporter receives immutable inputs; its Host rejects device operations.
type ResourceReporter interface {
	PrepareResourceReport(context.Context, ReportRequest, Host) (Report, error)
}
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type ObservationRequest struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Role string `json:"role,omitempty"`
}
type Observation struct {
	Source   *ResourceReceipt `json:"source,omitempty"`
	Element  *Rect            `json:"element,omitempty"`
	Viewport *Rect            `json:"viewport,omitempty"`
}
type ResourceRead struct {
	ID     string `json:"id"`
	Offset int64  `json:"offset"`
	Limit  int    `json:"limit"`
}
type ResourceChunk struct {
	Data       []byte          `json:"data"`
	NextOffset int64           `json:"nextOffset"`
	EOF        bool            `json:"eof"`
	Receipt    ResourceReceipt `json:"receipt"`
}

// ReadResourceTo streams a host-registered immutable resource, never a host path.
func ReadResourceTo(ctx context.Context, h Host, id string, w io.Writer) error {
	var off int64
	var receipt ResourceReceipt
	hash := sha256.New()
	for {
		r, e := h.ReadResource(ctx, ResourceRead{ID: id, Offset: off, Limit: ChunkBytes})
		if e != nil {
			return e
		}
		if r.NextOffset != off+int64(len(r.Data)) || (!r.EOF && len(r.Data) == 0) || r.NextOffset > MaxResourceBytes {
			return fmt.Errorf("special: invalid resource cursor")
		}
		if off == 0 {
			receipt = r.Receipt
		}
		if receipt.ID != id || r.Receipt != receipt || receipt.Size < 0 || receipt.Size > MaxResourceBytes || len(receipt.SHA256) != 64 || r.NextOffset > int64(receipt.Size) {
			return fmt.Errorf("special: invalid resource receipt")
		}
		if _, e = io.MultiWriter(w, hash).Write(r.Data); e != nil {
			return e
		}
		off = r.NextOffset
		if r.EOF {
			if off != int64(receipt.Size) || hex.EncodeToString(hash.Sum(nil)) != receipt.SHA256 {
				return fmt.Errorf("special: resource checksum mismatch")
			}
			return nil
		}
	}
}

// PutResource writes bounded chunks and returns only a committed receipt.
func PutResource(ctx context.Context, h Host, id, name, contentType string, data []byte) (ResourceReceipt, error) {
	if len(data) > MaxResourceBytes {
		return ResourceReceipt{}, fmt.Errorf("special: resource too large")
	}
	for off := 0; ; {
		end := min(off+ChunkBytes, len(data))
		r, e := h.Resource(ctx, Resource{ID: id, Name: name, ContentType: contentType, Offset: int64(off), Total: int64(len(data)), Commit: end == len(data), Data: data[off:end]})
		if e != nil {
			return r, e
		}
		if end == len(data) {
			sum := sha256.Sum256(data)
			if r.ID != id || r.Size != len(data) || r.SHA256 != hex.EncodeToString(sum[:]) {
				return ResourceReceipt{}, fmt.Errorf("special: committed receipt mismatch")
			}
			return r, nil
		}
		off = end
	}
}
