package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestServerClientIntegration(t *testing.T) {
    t.Log("Starting test")

    sr, cw := io.Pipe()
    cr, sw := io.Pipe()

    serverTransport := &testTransport{Reader: sr, Writer: sw}
    clientTransport := &testTransport{Reader: cr, Writer: cw}

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
    t.Log("Server configured")

    done := make(chan struct{})
    go func() {
        t.Log("Server starting")
        err := server.Serve(serverTransport)
        t.Logf("Server stopped with err: %v", err)
        close(done)
    }()

    time.Sleep(100 * time.Millisecond)

    client := NewClient(clientTransport)
    t.Log("Client created")

    resultChan := make(chan error)
    go func() {
        t.Log("Sending ListPrompts request")
        prompts, err := client.ListPrompts()
        if err != nil {
            resultChan <- err
            return
        }
        t.Logf("ListPrompts result: %v", prompts)
        if len(prompts) != 1 || prompts["test"].Template != "Test {{value}}" {
            resultChan <- fmt.Errorf("expected prompt 'test', got %v", prompts)
            return
        }

        t.Log("Sending GetResource request")
        rawResp, err := client.GetResource("double", TestParams{Value: 5})
        if err != nil {
            resultChan <- err
            return
        }
        t.Logf("GetResource raw response: %s", rawResp)
        var resp TestResponse
        if err := json.Unmarshal(rawResp, &resp); err != nil {
            resultChan <- err
            return
        }
        t.Logf("GetResource parsed response: %+v", resp)
        if resp.Double != 10 {
            resultChan <- fmt.Errorf("expected Double=10, got %d", resp.Double)
            return
        }

        resultChan <- nil
    }()

    select {
    case err := <-resultChan:
        if err != nil {
            t.Fatal(err)
        }
    case <-time.After(5 * time.Second):
        t.Fatal("Client requests timed out after 5 seconds")
    }

    t.Log("Closing client")
    if err := client.Close(); err != nil {
        t.Fatalf("Close failed: %v", err)
    }
    cw.Close()
    sw.Close()

    t.Log("Waiting for server to stop")
    select {
    case <-done:
        t.Log("Server stopped successfully")
    case <-time.After(5 * time.Second):
        t.Fatal("Test timed out after 5 seconds")
    }
    t.Log("Test completed")
}

type testTransport struct {
    Reader io.Reader
    Writer io.WriteCloser
}

func (t *testTransport) ReadMessage() (json.RawMessage, error) {
    reader := bufio.NewReader(t.Reader)
    line, err := reader.ReadBytes('\n')
    if err != nil {
        return nil, err
    }
    var msg json.RawMessage
    if err := json.Unmarshal(line, &msg); err != nil {
        return nil, err
    }
    return msg, nil
}

func (t *testTransport) WriteMessage(message json.RawMessage) error {
    _, err := t.Writer.Write(append(message, '\n'))
    return err
}

func (t *testTransport) Close() error {
    return t.Writer.Close()
}
