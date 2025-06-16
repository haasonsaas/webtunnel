package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yourusername/webtunnel/internal/config"
	"github.com/yourusername/webtunnel/internal/server"
	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "webtunnel",
		Short: "WebTunnel - Secure remote terminal access",
		Long:  "WebTunnel provides secure, scalable remote terminal access through web interface",
	}

	rootCmd.AddCommand(
		newServeCommand(),
		newVersionCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func newServeCommand() *cobra.Command {
	var configFile string
	
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the WebTunnel server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(configFile)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "config file (default is $HOME/.webtunnel.yaml)")
	cmd.Flags().String("port", "8443", "port to listen on")
	cmd.Flags().String("host", "0.0.0.0", "host to bind to")
	cmd.Flags().Bool("tls", true, "use TLS")
	cmd.Flags().String("db-url", "postgres://localhost/webtunnel?sslmode=disable", "database URL")
	cmd.Flags().String("redis-url", "redis://localhost:6379", "Redis URL")

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("WebTunnel %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built: %s\n", date)
		},
	}
}

func runServer(configFile string) error {
	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logger
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	// Create server
	srv, err := server.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start server
	logger.Info("Starting WebTunnel server", 
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
		zap.Bool("tls", cfg.Server.TLS),
	)

	return srv.Run(ctx)
}