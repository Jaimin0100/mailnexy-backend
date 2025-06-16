package routes

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	controller "mailnexy/controllers"
	"mailnexy/middleware"
	"mailnexy/utils"
	"gorm.io/gorm"
)

func SetupRoutes(app *fiber.App, db *gorm.DB) {
	controller.InitStripe()
	api := app.Group("/api")

	// Initialize logger
	verifyLogger := log.New(os.Stdout, "VERIFY: ", log.Ldate|log.Ltime|log.Lshortfile)
	// Initialize logger
	warmupLogger := log.New(os.Stdout, "WARMUP: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Initialize controllers
	warmupController := controller.NewWarmupController(warmupLogger)
	verificationController := controller.NewVerificationController(db, verifyLogger)

	// Payment routes
	payment := app.Group("/payment")
	payment.Post("/create-intent", middleware.Protected(), controller.CreatePaymentIntent)
	payment.Post("/webhook", controller.HandlePaymentWebhook)

	// Auth routes
	auth := app.Group("/auth")
	auth.Post("/register", controller.Register)
	auth.Post("/login", controller.Login)
	auth.Post("/logout", middleware.Protected(), controller.Logout)
	auth.Post("/refresh", controller.RefreshToken)

	// Password reset flow
	auth.Post("/forgot-password", controller.ForgotPassword)
	auth.Post("/verify-reset-otp", controller.VerifyResetPasswordOTP)
	auth.Post("/reset-password", controller.ResetPassword)

	// OTP routes
	otp := app.Group("/otp")
	otp.Post("/send", controller.SendOTP)
	otp.Post("/verify", controller.VerifyOTP)
	otp.Post("/resend", controller.ResendOTP)

	// Google OAuth routes
	oauth := api.Group("/oauth")
	oauth.Get("/google", controller.GoogleOAuth)
	oauth.Get("/google/callback", controller.GoogleOAuthCallback)

	// Protected routes
	protected := api.Group("/protected", middleware.Protected())
	protected.Get("/me", controller.GetCurrentUser)

	// Sender routes
	sender := protected.Group("/senders")
	sender.Post("/", controller.CreateSender)
	sender.Get("/", controller.GetSenders)
	sender.Get("/:id", controller.GetSender)
	sender.Put("/:id", controller.UpdateSender)
	sender.Delete("/:id", controller.DeleteSender)
	sender.Post("/:id/test", controller.TestSender)
	sender.Post("/:id/verify", controller.VerifySender)

	// Warmup routes
	warmup := sender.Group("/:id/warmup", middleware.Protected())
	warmup.Post("/start", warmupController.StartWarmup)
	warmup.Post("/stop", warmupController.StopWarmup)
	warmup.Get("/status", warmupController.GetWarmupStatus)

	// Warmup schedule routes
	warmup.Post("/schedules", warmupController.CreateWarmupSchedule)
	warmup.Get("/schedules", warmupController.GetWarmupSchedules)
	warmup.Get("/stats", warmupController.GetWarmupStats)

	// Separate route for schedule updates since it has different path param
	sender.Put("/warmup/schedules/:id", middleware.Protected(), warmupController.UpdateWarmupSchedule)

	// Verification routes
	verify := protected.Group("/verify")
	verify.Get("/email", verificationController.VerifyEmail) // GET /api/protected/verify/email?email=test@example.com
	verify.Post("/bulk", verificationController.BulkVerify)
	verify.Get("/results/:id", verificationController.GetVerificationResults)

	// Add these routes after the existing ones
	campaign := protected.Group("/campaigns")
	campaignController := controller.NewCampaignController(db, log.New(os.Stdout, "CAMPAIGN: ", log.LstdFlags))

	campaign.Post("/", campaignController.CreateCampaign)
	campaign.Get("/", campaignController.GetCampaigns)
	campaign.Get("/:id", campaignController.GetCampaign)
	campaign.Put("/:id", campaignController.UpdateCampaign)
	// campaign.Delete("/:id", campaignController.DeleteCampaign)
	campaign.Post("/:id/start", campaignController.StartCampaign)
	campaign.Post("/:id/stop", campaignController.StopCampaign)
	campaign.Get("/:id/flow", campaignController.GetCampaignFlow)
	campaign.Put("/:id/flow", campaignController.UpdateCampaignFlow)
	campaign.Get("/:id/stats", campaignController.GetCampaignStats)
	// Add webhook route
	campaign.Post("/webhook", campaignController.HandleCampaignWebhook)

	// Start the sender counter reset goroutine
	campaignSender := utils.NewCampaignSender(db, log.New(os.Stdout, "SENDER: ", log.LstdFlags))
	go campaignSender.ResetDailyCounters()

	// Add this near other controller initializations
	leadController := controller.NewLeadController(db, log.New(os.Stdout, "LEAD: ", log.LstdFlags))

	// Then update your lead routes to use the controller:
	lead := protected.Group("/leads")
	lead.Post("/", leadController.CreateLead)
	lead.Get("/", leadController.GetLeads)
	lead.Get("/:id", leadController.GetLead)
	lead.Put("/:id", leadController.UpdateLead)
	lead.Delete("/:id", leadController.DeleteLead)
	lead.Post("/import", leadController.ImportLeads)
	lead.Post("/export", leadController.ExportLeads)

	leadList := protected.Group("/lead-lists")
	leadList.Post("/", leadController.CreateLeadList)
	leadList.Get("/", leadController.GetLeadLists)
	leadList.Get("/:id", leadController.GetLeadList)
	leadList.Put("/:id", leadController.UpdateLeadList)
	leadList.Delete("/:id", leadController.DeleteLeadList)
	leadList.Post("/:id/add-leads", leadController.AddLeadsToList)
	leadList.Post("/:id/remove-leads", leadController.RemoveLeadsFromList)
	leadList.Get("/:id/leads", leadController.GetLeadListMembers)
}