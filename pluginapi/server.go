package pluginapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Handler 由 Go 插件实现。Server 负责分帧、请求取消、并发边界与生命周期分派。
type Handler interface {
	Handshake(context.Context, HandshakeParams) (HandshakeResult, error)
	Health(context.Context) (HealthResult, error)
	Execute(context.Context, ExecuteParams) (ExecuteResult, error)
	Shutdown(context.Context) error
}

type Server struct {
	In              io.Reader
	Out             io.Writer
	Handler         Handler
	MaxPayloadBytes int

	writeMu  sync.Mutex
	activeMu sync.Mutex
	active   map[string]context.CancelFunc
	sem      chan struct{}
}

// Serve 持续运行，直至关闭、EOF、协议违规或上下文结束。
func (s *Server) Serve(ctx context.Context) error {
	if s == nil || s.In == nil || s.Out == nil || s.Handler == nil {
		return fmt.Errorf("plugin server input, output, and handler are required")
	}
	if s.MaxPayloadBytes <= 0 {
		s.MaxPayloadBytes = DefaultMaxFrame
	}
	s.active = make(map[string]context.CancelFunc)
	maxConcurrency := 1
	s.sem = make(chan struct{}, maxConcurrency)
	handshaken := false
	seen := make(map[string]bool)
	var workers sync.WaitGroup
	defer func() {
		s.cancelAll()
		workers.Wait()
	}()

	for {
		var request Request
		if err := ReadFrame(s.In, s.MaxPayloadBytes, &request); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		if request.ProtocolVersion != ProtocolVersion || request.ID == "" || request.Method == "" || request.Deadline.IsZero() {
			return fmt.Errorf("plugin request envelope is invalid")
		}
		if seen[request.ID] {
			return fmt.Errorf("plugin request ID %q was reused", request.ID)
		}
		seen[request.ID] = true
		if request.Method != "handshake" && !handshaken {
			return fmt.Errorf("plugin request %q arrived before handshake", request.Method)
		}
		switch request.Method {
		case "handshake":
			if handshaken {
				return fmt.Errorf("plugin handshake may occur only once")
			}
			var params HandshakeParams
			if err := decodeParams(request.Params, &params); err != nil {
				return err
			}
			callCtx, cancel := requestContext(ctx, request.Deadline)
			result, err := s.Handler.Handshake(callCtx, params)
			cancel()
			if err != nil {
				if responseErr := s.respond(request.ID, nil, handlerError("plugin.handshake_failed", err)); responseErr != nil {
					return responseErr
				}
				return err
			}
			if result.ProtocolVersion != ProtocolVersion || result.MaxConcurrency < 1 || result.MaxConcurrency > 64 {
				return fmt.Errorf("plugin handshake result is incompatible")
			}
			maxConcurrency = result.MaxConcurrency
			s.sem = make(chan struct{}, maxConcurrency)
			if err := s.respond(request.ID, result, nil); err != nil {
				return err
			}
			handshaken = true
		case "health":
			callCtx, cancel := requestContext(ctx, request.Deadline)
			result, err := s.Handler.Health(callCtx)
			cancel()
			if err := s.respond(request.ID, result, handlerError("plugin.health_failed", err)); err != nil {
				return err
			}
		case "execute":
			var params ExecuteParams
			if err := decodeParams(request.Params, &params); err != nil {
				return err
			}
			select {
			case s.sem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			callCtx, cancel := requestContext(ctx, request.Deadline)
			if !s.addActive(request.ID, cancel) {
				cancel()
				<-s.sem
				return fmt.Errorf("duplicate active plugin request ID %q", request.ID)
			}
			workers.Add(1)
			go func(request Request, params ExecuteParams) {
				defer workers.Done()
				defer func() { <-s.sem }()
				defer s.removeActive(request.ID)
				result, err := s.Handler.Execute(callCtx, params)
				_ = s.respond(request.ID, result, handlerError("plugin.execute_failed", err))
			}(request, params)
		case "cancel":
			var params CancelParams
			if err := decodeParams(request.Params, &params); err != nil {
				return err
			}
			accepted := s.cancel(params.RequestID)
			if err := s.respond(request.ID, ShutdownResult{Accepted: accepted}, nil); err != nil {
				return err
			}
		case "shutdown":
			callCtx, cancel := requestContext(ctx, request.Deadline)
			s.cancelAll()
			workers.Wait()
			err := s.Handler.Shutdown(callCtx)
			cancel()
			if responseErr := s.respond(request.ID, ShutdownResult{Accepted: err == nil}, handlerError("plugin.shutdown_failed", err)); responseErr != nil {
				return responseErr
			}
			return err
		default:
			return fmt.Errorf("unknown plugin method %q", request.Method)
		}
	}
}

func (s *Server) respond(id string, result any, rpcErr *RPCError) error {
	response := Response{ProtocolVersion: ProtocolVersion, ID: id, Error: rpcErr}
	if rpcErr == nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode plugin response result: %w", err)
		}
		response.Result = encoded
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return WriteFrame(s.Out, response, s.MaxPayloadBytes)
}

func (s *Server) addActive(id string, cancel context.CancelFunc) bool {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	if _, exists := s.active[id]; exists {
		return false
	}
	s.active[id] = cancel
	return true
}

func (s *Server) removeActive(id string) {
	s.activeMu.Lock()
	cancel := s.active[id]
	delete(s.active, id)
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) cancel(id string) bool {
	s.activeMu.Lock()
	cancel := s.active[id]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
		return true
	}
	return false
}

func (s *Server) cancelAll() {
	s.activeMu.Lock()
	values := make([]context.CancelFunc, 0, len(s.active))
	for _, cancel := range s.active {
		values = append(values, cancel)
	}
	s.activeMu.Unlock()
	for _, cancel := range values {
		cancel()
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode plugin request params: %w", err)
	}
	return nil
}

func requestContext(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}

func handlerError(code string, err error) *RPCError {
	if err == nil {
		return nil
	}
	message := "plugin handler failed"
	if errors.Is(err, context.Canceled) {
		code, message = "plugin.canceled", "plugin request was canceled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		code, message = "plugin.deadline_exceeded", "plugin request deadline was exceeded"
	}
	return &RPCError{Code: code, Message: message}
}
