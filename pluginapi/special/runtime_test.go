package special

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

type chunkHost struct {
	Host
	read func(ResourceRead) (ResourceChunk, error)
}

func (h chunkHost) ReadResource(_ context.Context, r ResourceRead) (ResourceChunk, error) {
	return h.read(r)
}
func TestReadResourceChecksDigestAndStableReceipt(t *testing.T) {
	data := []byte("abcdef")
	sum := sha256.Sum256(data)
	for _, mode := range []string{"valid", "corrupt", "changed-receipt", "truncated", "no-progress"} {
		t.Run(mode, func(t *testing.T) {
			receipt := ResourceReceipt{ID: "sample", Size: len(data), SHA256: hex.EncodeToString(sum[:])}
			host := chunkHost{read: func(r ResourceRead) (ResourceChunk, error) {
				end := min(int(r.Offset)+3, len(data))
				part := append([]byte(nil), data[r.Offset:end]...)
				c := ResourceChunk{Data: part, NextOffset: int64(end), EOF: end == len(data), Receipt: receipt}
				switch mode {
				case "corrupt":
					c.Data[0] = 'X'
				case "changed-receipt":
					if r.Offset > 0 {
						c.Receipt.ID = "other"
					}
				case "truncated":
					c.EOF = true
				case "no-progress":
					c.Data = nil
					c.NextOffset = r.Offset
				}
				return c, nil
			}}
			var out bytes.Buffer
			err := ReadResourceTo(context.Background(), host, "sample", &out)
			if mode == "valid" {
				if err != nil || !bytes.Equal(out.Bytes(), data) {
					t.Fatalf("valid: %s %v", out.Bytes(), err)
				}
			} else if err == nil {
				t.Fatal("accepted corrupt transfer")
			}
		})
	}
}
