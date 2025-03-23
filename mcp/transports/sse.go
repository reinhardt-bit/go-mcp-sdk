package transports

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// SSETransport implements the Transport interface using Server-Sent Events.
type SSETransport struct {
	url       string
	client    *http.Client
	eventChan chan json.RawMessage
	stop      chan struct{}
	wg        sync.WaitGroup
}

// NewSSETransport creates a new SSE transport instance.
func NewSSETransport(url string) *SSETransport {
	t := &SSETransport{
		url:       url,
		client:    &http.Client{},
		eventChan: make(chan json.RawMessage),
		stop:      make(chan struct{}),
	}
	t.wg.Add(1)
	go t.readLoop()
	return t
}

// readLoop reads SSE events from the server.
func (t *SSETransport) readLoop() {
	defer t.wg.Done()
	req, _ := http.NewRequest("GET", t.url+"/events", nil)
	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-t.stop:
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if bytes.HasPrefix([]byte(line), []byte("data: ")) {
				data := bytes.TrimPrefix([]byte(line), []byte("data: "))
				data = bytes.TrimSpace(data)
				var msg json.RawMessage
				if json.Unmarshal(data, &msg) == nil {
					t.eventChan <- msg
				}
			}
		}
	}
}

// ReadMessage reads a message from the SSE event channel.
func (t *SSETransport) ReadMessage() (json.RawMessage, error) {
	select {
	case msg := <-t.eventChan:
		return msg, nil
	case <-t.stop:
		return nil, io.EOF
	}
}

// WriteMessage sends a message to the server via HTTP POST.
func (t *SSETransport) WriteMessage(message json.RawMessage) error {
	resp, err := t.client.Post(t.url+"/request", "application/json", bytes.NewReader(message))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// Close shuts down the SSE transport.
func (t *SSETransport) Close() error {
	close(t.stop)
	t.wg.Wait()
	return nil
}
