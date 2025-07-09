package routes

import (
	"log"
	"os"

	controller "mailnexy/controllers"
	"mailnexy/middleware"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"gorm.io/gorm"
)

func SetupAuthRoutes(app *fiber.App, db *gorm.DB) {
	// Initialize logger
	authLogger := log.New(os.Stdout, "AUTH: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Auth routes group with logging middleware
	auth := app.Group("/auth", logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Public auth endpoints (no authentication required)
	auth.Post("/register", controller.Register)
	auth.Post("/login", controller.Login)
	auth.Post("/forgot-password", controller.ForgotPassword)
	auth.Post("/verify-reset-otp", controller.VerifyResetPasswordOTP)
	auth.Post("/reset-password", controller.ResetPassword)
	auth.Post("/refresh", controller.RefreshToken)

	// Google OAuth routes
	auth.Get("/google", controller.GoogleOAuth)
	auth.Get("/google/callback", controller.GoogleOAuthCallback)

	// Protected auth endpoints (require valid JWT)
	protectedAuth := auth.Group("", middleware.Protected())
	protectedAuth.Post("/logout", controller.Logout)
	protectedAuth.Post("/change-password", controller.ChangePassword)
	protectedAuth.Get("/me", controller.GetCurrentUser)

	// OTP routes group
	otp := app.Group("/otp", logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	otp.Post("/send", controller.SendOTP)
	otp.Post("/verify", controller.VerifyOTP)
	otp.Post("/resend", controller.ResendOTP)

	// Payment routes (protected)
	payment := app.Group("/payment", middleware.Protected())
	payment.Post("/create-intent", controller.CreatePaymentIntent)
	payment.Post("/webhook", controller.HandlePaymentWebhook)

	// Log initialization
	authLogger.Println("Authentication routes initialized successfully")
}

func SetupAPIRoutes(app *fiber.App, db *gorm.DB) {
	// Initialize controllers with their respective loggers
	verifyLogger := log.New(os.Stdout, "VERIFY: ", log.Ldate|log.Ltime|log.Lshortfile)
	warmupLogger := log.New(os.Stdout, "WARMUP: ", log.Ldate|log.Ltime|log.Lshortfile)

	warmupController := controller.NewWarmupController(warmupLogger)
	verificationController := controller.NewVerificationController(db, verifyLogger)
	campaignController := controller.NewCampaignController(db, log.New(os.Stdout, "CAMPAIGN: ", log.LstdFlags))
	leadController := controller.NewLeadController(db, log.New(os.Stdout, "LEAD: ", log.LstdFlags))
	dashboardController := controller.NewDashboardController(db, log.New(os.Stdout, "DASHBOARD: ", log.LstdFlags))
	uniboxController := controller.NewUniboxController(db, log.New(os.Stdout, "UNIBOX: ", log.LstdFlags))

	// API group with versioning and protection
	api := app.Group("/api/v1", middleware.Protected(), logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Dashboard routes
	dashboard := api.Group("/dashboard")
	dashboard.Get("/stats", dashboardController.GetDashboardStats)
	dashboard.Get("/metrics", dashboardController.GetEmailMetricsOverTime)
	dashboard.Get("/recent-campaigns", dashboardController.GetRecentCampaigns)

	// Sender routes with rate limiting
	sender := api.Group("/senders", middleware.SenderRateLimiter())
	sender.Post("/", controller.CreateSender)
	sender.Get("/", controller.GetSenders)
	sender.Get("/:id", controller.GetSender)
	sender.Put("/:id", controller.UpdateSender)
	sender.Delete("/:id", controller.DeleteSender)
	sender.Post("/:id/test", controller.TestSender)
	sender.Post("/:id/verify", controller.VerifySender)

	// Warmup routes
	warmup := sender.Group("/:id/warmup")
	warmup.Post("/start", warmupController.StartWarmup)
	warmup.Post("/stop", warmupController.StopWarmup)
	warmup.Get("/status", warmupController.GetWarmupStatus)
	warmup.Post("/schedules", warmupController.CreateWarmupSchedule)
	warmup.Get("/schedules", warmupController.GetWarmupSchedules)
	warmup.Get("/stats", warmupController.GetWarmupStats)

	// Separate route for schedule updates
	api.Put("/senders/warmup/schedules/:id", warmupController.UpdateWarmupSchedule)

	// Verification routes
	verify := api.Group("/verify")
	verify.Get("/email", verificationController.VerifyEmail)
	verify.Post("/bulk", verificationController.BulkVerify)
	verify.Get("/results/:id", verificationController.GetVerificationResults)

	// Campaign routes
	campaign := api.Group("/campaigns")
	campaign.Post("/", campaignController.CreateCampaign)
	campaign.Get("/", campaignController.GetCampaigns)
	campaign.Get("/:id", campaignController.GetCampaign)
	campaign.Put("/:id", campaignController.UpdateCampaign)
	campaign.Post("/:id/start", campaignController.StartCampaign)
	campaign.Post("/:id/stop", campaignController.StopCampaign)
	campaign.Get("/:id/flow", campaignController.GetCampaignFlow)
	campaign.Put("/:id/flow", campaignController.UpdateCampaignFlow)
	campaign.Get("/:id/stats", campaignController.GetCampaignStats)
	campaign.Delete("/:id", campaignController.DeleteCampaign)
	campaign.Post("/webhook", campaignController.HandleCampaignWebhook)
	// routes.go
	campaign.Put("/:id/lead-lists", campaignController.UpdateCampaignLeadLists)
	// routes.go - Add these to your existing routes
	campaign.Put("/:id/settings", campaignController.UpdateCampaignSettings)
	campaign.Get("/:id/tracking-stats", campaignController.GetTrackingStats)

	// WebSocket route for campaign progress
	app.Get("/api/v1/campaigns/progress", websocket.New(func(c *websocket.Conn) {
		controller.HandleCampaignProgressWS(c)
	}))
	app.Get("/track/open/:messageID/:token", campaignController.HandleOpenTracking)
	app.Get("/track/click/:messageID/:token", campaignController.HandleClickTracking)

	// Lead routes
	lead := api.Group("/leads")
	lead.Post("/", leadController.CreateLead)
	lead.Get("/", leadController.GetLeads)
	lead.Get("/:id", leadController.GetLead)
	lead.Put("/:id", leadController.UpdateLead)
	lead.Delete("/:id", leadController.DeleteLead)
	lead.Post("/import", leadController.ImportLeads)
	lead.Post("/export", leadController.ExportLeads)

	// Lead list routes
	leadList := api.Group("/lead-lists")
	leadList.Post("/", leadController.CreateLeadList)
	leadList.Get("/", leadController.GetLeadLists)
	leadList.Get("/:id", leadController.GetLeadList)
	leadList.Put("/:id", leadController.UpdateLeadList)
	leadList.Delete("/:id", leadController.DeleteLeadList)
	leadList.Post("/:id/add-leads", leadController.AddLeadsToList)
	leadList.Post("/:id/remove-leads", leadController.RemoveLeadsFromList)
	leadList.Get("/:id/leads", leadController.GetLeadListMembers)
	

	// Start the sender counter reset goroutine
	campaignSender := utils.NewCampaignSender(db, log.New(os.Stdout, "SENDER: ", log.LstdFlags))
	go campaignSender.ResetDailyCounters()

	
	// Unibox routes
	unibox := api.Group("/unibox")
	unibox.Post("/fetch", uniboxController.FetchEmails)
	unibox.Get("/emails", uniboxController.GetEmails)
	unibox.Get("/emails/:id", uniboxController.GetEmail)
	unibox.Put("/emails/:id", uniboxController.UpdateEmail)
	unibox.Put("/emails/:id/move", uniboxController.MoveEmail)
	unibox.Get("/folders", uniboxController.GetFolders)
	unibox.Post("/folders", uniboxController.CreateFolder)
	unibox.Delete("/folders/:id", uniboxController.DeleteFolder)

	// Log initialization
	log.Println("API routes initialized successfully")
}

func SetupRoutes(app *fiber.App, db *gorm.DB) {
	// Initialize Stripe
	controller.InitStripe()

	// Setup health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Setup auth routes
	SetupAuthRoutes(app, db)

	// Setup API routes
	SetupAPIRoutes(app, db)

	// Setup 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Not Found",
			"message": "The requested resource was not found",
		})
	})
}
