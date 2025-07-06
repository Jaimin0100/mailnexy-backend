package worker

import (
	"context"
	"log"
	"time"

	controller "mailnexy/controllers"
	"mailnexy/models"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

type UniboxWorker struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewUniboxWorker(db *gorm.DB, logger *log.Logger) *UniboxWorker {
	return &UniboxWorker{
		db:     db,
		logger: logger,
	}
}

func (uw *UniboxWorker) Start(ctx context.Context) {
	uw.logger.Println("Starting Unibox worker...")
	ticker := time.NewTicker(5 * time.Minute) // Fetch emails every 5 minutes

	for {
		select {
		case <-ticker.C:
			uw.fetchAllEmails()
		case <-ctx.Done():
			uw.logger.Println("Stopping Unibox worker...")
			ticker.Stop()
			return
		}
	}
}

func (uw *UniboxWorker) fetchAllEmails() {
	uw.logger.Println("Fetching emails for all users...")

	// Get all users with senders configured
	var users []models.User
	if err := uw.db.Preload("Senders", "imap_host IS NOT NULL AND imap_host != ''").Find(&users).Error; err != nil {
		uw.logger.Printf("Failed to fetch users: %v", err)
		return
	}

	uniboxController := controller.NewUniboxController(uw.db, uw.logger)

	// Create a minimal Fiber app to get the proper context
	app := fiber.New()

	for _, user := range users {
		if len(user.Senders) == 0 {
			continue
		}

		// Create a proper Fiber v2 context
		fctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(fctx)
		fctx.Locals("user", &user)

		if err := uniboxController.FetchEmails(fctx); err != nil {
			uw.logger.Printf("Failed to fetch emails for user %d: %v", user.ID, err)
			continue
		}
	}
}
