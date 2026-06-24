package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheGh0xt/Sagittarius/internal/mcp"
)

func main() {
	// Set up flags
	transportType := flag.String("transport", "stdio", "Transport type to use (stdio or sse)")
	port := flag.Int("port", 8080, "Port to listen on when using sse transport")
	flag.Parse()

	// Redirect standard log output to stderr to prevent corrupting stdio transport
	log.SetOutput(os.Stderr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Start server
	server := mcp.NewServer()

	switch *transportType {
	case "stdio":
		log.Println("Starting Sagittarius MCP Server in stdio mode...")
		if err := server.RunStdio(ctx); err != nil {
			log.Fatalf("Server exit error: %v", err)
		}
	case "sse":
		log.Printf("Starting Sagittarius MCP Server in SSE mode on port %d...\n", *port)
		mux := http.NewServeMux()
		
		// Mount SSE handler at /sse and /message (internally handled by SSEHandler)
		handler := server.GetSSEHandler()
		mux.Handle("/sse", handler)
		mux.Handle("/message", handler)

		httpServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", *port),
			Handler: mux,
		}

		go func() {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()

		// Wait for context cancellation to shutdown HTTP server
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
		log.Println("SSE server stopped.")
	default:
		log.Fatalf("Unknown transport type: %s", *transportType)
	}
}
