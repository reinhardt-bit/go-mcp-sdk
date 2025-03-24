package main

import (
	"fmt"
	"log"

	"github.com/reinhardt-bit/go-mcp-sdk/mcp"
	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
)

func main() {
	// Create and configure the server
	server := mcp.NewServer()
	server.SetOnStart(func() error {
		fmt.Println("Server starting...")
		return nil
	})
	server.SetOnStop(func() error {
		fmt.Println("Server stopping...")
		return nil
	})

	// Register an echo resource
	type EchoParams struct {
		Message string `json:"message"`
	}
	type EchoResponse struct {
		Echo string `json:"echo"`
	}
	server.RegisterResource("echo", mcp.NewResource(func(params EchoParams) (EchoResponse, error) {
		return EchoResponse{Echo: params.Message}, nil
	}))

	// Register a prompt
	server.RegisterPrompt(mcp.Prompt{
		Name:     "greeting",
		Template: "Hello, {{name}}!",
	})

	// Serve over stdio
	transport := transports.NewStdioTransport()
	if err := server.Serve(transport); err != nil {
		log.Fatal(err)
	}
}

// package main

// import (
// 	"fmt"
// 	"log"

// 	"github.com/reinhardt-bit/go-mcp-sdk/mcp"
// 	"github.com/reinhardt-bit/go-mcp-sdk/mcp/transports"
// )

// func main() {
//     // Create and configure the server
//     server := mcp.NewServer()
//     server.SetOnStart(func() error {
//         fmt.Println("Server starting...")
//         return nil
//     })
//     server.SetOnStop(func() error {
//         fmt.Println("Server stopping...")
//         return nil
//     })

//     // Register an echo resource
//     type EchoParams struct {
//         Message string `json:"message"`
//     }
//     type EchoResponse struct {
//         Echo string `json:"echo"`
//     }
//     server.RegisterResource("echo", mcp.NewResource(func(params EchoParams) (EchoResponse, error) {
//         return EchoResponse{Echo: params.Message}, nil
//     }))

//     // Register a prompt
//     server.RegisterPrompt(mcp.Prompt{
//         Name:     "greeting",
//         Template: "Hello, {{name}}!",
//     })

//     // Serve over stdio
//     transport := transports.NewStdioTransport()
//     if err := server.Serve(transport); err != nil {
//         log.Fatal(err)
//     }
// }
