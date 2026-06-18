package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kent/infrapilot/internal/config"
	"github.com/kent/infrapilot/internal/server"
	"github.com/kent/infrapilot/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	configPath := flag.String("config", "", "Path to JSON configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st, err := store.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	srv := server.New(cfg, st)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("InfraPilot MCP server starting (region=%s, read_only=%t)", cfg.AWSRegion, cfg.ReadOnly)
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
