package pluginapi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

func WriteFrame(writer io.Writer, value any, maxBytes int) error {
	if writer == nil {
		return fmt.Errorf("plugin frame writer is required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxFrame
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode plugin frame: %w", err)
	}
	if len(payload) == 0 || len(payload) > maxBytes {
		return fmt.Errorf("plugin frame payload size %d is outside 1..%d", len(payload), maxBytes)
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if err := writeAll(writer, header[:]); err != nil {
		return fmt.Errorf("write plugin frame header: %w", err)
	}
	if err := writeAll(writer, payload); err != nil {
		return fmt.Errorf("write plugin frame payload: %w", err)
	}
	return nil
}

func ReadFrame(reader io.Reader, maxBytes int, target any) error {
	if reader == nil || target == nil {
		return fmt.Errorf("plugin frame reader and target are required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxFrame
	}
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return fmt.Errorf("read plugin frame header: %w", err)
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size <= 0 || size > maxBytes {
		return fmt.Errorf("plugin frame payload size %d is outside 1..%d", size, maxBytes)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return fmt.Errorf("read plugin frame payload: %w", err)
	}
	if !utf8.Valid(payload) {
		return fmt.Errorf("plugin frame payload is not UTF-8")
	}
	decoder := json.NewDecoder(bufio.NewReaderSize(bytes.NewReader(payload), len(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode plugin frame JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("plugin frame contains trailing JSON")
	}
	return nil
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		n, err := writer.Write(value)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		value = value[n:]
	}
	return nil
}
