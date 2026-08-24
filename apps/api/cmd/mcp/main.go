package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"api/internal/platform/mcpface"
	"api/pkg/health"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultPort = 9285
	healthPath  = "/healthz"
	mcpPath     = "/mcp"
)

func main() {
	port := envInt("KUN_MCP_PORT", defaultPort)

	health.MaybeProbe(port, healthPath)

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	upstreamBase := os.Getenv("KUN_MCP_UPSTREAM_BASE")
	if upstreamBase == "" {
		slog.Error("KUN_MCP_UPSTREAM_BASE is required (the public /v1 face base, e.g. http://catalog:9281)")
		os.Exit(1)
	}
	host := os.Getenv("KUN_MCP_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	up := mcpface.NewUpstream(upstreamBase)
	spec, err := loadV2Spec(up)
	if err != nil {
		slog.Error("load v2 openapi", "error", err)
		os.Exit(1)
	}
	server, err := mcpface.NewServer(up, spec)
	if err != nil {
		slog.Error("mcp server from spec", "error", err)
		os.Exit(1)
	}

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	mux := http.NewServeMux()
	mux.Handle(mcpPath, handler)
	mux.Handle(mcpPath+"/", handler)
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := host + ":" + strconv.Itoa(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("mcp server listening", "addr", addr, "upstream", upstreamBase, "endpoint", mcpPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("mcp server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("mcp server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("mcp server shutdown", "error", err)
	}
}

func loadV2Spec(up *mcpface.Upstream) ([]byte, error) {
	if p := os.Getenv("KUN_MCP_OPENAPI_PATH"); p != "" {
		return os.ReadFile(p)
	}
	status, body, err := up.Get(context.Background(), "/v2/catalog/openapi.json", nil, "")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET /v2/catalog/openapi.json: %d", status)
	}
	return body, nil
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
