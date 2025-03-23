package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestServerClientIntegration(t *testing.T) {
    // Create pipes for bidirectional communication
    sr, cw := io.Pipe() // Server reads, Client writes
    cr, sw := io.Pipe() // Client reads, Server writes

    serverTransport := &testTransport{Reader: sr, Writer: sw}
    clientTransport := &testTransport{Reader: cr, Writer: cw}

    // Set up server
    server := NewServer()
    type TestParams struct {
        Value int `json:"value"`
    }
    type TestResponse struct {
        Double int `json:"double"`
    }
    server.RegisterResource("double", NewResource(func(params TestParams) (TestResponse, error) {
        return TestResponse{Double: params.Value * 2}, nil
    }))
    server.RegisterPrompt(Prompt{Name: "test", Template: "Test {{value}}"})

    go server.Serve(serverTransport)

    // Set up client
    client := NewClient(clientTransport)

    // Test ListPrompts
    prompts, err := client.ListPrompts()
    if err != nil {
        t.Fatal(err)
    }
    if len(prompts) != 1 || prompts["test"].Template != "Test {{value}}" {
        t.Errorf("expected prompt 'test', got %v", prompts)
    }

    // Test GetResource
    rawResp, err := client.GetResource("double", TestParams{Value: 5})
    if err != nil {
        t.Fatal(err)
    }
    var resp TestResponse
    if err := json.Unmarshal(rawResp, &resp); err != nil {
        t.Fatal(err)
    }
    if resp.Double != 10 {
        t.Errorf("expected Double=10, got %d", resp.Double)
    }

    // Clean up
    client.Close()
}

// testTransport is a mock transport for testing.
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
