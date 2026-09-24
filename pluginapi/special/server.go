package special

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Decode rejects extra fields and multiple JSON values at every SDK boundary.
func Decode(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("special: trailing JSON")
	}
	return nil
}

type remoteHost struct{ peer *Peer }

func (h remoteHost) Call(ctx context.Context, v Call) (r Feedback, err error) {
	err = h.peer.Call(ctx, "host.call", v, &r)
	return
}
func (h remoteHost) Event(ctx context.Context, v Event) error {
	return h.peer.Call(ctx, "host.event", v, nil)
}
func (h remoteHost) Variable(ctx context.Context, v VariableRequest) (r Variable, err error) {
	err = h.peer.Call(ctx, "host.variable", v, &r)
	return
}
func (h remoteHost) Resource(ctx context.Context, v Resource) (r ResourceReceipt, err error) {
	err = h.peer.Call(ctx, "host.resource", v, &r)
	return
}

// Serve serves one provider instance. Cancellation is independent of the single
// lifecycle slot; cleanup is rejected while a provider method is still active.
func Serve(ctx context.Context, stream io.ReadWriteCloser, instance string, provider Provider) error {
	if provider == nil || instance == "" {
		return fmt.Errorf("special: provider and instance required")
	}
	var gate sync.Mutex
	state := "new"
	var frozenRun []byte
	var peer *Peer
	handler := func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		if !gate.TryLock() {
			return nil, fmt.Errorf("special: lifecycle busy")
		}
		defer gate.Unlock()
		host := remoteHost{peer}
		switch method {
		case "describe":
			return provider.Describe(), nil
		case "template":
			var r TemplateRequest
			if err := Decode(raw, &r); err != nil {
				return nil, err
			}
			return provider.Template(ctx, r)
		case "compile":
			var r CompileRequest
			if err := Decode(raw, &r); err != nil {
				return nil, err
			}
			return provider.Compile(ctx, r)
		case "prepare", "run", "cleanup":
			var r RunRequest
			if err := Decode(raw, &r); err != nil {
				return nil, err
			}
			if r.RunID == "" {
				return nil, fmt.Errorf("special: run ID required")
			}
			encodedRun, _ := json.Marshal(r)
			if method != "prepare" && len(frozenRun) > 0 && !bytes.Equal(frozenRun, encodedRun) {
				return nil, fmt.Errorf("special: run identity or compiled input changed")
			}
			switch method {
			case "prepare":
				if state != "new" {
					return nil, fmt.Errorf("special: already prepared")
				}
				frozenRun = encodedRun
				state = "preparing"
				err := provider.Prepare(ctx, r, host)
				state = "prepared"
				return nil, err
			case "run":
				if state != "prepared" {
					return nil, fmt.Errorf("special: run requires prepare")
				}
				state = "running"
				result, err := provider.Run(ctx, r, host)
				state = "finished"
				return result, err
			default:
				if state == "cleaned" {
					return nil, nil
				}
				state = "cleaning"
				err := provider.Cleanup(ctx, r, host)
				if err == nil {
					state = "cleaned"
				}
				return nil, err
			}
		case "report.prepare":
			var r ReportRequest
			if err := Decode(raw, &r); err != nil {
				return nil, err
			}
			return provider.PrepareReport(ctx, r)
		case "report.render":
			var r RenderRequest
			if err := Decode(raw, &r); err != nil {
				return nil, err
			}
			return provider.RenderReport(ctx, r)
		default:
			return nil, fmt.Errorf("special: unknown method %s", method)
		}
	}
	// Ready barrier prevents a fast transport from entering before peer assignment.
	ready := make(chan struct{})
	peer = NewPeer(stream, instance, "plugin", func(c context.Context, m string, r json.RawMessage) (any, error) { <-ready; return handler(c, m, r) })
	close(ready)
	defer peer.Close()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-peer.Done():
		return nil
	}
}

// Stdio wraps process pipes without allowing protocol data on diagnostic stderr.
type Stdio struct {
	Reader io.ReadCloser
	Writer io.WriteCloser
}

func (s Stdio) Read(b []byte) (int, error)  { return s.Reader.Read(b) }
func (s Stdio) Write(b []byte) (int, error) { return s.Writer.Write(b) }
func (s Stdio) Close() error                { _ = s.Reader.Close(); return s.Writer.Close() }
