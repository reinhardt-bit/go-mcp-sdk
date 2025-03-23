package transports

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// Transport defines the interface for MCP communication.
type Transport interface {
	ReadMessage() (json.RawMessage, error)
	WriteMessage(message json.RawMessage) error
	Close() error
}

// StdioTransport implements the Transport interface using stdin and stdout.
type StdioTransport struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
}

// NewStdioTransport creates a new stdio transport instance.
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

// ReadMessage reads a JSON-RPC message from stdin.
func (t *StdioTransport) ReadMessage() (json.RawMessage, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var msg json.RawMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// WriteMessage writes a JSON-RPC message to stdout.
func (t *StdioTransport) WriteMessage(message json.RawMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, err := t.writer.Write(message)
	if err != nil {
		return err
	}
	_, err = t.writer.Write([]byte("\n"))
	return err
}

// Close closes the transport (no-op for stdio).
func (t *StdioTransport) Close() error {
	return nil
}
