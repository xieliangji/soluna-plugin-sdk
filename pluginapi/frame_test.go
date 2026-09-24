package pluginapi

import (
	"bytes"
	"strings"
	"testing"
)

func TestFrameRoundTripWithPartialWrites(t *testing.T) {
	t.Parallel()
	writer := &shortWriter{}
	value := Request{ProtocolVersion: ProtocolVersion, ID: "request-1", Method: "health"}
	if err := WriteFrame(writer, value, 1024); err != nil {
		t.Fatal(err)
	}
	var decoded Request
	if err := ReadFrame(bytes.NewReader(writer.Bytes()), 1024, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != value.ID || decoded.Method != value.Method {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestReadFrameRejectsInvalidLengthsAndJSON(t *testing.T) {
	t.Parallel()
	tests := map[string][]byte{
		"zero":      {0, 0, 0, 0},
		"oversize":  {0, 0, 4, 1},
		"truncated": append([]byte{0, 0, 0, 4}, []byte("{}")...),
		"json":      append([]byte{0, 0, 0, 1}, '{'),
		"utf8":      append([]byte{0, 0, 0, 1}, 0xff),
	}
	for name, data := range tests {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var target map[string]any
			if err := ReadFrame(bytes.NewReader(data), 1024, &target); err == nil {
				t.Fatal("invalid frame unexpectedly passed")
			}
		})
	}
}

func TestWriteFrameRejectsOversizedPayload(t *testing.T) {
	t.Parallel()
	if err := WriteFrame(&bytes.Buffer{}, strings.Repeat("x", 100), 10); err == nil {
		t.Fatal("oversized payload unexpectedly passed")
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(value []byte) (int, error) {
	if len(value) > 2 {
		value = value[:2]
	}
	return w.Buffer.Write(value)
}
