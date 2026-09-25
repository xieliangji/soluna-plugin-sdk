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

type remoteHost struct {
	peer  *Peer
	epoch string
	ctx   context.Context
}

func (h remoteHost) invoke(ctx context.Context, m string, v, out any) error {
	if h.ctx != nil {
		if e := h.ctx.Err(); e != nil {
			return e
		}
	}
	if h.epoch != "" {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		return h.peer.Call(ctx, m, Callback{Epoch: h.epoch, Params: b}, out)
	}
	return h.peer.Call(ctx, m, v, out)
}
func (h remoteHost) ReadResource(ctx context.Context, v ResourceRead) (r ResourceChunk, e error) {
	e = h.invoke(ctx, "host.resource.read", v, &r)
	return
}
func (h remoteHost) Observe(ctx context.Context, v ObservationRequest) (r Observation, e error) {
	e = h.invoke(ctx, "host.observe", v, &r)
	return
}

func (h remoteHost) Call(ctx context.Context, v Call) (r Feedback, err error) {
	err = h.invoke(ctx, "host.call", v, &r)
	return
}
func (h remoteHost) Event(ctx context.Context, v Event) error {
	return h.invoke(ctx, "host.event", v, nil)
}
func (h remoteHost) Variable(ctx context.Context, v VariableRequest) (r Variable, err error) {
	err = h.invoke(ctx, "host.variable", v, &r)
	return
}
func (h remoteHost) Resource(ctx context.Context, v Resource) (r ResourceReceipt, err error) {
	err = h.invoke(ctx, "host.resource", v, &r)
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
	lastUnit := 0
	var frozenRun []byte
	var peer *Peer
	handler := func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		if !gate.TryLock() {
			return nil, fmt.Errorf("special: lifecycle busy")
		}
		defer gate.Unlock()
		phaseCtx, revoke := context.WithCancel(ctx)
		defer revoke()
		host := remoteHost{peer: peer, ctx: phaseCtx}
		switch method {
		case "health":
			if h, ok := provider.(HealthProvider); ok {
				return h.Health(ctx)
			}
			return Health{Ready: true, Revision: ContractRevision}, nil
		case "snapshot":
			if state != "finished" && state != "cleaned" {
				return nil, fmt.Errorf("special: snapshot unavailable")
			}
			if p, ok := provider.(SnapshotProvider); ok {
				return p.Snapshot(ctx)
			}
			return nil, fmt.Errorf("special: snapshot unsupported")
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
			host.epoch = r.Epoch
			identity := r
			identity.Epoch = ""
			identity.Unit = 0
			encodedRun, _ := json.Marshal(identity)
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
				if err != nil {
					state = "prepare_failed"
				} else {
					state = "prepared"
				}
				return nil, err
			case "run":
				if state != "prepared" && state != "finished" {
					return nil, fmt.Errorf("special: run requires prepare")
				}
				unit := r.Unit
				if unit == 0 && r.Compiled.Units == 0 {
					unit = 1
				}
				if unit != lastUnit+1 || unit > max(r.Compiled.Units, 1) {
					return nil, fmt.Errorf("special: unit identity already used or out of order")
				}
				lastUnit = unit
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
			if p, ok := provider.(ResourceReporter); ok {
				return p.PrepareResourceReport(ctx, r, host)
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
