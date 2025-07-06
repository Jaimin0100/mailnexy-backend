package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/middleware"
	"mailnexy/routes"
	"mailnexy/utils"
	"mailnexy/worker"
)

func main() {
	// Initialize logger
	logger := log.New(os.Stdout, "WARMUP: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Load configuration
	if err := config.LoadConfig(); err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	if err := config.ConnectDB(); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	// Create Fiber app
	app := fiber.New()

	// Add CORS middleware
	app.Use(middleware.CORS())

	// Initialize mailers - pass both DB and warmup email from config
	warmupMailer := utils.NewWarmupMailer(config.DB, config.AppConfig.WarmupEmail)

	// Initialize and start warmup worker
	warmupWorker := worker.NewWarmupWorker(config.DB, warmupMailer, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go warmupWorker.Start(ctx)

	// Add this after the warmup worker initialization
	uniboxWorker := worker.NewUniboxWorker(config.DB, log.New(os.Stdout, "UNIBOX: ", log.LstdFlags))
	go uniboxWorker.Start(ctx)

	// Setup routes
	routes.SetupRoutes(app, config.DB)

	// Health check endpoint
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "running",
			"version": "1.0.0",
		})
	})

	// Start server
	logger.Printf("🚀 Server starting on port %s", config.AppConfig.ServerPort)
	if err := app.Listen(":" + config.AppConfig.ServerPort); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}