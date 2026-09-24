package special

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
)

func TestDuplexCallsAndCancellation(t *testing.T) {
	left, right := net.Pipe()
	var calls atomic.Int32
	host := NewPeer(left, "test", "host", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		calls.Add(1)
		return map[string]string{"status": "passed"}, nil
	})
	defer host.Close()
	var plugin *Peer
	ready := make(chan struct{})
	cancelled := make(chan struct{})
	plugin = NewPeer(right, "test", "plugin", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		<-ready
		if method == "wait" {
			<-ctx.Done()
			close(cancelled)
			return nil, ctx.Err()
		}
		var result map[string]string
		err := plugin.Call(ctx, "host.call", struct{}{}, &result)
		return result, err
	})
	close(ready)
	defer plugin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out map[string]string
	if err := host.Call(ctx, "run", struct{}{}, &out); err != nil || out["status"] != "passed" || calls.Load() != 1 {
		t.Fatalf("%v %+v", err, out)
	}
	short, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()
	if err := host.Call(short, "wait", struct{}{}, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked by run")
	}
	if err := host.Call(ctx, "run", struct{}{}, &out); err != nil {
		t.Fatal("peer unavailable after acknowledged cancellation", err)
	}
}

func TestPeerRejectsAmbiguousEnvelope(t *testing.T) {
	for _, message := range []Message{
		{Reply: true, Method: "run", Payload: json.RawMessage(`null`)},
		{Reply: true, Payload: json.RawMessage(`null`), Error: "failed"},
		{Method: "cancel", Payload: json.RawMessage(`{}`)},
		{Method: "run", Deadline: time.Now().Add(time.Second), Error: "failed"},
	} {
		left, right := net.Pipe()
		peer := NewPeer(left, "test", "host", nil)
		message.Version, message.Instance, message.ID = ProtocolVersion, "test", "plugin-1"
		if err := pluginapi.WriteFrame(right, message, MaxFrame); err != nil {
			t.Fatal(err)
		}
		select {
		case <-peer.Done():
		case <-time.After(time.Second):
			t.Fatal("ambiguous envelope accepted")
		}
		_ = right.Close()
	}
}
func TestPeerRejectsWrongInstanceAndUnblocksPending(t *testing.T) {
	left, right := net.Pipe()
	host := NewPeer(left, "one", "host", nil)
	defer host.Close()
	plugin := NewPeer(right, "two", "plugin", nil)
	defer plugin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := host.Call(ctx, "describe", struct{}{}, nil); err == nil {
		t.Fatal("accepted wrong instance")
	}
}
func TestPeerCloseUnblocksStuckWriter(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	p := NewPeer(left, "one", "host", nil)
	done := make(chan error, 1)
	go func() { done <- p.Call(context.Background(), "run", struct{}{}, nil) }()
	_ = p.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("missing error")
		}
	case <-time.After(time.Second):
		t.Fatal("call stuck")
	}
}
