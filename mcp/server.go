package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
)

// Server handles MCP server-side logic, processing requests and sending responses/notifications.
type Server struct {
	handlers         map[string]HandlerFunc
	prompts          map[string]Prompt
	resourceHandlers map[string]Handler
	toolHandlers     map[string]Handler
	onStart          func() error
	onStop           func() error
	transport        transports.Transport
	mu               sync.Mutex
}

// Handler defines the interface for handling JSON-RPC requests.
type Handler interface {
	ServeJSONRPC(ctx context.Context, params json.RawMessage) (interface{}, error)
}

// HandlerFunc is a function type for handling JSON-RPC requests.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (interface{}, error)

// ServeJSONRPC implements the Handler interface for HandlerFunc.
func (f HandlerFunc) ServeJSONRPC(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return f(ctx, params)
}

// Prompt represents an MCP prompt template.
type Prompt struct {
	Name     string
	Template string
}

// Resource defines a generic resource handler.
type Resource[Req, Resp any] struct {
	Handler func(Req) (Resp, error)
}

// ServeJSONRPC implements the Handler interface for Resource.
func (r Resource[Req, Resp]) ServeJSONRPC(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Name   string          `json:"name"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	var p Req
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
	}
	return r.Handler(p)
}

// Tool defines a generic tool handler.
type Tool[Req, Resp any] struct {
	Execute func(Req) (Resp, error)
}

// ServeJSONRPC implements the Handler interface for Tool.
func (t Tool[Req, Resp]) ServeJSONRPC(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Name   string          `json:"name"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}
	var p Req
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, err
	}
	return t.Execute(p)
}

// NewResource creates a new resource handler.
func NewResource[Req, Resp any](handler func(Req) (Resp, error)) Handler {
	return Resource[Req, Resp]{Handler: handler}
}

// NewTool creates a new tool handler.
func NewTool[Req, Resp any](execute func(Req) (Resp, error)) Handler {
	return Tool[Req, Resp]{Execute: execute}
}

// NewServer creates a new MCP server instance.
func NewServer() *Server {
	s := &Server{
		handlers:         make(map[string]HandlerFunc),
		prompts:          make(map[string]Prompt),
		resourceHandlers: make(map[string]Handler),
		toolHandlers:     make(map[string]Handler),
	}
	s.handlers["listPrompts"] = s.listPromptsHandler()
	s.handlers["getPrompt"] = s.getPromptHandler()
	s.handlers["getResource"] = s.getResourceHandler()
	s.handlers["executeTool"] = s.executeToolHandler()
	return s
}

// RegisterHandler registers a handler for a method.
func (s *Server) RegisterHandler(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// RegisterResource registers a resource with a specific name.
func (s *Server) RegisterResource(name string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceHandlers[name] = handler
}

// RegisterTool registers a tool with a specific name.
func (s *Server) RegisterTool(name string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolHandlers[name] = handler
}

// RegisterPrompt registers a prompt template.
func (s *Server) RegisterPrompt(prompt Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

// SetOnStart sets the startup hook.
func (s *Server) SetOnStart(handler func() error) {
	s.onStart = handler
}

// SetOnStop sets the shutdown hook.
func (s *Server) SetOnStop(handler func() error) {
	s.onStop = handler
}

// SendNotification sends a notification to the client.
func (s *Server) SendNotification(method string, params interface{}) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	n := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  p,
	}
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return s.transport.WriteMessage(data)
}

// Serve starts the server with the given transport.
func (s *Server) Serve(transport transports.Transport) error {
	s.transport = transport
	// if s.onStart != nil {
	// 	if err := s.onStart(); err != nil {
	// 		return err
	// 	}
	// }
	// defer func() {
	// 	if s.onStop != nil {
	// 		s.onStop()
	// 	}
	// }()
	for {
		msg, err := transport.ReadMessage()
		if err == io.EOF {
			fmt.Println("Server received EOF, stopping")
			return nil
		}
		if err != nil {
			fmt.Println("Server read error:", err)
			return err
		}
		fmt.Println("Server received message:", string(msg))
		go s.handleMessage(msg)
	}
}

func (s *Server) handleMessage(msg json.RawMessage) {
	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		fmt.Println("Server unmarshal error:", err)
		s.sendError(nil, -32700, "Parse error", nil)
		return
	}
	fmt.Println("Server processing request:", req.Method, "ID:", req.ID)
	if req.JSONRPC != "2.0" {
		s.sendError(req.ID, -32600, "Invalid Request", nil)
		return
	}
	handler, ok := s.handlers[req.Method]
	if !ok {
		s.sendError(req.ID, -32601, "Method not found", nil)
		return
	}
	ctx := context.Background()
	result, err := handler(ctx, req.Params)
	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	if err != nil {
		resp.Error = &RPCError{
			Code:    -32000,
			Message: err.Error(),
		}
	} else if result != nil {
		resp.Result, _ = json.Marshal(result)
	}
	// data, _ := json.Marshal(resp)
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Println("Error marchaling response:", err)
		return
	}
	s.transport.WriteMessage(data)
}

func (s *Server) sendError(id interface{}, code int, message string, errorData interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    errorData,
		},
	}
	data, err := json.Marshal(resp) // Use = instead of := if data is already declared, or handle properly
	if err != nil {
		fmt.Println("Error marshaling error response:", err)
		return
	}
	s.transport.WriteMessage(data)
}

// listPromptsHandler returns a handler for the "listPrompts" method.
func (s *Server) listPromptsHandler() HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.prompts, nil
	}
}

// getPromptHandler returns a handler for the "getPrompt" method.
func (s *Server) getPromptHandler() HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		prompt, ok := s.prompts[p.Name]
		if !ok {
			return nil, fmt.Errorf("prompt not found: %s", p.Name)
		}
		return prompt, nil
	}
}

// getResourceHandler returns a handler for the "getResource" method.
func (s *Server) getResourceHandler() HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		handler, ok := s.resourceHandlers[p.Name]
		s.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("resource not found: %s", p.Name)
		}
		return handler.ServeJSONRPC(ctx, params)
	}
}

// executeToolHandler returns a handler for the "executeTool" method.
func (s *Server) executeToolHandler() HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (interface{}, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		handler, ok := s.toolHandlers[p.Name]
		s.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("tool not found: %s", p.Name)
		}
		return handler.ServeJSONRPC(ctx, params)
	}
}
