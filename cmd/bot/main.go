package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/josinSbazin/idea-bot/internal/config"
	"github.com/josinSbazin/idea-bot/internal/domain/service"
	"github.com/josinSbazin/idea-bot/internal/storage"
	"github.com/josinSbazin/idea-bot/internal/telegram"
	"github.com/josinSbazin/idea-bot/internal/web"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Idea Bot...")

	// Load configuration
	config.Load()
	cfg := config.Get()

	// Validate required config
	if cfg.Telegram.BotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.Claude.APIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}
	if cfg.Web.Username == "" || cfg.Web.Password == "" {
		log.Fatal("WEB_USERNAME and WEB_PASSWORD are required")
	}

	// Initialize SQLite
	if err := storage.Init(cfg.SQLite.Path); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer storage.Close()

	// Create services
	ideaService := service.NewIdeaService()

	// Create Telegram bot
	bot, err := telegram.NewBot(ideaService)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Create web handler
	webHandler, err := web.NewHandler(ideaService)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}

	// Setup HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Web.Port,
		Handler:      webHandler.SetupRoutes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to signal shutdown
	done := make(chan struct{})

	// Start Telegram bot in goroutine
	go func() {
		if err := bot.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Telegram bot error: %v", err)
		}
	}()

	// Start HTTP server in goroutine
	go func() {
		log.Printf("Web server listening on http://localhost:%s", cfg.Web.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)

		// Cancel context to stop Telegram bot
		cancel()

		// Graceful shutdown of HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		close(done)
	}()

	// Print startup info
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Idea Bot is running!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Web UI: http://localhost:%s\n", cfg.Web.Port)
	fmt.Printf("  Health: http://localhost:%s/health\n", cfg.Web.Port)
	fmt.Println("  Telegram: waiting for /idea commands...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()

	<-done
	log.Println("Shutdown complete")
}
