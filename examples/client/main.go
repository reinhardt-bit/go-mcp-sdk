package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp"
	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
)

func main() {
    // Use stdio transport to connect to the server
    transport := transports.NewStdioTransport()
    client := mcp.NewClient(transport)

    // Test ListPrompts
    prompts, err := client.ListPrompts()
    if err != nil {
        log.Fatal("ListPrompts failed:", err)
    }
    fmt.Println("Prompts:", prompts)

    // Test GetResource
    type EchoParams struct {
        Message string `json:"message"`
    }
    type EchoResponse struct {
        Echo string `json:"echo"`
    }
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
