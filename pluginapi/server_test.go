package pluginapi

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestServerNegotiatesConcurrencyAndReturnsOutOfOrderResponses(t *testing.T) {
	t.Parallel()
	serverInput, clientOutput := io.Pipe()
	clientInput, serverOutput := io.Pipe()
	server := &Server{In: serverInput, Out: serverOutput, Handler: concurrentHandler{}}
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()
	deadline := time.Now().Add(5 * time.Second)
	writeRequest := func(id, method string, params any) {
		t.Helper()
		encoded, _ := json.Marshal(params)
		if err := WriteFrame(clientOutput, Request{ProtocolVersion: ProtocolVersion, ID: id, Method: method, Deadline: deadline, Params: encoded}, DefaultMaxFrame); err != nil {
			t.Fatal(err)
		}
	}
	readResponse := func() Response {
		t.Helper()
		var response Response
		if err := ReadFrame(clientInput, DefaultMaxFrame, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	writeRequest("handshake", "handshake", HandshakeParams{ProtocolVersions: []string{ProtocolVersion}})
	if response := readResponse(); response.ID != "handshake" || response.Error != nil {
		t.Fatalf("handshake response = %+v", response)
	}
	writeRequest("slow", "execute", ExecuteParams{Capability: "test", Arguments: json.RawMessage(`{"delayMs":100}`)})
	writeRequest("fast", "execute", ExecuteParams{Capability: "test", Arguments: json.RawMessage(`{"delayMs":1}`)})
	if first, second := readResponse(), readResponse(); first.ID != "fast" || second.ID != "slow" {
		t.Fatalf("response order = %s, %s", first.ID, second.ID)
	}
	writeRequest("shutdown", "shutdown", ShutdownParams{})
	_ = readResponse()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type concurrentHandler struct{}

func (concurrentHandler) Handshake(context.Context, HandshakeParams) (HandshakeResult, error) {
	return HandshakeResult{PluginID: "test", PluginName: "Test", PluginVersion: "0.1.0", ProtocolVersion: ProtocolVersion, MaxConcurrency: 2, ResourceSchemes: []string{"file"}}, nil
}

func (concurrentHandler) Health(context.Context) (HealthResult, error) {
	return HealthResult{Ready: true}, nil
}

func (concurrentHandler) Execute(ctx context.Context, params ExecuteParams) (ExecuteResult, error) {
	var arguments struct {
		DelayMS int `json:"delayMs"`
	}
	_ = json.Unmarshal(params.Arguments, &arguments)
	select {
	case <-time.After(time.Duration(arguments.DelayMS) * time.Millisecond):
		return ExecuteResult{Status: "passed"}, nil
	case <-ctx.Done():
		return ExecuteResult{}, ctx.Err()
	}
}

func (concurrentHandler) Shutdown(context.Context) error { return nil }
