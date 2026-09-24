package special

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xieliangji/soluna-plugin-sdk/pluginapi"
)

// Message is independent from the action plugin v1 envelope. Payloads are
// direction-specific; cancellation is handled by the reader, not execution.
type Message struct {
	Version  string          `json:"version"`
	Instance string          `json:"instance"`
	ID       string          `json:"id"`
	Reply    bool            `json:"reply,omitempty"`
	Method   string          `json:"method,omitempty"`
	Deadline time.Time       `json:"deadline,omitzero"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Error    string          `json:"error,omitempty"`
}
type Handler func(context.Context, string, json.RawMessage) (any, error)
type Peer struct {
	stream           io.ReadWriteCloser
	instance, prefix string
	handler          Handler
	done             chan struct{}
	out              chan Message
	slots            chan struct{}
	once             sync.Once
	mu               sync.Mutex
	pending          map[string]chan Message
	active           map[string]context.CancelFunc
	serial           atomic.Uint64
}

func NewPeer(stream io.ReadWriteCloser, instance, prefix string, handler Handler) *Peer {
	p := &Peer{stream: stream, instance: instance, prefix: prefix, handler: handler, done: make(chan struct{}), out: make(chan Message, 32), slots: make(chan struct{}, 8), pending: map[string]chan Message{}, active: map[string]context.CancelFunc{}}
	go p.read()
	go p.write()
	return p
}
func (p *Peer) Done() <-chan struct{} { return p.done }
func (p *Peer) Close() error {
	p.once.Do(func() {
		close(p.done)
		_ = p.stream.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, cancel := range p.active {
			cancel()
		}
	})
	return nil
}
func (p *Peer) send(ctx context.Context, m Message) error {
	m.Version, m.Instance = ProtocolVersion, p.instance
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return io.ErrClosedPipe
	case p.out <- m:
		return nil
	}
}
func (p *Peer) Call(ctx context.Context, method string, input, output any) error {
	if ctx == nil {
		return errors.New("special: context required")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if len(raw) > MaxFrame/2 {
		return errors.New("special: payload too large")
	}
	id := fmt.Sprintf("%s-%d", p.prefix, p.serial.Add(1))
	ch := make(chan Message, 1)
	p.mu.Lock()
	if len(p.pending) >= 32 {
		p.mu.Unlock()
		return errors.New("special: pending limit exceeded")
	}
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.pending, id); p.mu.Unlock() }()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Minute)
	}
	if err = p.send(ctx, Message{ID: id, Method: method, Deadline: deadline, Payload: raw}); err != nil {
		return err
	}
	select {
	case m := <-ch:
		if m.Error != "" {
			return errors.New(m.Error)
		}
		if output == nil {
			return nil
		}
		return Decode(m.Payload, output)
	case <-p.done:
		return io.ErrUnexpectedEOF
	case <-ctx.Done():
		cancelCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if p.send(cancelCtx, Message{ID: id, Method: "cancel"}) != nil {
			_ = p.Close()
		}
		select {
		case <-ch: // Cancellation acknowledged after the handler has returned.
		case <-p.done:
		case <-cancelCtx.Done():
			_ = p.Close()
		}
		return ctx.Err()
	}
}
func (p *Peer) write() {
	for {
		select {
		case <-p.done:
			return
		case m := <-p.out:
			if pluginapi.WriteFrame(p.stream, m, MaxFrame) != nil {
				_ = p.Close()
				return
			}
		}
	}
}
func (p *Peer) read() {
	defer p.Close()
	for {
		var m Message
		if pluginapi.ReadFrame(p.stream, MaxFrame, &m) != nil {
			return
		}
		if m.Version != ProtocolVersion || m.Instance != p.instance || m.ID == "" {
			return
		}
		if m.Reply {
			if m.Method != "" || !m.Deadline.IsZero() || (m.Error == "") == (len(m.Payload) == 0) {
				return
			}
		} else if m.Error != "" || (m.Method == "cancel" && (!m.Deadline.IsZero() || len(m.Payload) != 0)) || (m.Method != "cancel" && len(m.Payload) == 0) {
			return
		}
		p.mu.Lock()
		if m.Reply {
			ch := p.pending[m.ID]
			p.mu.Unlock()
			if ch != nil {
				select {
				case ch <- m:
				default:
					return
				}
			}
			continue
		}
		if m.Method == "cancel" {
			if c := p.active[m.ID]; c != nil {
				c()
			}
			p.mu.Unlock()
			continue
		}
		if _, exists := p.active[m.ID]; exists {
			p.mu.Unlock()
			return
		}
		if m.Method == "" || m.Deadline.IsZero() {
			p.mu.Unlock()
			return
		}
		select {
		case p.slots <- struct{}{}:
		default:
			p.mu.Unlock()
			return
		}
		ctx, cancel := context.WithDeadline(context.Background(), m.Deadline)
		p.active[m.ID] = cancel
		p.mu.Unlock()
		go p.handle(ctx, cancel, m)
	}
}
func (p *Peer) handle(ctx context.Context, cancel context.CancelFunc, m Message) {
	defer func() { cancel(); p.mu.Lock(); delete(p.active, m.ID); p.mu.Unlock(); <-p.slots }()
	var value any
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("special: handler panic: %v", recovered)
			}
		}()
		if p.handler == nil {
			err = errors.New("special: unsupported method")
		} else {
			value, err = p.handler(ctx, m.Method, m.Payload)
		}
	}()
	response := Message{ID: m.ID, Reply: true}
	if err != nil {
		response.Error = err.Error()
	} else {
		response.Payload, err = json.Marshal(value)
		if err != nil {
			response.Error = err.Error()
		}
	}
	replyCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if p.send(replyCtx, response) != nil {
		_ = p.Close()
	}
}
