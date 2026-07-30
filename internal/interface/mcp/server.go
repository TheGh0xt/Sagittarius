package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheGh0xt/Sagittarius/internal/application/polymarket"
	"github.com/TheGh0xt/Sagittarius/internal/application/signal"
	handler "github.com/TheGh0xt/Sagittarius/internal/interface/mcp/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	ms  *mcp.Server
	ph  *handler.Pmhandler
	sh  *handler.SignalHandler
	slg *slog.Logger
}

func NewServer(ms *mcp.Server, ps polymarket.Service, ss signal.Service, slg *slog.Logger) *Server {
	s := &Server{
		ms:  ms,
		ph:  handler.NewPmhandler(ps, slg),
		sh:  handler.NewSignalHandler(ss, slg),
		slg: slg,
	}

	s.RegisterPmiTools()

	return s
}

func (s *Server) GetMCPServer() *mcp.Server {
	return s.ms
}

func (s *Server) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) RunStdio(ctx context.Context) error {
	s.slg.Info("Sagittarius", "state", "Starting Sagittarius MCP Server in stdio mode...")
	err := s.ms.Run(ctx, &mcp.StdioTransport{})
	s.slg.Info("Sagittarius", "state", "Stopped", "error", err.Error())
	return err
}

func (s *Server) GetSSEHandler() http.Handler {
	s.slg.Info("Sagittarius", "state", "Getting SSE handler...")
	return mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return s.ms
	}, nil)
}

func (s *Server) GetHTTPHandler(logger *slog.Logger) http.Handler {
	s.slg.Info("Sagittarius", "state", "Getting HTTP handler...")
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.ms
	}, &mcp.StreamableHTTPOptions{
		Logger:         logger,
		SessionTimeout: time.Minute * 15,
	})
}

func (s *Server) RunHTTP(ctx context.Context, port string) error {
	s.slg.Info("Sagittarius", "state", "Starting Streamable HTTP transport on :"+port)
	mux := http.NewServeMux()

	mux.Handle("/mcp", s.GetHTTPHandler(s.slg))
	mux.HandleFunc("/health", s.healthcheck)

	return s.runServer(ctx, port, mux)
}

func (s *Server) RunSSE(ctx context.Context, port string) error {
	s.slg.Info("Sagittarius", "state", "Starting SSE transport on :"+port)
	handler := s.GetSSEHandler()

	mux := http.NewServeMux()
	mux.Handle("/sse", handler)
	mux.Handle("/message", handler)
	mux.HandleFunc("/health", s.healthcheck)

	return s.runServer(ctx, port, mux)
}

func (s *Server) runServer(ctx context.Context, port string, handler http.Handler) error {
	s.slg.Info("Sagittarius", "state", "Starting server on :"+port, "local addr", "http://localhost:"+port)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():

	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.slg.Info("Sagittarius", "state", "Shutting down server on :"+port)
	return srv.Shutdown(shutdownCtx)
}
