package mcp

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
)

// Client interacts with an MCP server, sending requests and handling responses/notifications.
type Client struct {
	transport            transports.Transport
	notificationHandlers map[string]NotificationHandler
	pendingRequests      map[interface{}]chan responseChan
	mu                   sync.Mutex
	nextID               int
	stop                 chan struct{}
	wg                   sync.WaitGroup
}

// NotificationHandler handles incoming notifications.
type NotificationHandler func(method string, params json.RawMessage) error

type responseChan struct {
	result json.RawMessage
	err    *RPCError
}

// NewClient creates a new MCP client instance with the given transport.
func NewClient(transport transports.Transport) *Client {
	c := &Client{
		transport:            transport,
		notificationHandlers: make(map[string]NotificationHandler),
		pendingRequests:      make(map[interface{}]chan responseChan),
		nextID:               1,
		stop:                 make(chan struct{}),
	}
	c.wg.Add(1)
	go c.readLoop()
	return c
}

// readLoop processes incoming messages from the transport.
func (c *Client) readLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stop:
			return
		default:
			msg, err := c.transport.ReadMessage()
			if err != nil {
				return
			}
			c.handleMessage(msg)
		}
	}
}

// handleMessage processes incoming messages.
func (c *Client) handleMessage(msg json.RawMessage) {
	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		return
	}
	if id, ok := m["id"]; ok {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			return
		}
		c.mu.Lock()
		ch, exists := c.pendingRequests[id]
		if exists {
			delete(c.pendingRequests, id)
		}
		c.mu.Unlock()
		if exists {
			ch <- responseChan{result: resp.Result, err: resp.Error}
			close(ch)
		}
	} else if method, ok := m["method"].(string); ok {
		c.mu.Lock()
		handler, exists := c.notificationHandlers[method]
		c.mu.Unlock()
		if exists {
			var n Notification
			json.Unmarshal(msg, &n)
			handler(method, n.Params)
		}
	}
}

// CallRaw performs a JSON-RPC call and returns the raw result.
func (c *Client) CallRaw(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
	}
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		req.Params = p
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := c.transport.WriteMessage(data); err != nil {
		return nil, err
	}

	ch := make(chan responseChan, 1)
	c.mu.Lock()
	c.pendingRequests[id] = ch
	c.mu.Unlock()

	resp := <-ch
	if resp.err != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.err.Message)
	}
	return resp.result, nil
}

// Call provides a type-safe wrapper around CallRaw.
func Call[Resp any](c *Client, method string, params interface{}) (Resp, error) {
	raw, err := c.CallRaw(method, params)
	if err != nil {
		return *new(Resp), err
	}
	var resp Resp
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &resp); err != nil {
			return *new(Resp), err
		}
	}
	return resp, nil
}

// RegisterNotificationHandler registers a handler for a specific notification method.
func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notificationHandlers[method] = handler
}

// Close shuts down the client.
func (c *Client) Close() error {
	close(c.stop)
	c.wg.Wait()
	return c.transport.Close()
}

// ListPrompts calls the "listPrompts" method with type safety.
func (c *Client) ListPrompts() (map[string]Prompt, error) {
	return Call[map[string]Prompt](c, "listPrompts", nil)
}

// GetPrompt calls the "getPrompt" method with type safety.
func (c *Client) GetPrompt(name string) (Prompt, error) {
	params := map[string]string{"name": name}
	return Call[Prompt](c, "getPrompt", params)
}

// GetResource calls the "getResource" method and returns the raw result.
func (c *Client) GetResource(name string, params interface{}) (json.RawMessage, error) {
	req := struct {
		Name   string      `json:"name"`
		Params interface{} `json:"params,omitempty"`
	}{
		Name:   name,
		Params: params,
	}
	return c.CallRaw("getResource", req)
}

// ExecuteTool calls the "executeTool" method and returns the raw result.
func (c *Client) ExecuteTool(name string, params interface{}) (json.RawMessage, error) {
	req := struct {
		Name   string      `json:"name"`
		Params interface{} `json:"params,omitempty"`
	}{
		Name:   name,
		Params: params,
	}
	return c.CallRaw("executeTool", req)
}

// GetResourceTyped provides a type-safe wrapper for GetResource.
func GetResource[Resp any](c *Client, name string, params interface{}) (Resp, error) {
	raw, err := c.GetResource(name, params)
	if err != nil {
		return *new(Resp), err
	}
	var resp Resp
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &resp); err != nil {
			return *new(Resp), err
		}
	}
	return resp, nil
}

// ExecuteToolTyped provides a type-safe wrapper for ExecuteTool.
func ExecuteTool[Resp any](c *Client, name string, params interface{}) (Resp, error) {
	raw, err := c.ExecuteTool(name, params)
	if err != nil {
		return *new(Resp), err
	}
	var resp Resp
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &resp); err != nil {
			return *new(Resp), err
		}
	}
	return resp, nil
}
