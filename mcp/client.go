package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
)

// Client interacts with an MCP server, sending requests and handling responses/notifications.
type Client struct {
	transport            transports.Transport
	notificationHandlers map[string]NotificationHandler
	pendingRequests      map[int]chan responseChan // Changed from interface{}
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
		pendingRequests:      make(map[int]chan responseChan),
		stop:                 make(chan struct{}),
	}
	c.wg.Add(1)
	go c.readLoop()
	return c
}

// // readLoop processes incoming messages from the transport.
// func (c *Client) readLoop() {
// 	defer c.wg.Done()
// 	for {
// 		select {
// 		case <-c.stop:
// 			return
// 		default:
// 			msg, err := c.transport.ReadMessage()
// 			if err != nil {
// 				return
// 			}
// 			c.handleMessage(msg)
// 		}
// 	}
// }

// handleMessage processes incoming messages.
func (c *Client) handleMessage(msg json.RawMessage) {
	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		fmt.Println("Client handleMessage unmarshal error:", err)
		return
	}
	if id, ok := m["id"]; ok {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			fmt.Println("Client handleMessage response unmarshal error:", err)
			return
		}
		idInt := int(id.(float64))
		c.mu.Lock()
		ch, exists := c.pendingRequests[idInt]
		if exists {
			delete(c.pendingRequests, idInt)
		}
		c.mu.Unlock()
		fmt.Println("Client handleMessage ID:", idInt, "exists:", exists)
		if exists {
			fmt.Println("Client sending response to channel:", string(resp.Result))
			ch <- responseChan{result: resp.Result, err: resp.Error}
			close(ch)
		}
	}
}

// CallRaw performs a JSON-RPC call and returns the raw result.
func (c *Client) CallRaw(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan responseChan, 1)
	c.pendingRequests[id] = ch
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
	}
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			c.mu.Lock()
			delete(c.pendingRequests, id)
			c.mu.Unlock()
			return nil, err
		}
		req.Params = p
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, id)
		c.mu.Unlock()
		return nil, err
	}
	fmt.Println("Client sending request:", string(data))
	if err := c.transport.WriteMessage(data); err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, id)
		c.mu.Unlock()
		return nil, err
	}

	fmt.Println("Client waiting for response on ID:", id)
	resp := <-ch
	fmt.Println("Client received response:", resp.result, "err:", resp.err)
	if resp.err != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.err.Message)
	}
	return resp.result, nil
}

func (c *Client) readLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stop:
			fmt.Println("Client readLoop stopped")
			return
		default:
			msg, err := c.transport.ReadMessage()
			if err == io.EOF {
				fmt.Println("Client readLoop received EOF, stopping")
				return
			}
			if err != nil {
				fmt.Println("Client readLoop error:", err)
				return
			}
			fmt.Println("Client readLoop received:", string(msg))
			c.handleMessage(msg)
		}
	}
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
	c.mu.Lock()
	close(c.stop)
	c.mu.Unlock()
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

// type Request struct {
// 	JSONRPC string          `json:"jsonrpc"`
// 	Method  string          `json:"method"`
// 	Params  json.RawMessage `json:"params,omitempty"`
// 	ID      int             `json:"id"`
// }

// type Response struct {
// 	JSONRPC string          `json:"jsonrpc"`
// 	Result  json.RawMessage `json:"result,omitempty"`
// 	Error   *Error          `json:"error,omitempty"`
// 	ID      int             `json:"id"`
// }

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// type responseChan struct {
// 	result json.RawMessage
// 	err    *Error
// }

// type NotificationHandler func(method string, params json.RawMessage)
