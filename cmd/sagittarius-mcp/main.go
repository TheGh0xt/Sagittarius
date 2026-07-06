package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	pmApp "github.com/TheGh0xt/Sagittarius/internal/application/polymarket"
	sigApp "github.com/TheGh0xt/Sagittarius/internal/application/signal"
	"github.com/TheGh0xt/Sagittarius/internal/infrastructure/polymarket"

	smcp "github.com/TheGh0xt/Sagittarius/internal/interface/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	NAME    = "Sagittarius MCP Server"
	VERSION = "1.0.0"
)

var (
	transportType = flag.String("transport", "http", "Transport type to use (stdio, sse or http)")
	port          = flag.String("port", "8080", "Port to listen on when using sse transport")
)

func main() {
	flag.Parse()

	// return back to stderr
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handleSignals(cancel)

	ms := mcp.NewServer(
		&mcp.Implementation{
			Name:    NAME,
			Version: VERSION,
		},
		&mcp.ServerOptions{
			Logger: logger,
		},
	)

	client := polymarket.NewClient(logger)
	pSvc := pmApp.NewPmService(client, logger)
	sSvc := sigApp.NewSignalService(client, client, logger)
	server := smcp.NewServer(ms, pSvc, sSvc, logger)

	var err error

	switch *transportType {
	case "stdio":
		err = server.RunStdio(ctx)

	case "http":
		err = server.RunHTTP(ctx, *port)

	case "sse":
		err = server.RunSSE(ctx, *port)

	default:
		slog.Error("unknown transport", "transport", *transportType)
	}

	if err != nil {
		slog.Error("server error", "error", err)
	}
}

// handleSignals listens for shutdown signals and cancels the context when received.
func handleSignals(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Received shutdown signal...")
		cancel()
	}()
}
