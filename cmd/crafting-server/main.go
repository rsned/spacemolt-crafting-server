// SpaceMolt Crafting Query MCP Server
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/db"
	"github.com/rsned/spacemolt-crafting-server/internal/crafting/engine"
	"github.com/rsned/spacemolt-crafting-server/internal/crafting/mcp"
	"github.com/rsned/spacemolt-crafting-server/internal/crafting/sync"
)

func main() {
	// Parse flags
	dbPath := flag.String("db", "data/crafting/crafting.db", "Path to SQLite database")
	importRecipes := flag.String("import-recipes", "", "Import recipes from JSON file")
	importSkills := flag.String("import-skills", "", "Import skills from JSON file")
	importMarket := flag.String("import-market", "", "Import market data from JSON file")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down...")
		cancel()
	}()

	// Open database
	database, err := db.OpenAndInit(ctx, *dbPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	// Handle import commands
	if *importRecipes != "" || *importSkills != "" || *importMarket != "" {
		syncer := sync.NewSyncer(database)

		if *importRecipes != "" {
			logger.Info("importing recipes", "file", *importRecipes)
			if err := syncer.ImportRecipesFromFile(ctx, *importRecipes); err != nil {
				logger.Error("failed to import recipes", "error", err)
				os.Exit(1)
			}
			logger.Info("recipes imported successfully")
		}

		if *importSkills != "" {
			logger.Info("importing skills", "file", *importSkills)
			if err := syncer.ImportSkillsFromFile(ctx, *importSkills); err != nil {
				logger.Error("failed to import skills", "error", err)
				os.Exit(1)
			}
			logger.Info("skills imported successfully")
		}

		if *importMarket != "" {
			logger.Info("importing market data", "file", *importMarket)
			if err := syncer.ImportMarketDataFromFile(ctx, *importMarket); err != nil {
				logger.Error("failed to import market data", "error", err)
				os.Exit(1)
			}
			logger.Info("market data imported successfully")
		}

		// If only doing imports, exit
		if flag.NArg() == 0 {
			return
		}
	}

	// Create engine and server
	eng := engine.New(database)
	server := mcp.NewServer(eng, logger)

	// Run MCP server
	logger.Info("starting MCP server", "db", *dbPath)
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "server stopped")
}
