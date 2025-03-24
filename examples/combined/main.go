package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp"
)

func main() {
    // Create pipes for communication
    sr, cw := io.Pipe() // Server reads, Client writes
    cr, sw := io.Pipe() // Client reads, Server writes

    serverTransport := &testTransport{Reader: sr, Writer: sw}
    clientTransport := &testTransport{Reader: cr, Writer: cw}

    // Start the server
    server := mcp.NewServer()
    type EchoParams struct {
        Message string `json:"message"`
    }
    type EchoResponse struct {
        Echo string `json:"echo"`
    }
    server.RegisterResource("echo", mcp.NewResource(func(params EchoParams) (EchoResponse, error) {
        return EchoResponse{Echo: params.Message}, nil
    }))
    server.RegisterPrompt(mcp.Prompt{Name: "greeting", Template: "Hello, {{name}}!"})
    go server.Serve(serverTransport)

    // Start the client
    client := mcp.NewClient(clientTransport)

    // Test ListPrompts
    prompts, err := client.ListPrompts()
    if err != nil {
        log.Fatal("ListPrompts failed:", err)
    }
    fmt.Println("Prompts:", prompts)

    // Test GetResource
    rawResp, err := client.GetResource("echo", EchoParams{Message: "hello world"})
    if err != nil {
        log.Fatal("GetResource failed:", err)
    }
    var resp EchoResponse
    if err := json.Unmarshal(rawResp, &resp); err != nil {
        log.Fatal("Unmarshal failed:", err)
    }
    fmt.Println("Echo Response:", resp.Echo)

    // Clean up
    if err := client.Close(); err != nil {
        log.Fatal("Close failed:", err)
    }
}

// testTransport mimics the transport interface for testing
type testTransport struct {
    Reader io.Reader
    Writer io.Writer
}

func (t *testTransport) ReadMessage() (json.RawMessage, error) {
    var buf bytes.Buffer
    _, err := io.CopyN(&buf, t.Reader, 1024) // Arbitrary limit
    if err != nil && err != io.EOF {
        return nil, err
    }
    data := bytes.TrimSpace(buf.Bytes())
    if len(data) == 0 {
        return nil, io.EOF
    }
    var msg json.RawMessage
    if err := json.Unmarshal(data, &msg); err != nil {
        return nil, err
    }
    return msg, nil
}

func (t *testTransport) WriteMessage(message json.RawMessage) error {
    _, err := t.Writer.Write(append(message, '\n'))
    return err
}

func (t *testTransport) Close() error {
    return nil
}
