package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"mailnexy/models"
	"mailnexy/utils"
	"gorm.io/gorm"
)

type WarmupWorker struct {
	DB           *gorm.DB
	WarmupMailer *utils.WarmupMailer
	Logger       *log.Logger
}

func NewWarmupWorker(db *gorm.DB, mailer *utils.WarmupMailer, logger *log.Logger) *WarmupWorker {
	return &WarmupWorker{
		DB:           db,
		WarmupMailer: mailer,
		Logger:       logger,
	}
}

func (ww *WarmupWorker) Start(ctx context.Context) {
	// Initial delay to let the server start up
	time.Sleep(10 * time.Second)

	ww.Logger.Println("Warmup worker started")

	ticker := time.NewTicker(30 * time.Second) // Check every 1 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ww.Logger.Println("Warmup worker shutting down...")
			return
		case <-ticker.C:
			ww.processActiveWarmups()
		}
	}
}

func (ww *WarmupWorker) processActiveWarmups() {
	var activeSenders []models.Sender
	if err := ww.DB.Where("is_warming_up = ?", true).Find(&activeSenders).Error; err != nil {
		ww.Logger.Printf("Error fetching active warmups: %v", err)
		return
	}

	for _, sender := range activeSenders {
		if err := ww.processSenderWarmup(sender); err != nil {
			ww.Logger.Printf("Error processing warmup for sender %d: %v", sender.ID, err)
			ww.updateSenderError(sender.ID, err.Error())
		}
	}
}

func (ww *WarmupWorker) checkStageAdvancement(sender models.Sender, schedule models.WarmupSchedule, currentStage models.WarmupStage) error {
	// Check if we've completed the required days in this stage
	if sender.WarmupStartedAt != nil {
		daysInStage := int(time.Since(*sender.WarmupStartedAt).Hours() / 24)
		if daysInStage >= currentStage.DurationDays {
			// Find next stage
			nextStage := ww.getCurrentStage(currentStage.StageNumber+1, schedule.Stages)
			if nextStage != nil {
				// Advance to next stage
				if err := ww.DB.Model(&sender).Updates(map[string]interface{}{
					"warmup_stage":      nextStage.StageNumber,
					"warmup_sent_today": 0,
					"warmup_started_at": time.Now(), // Reset timer for new stage
				}).Error; err != nil {
					return fmt.Errorf("failed to advance to next stage: %v", err)
				}
			} else {
				// No more stages - warmup complete
				if err := ww.DB.Model(&sender).Updates(map[string]interface{}{
					"is_warming_up":       false,
					"warmup_completed_at": time.Now(),
				}).Error; err != nil {
					return fmt.Errorf("failed to complete warmup: %v", err)
				}
			}
		}
	}
	return nil
}

func (ww *WarmupWorker) processSenderWarmup(sender models.Sender) error {
	// Get the current warmup schedule
	var schedule models.WarmupSchedule
	if err := ww.DB.Where("sender_id = ? AND is_active = ?", sender.ID, true).
		Preload("Stages").
		First(&schedule).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			ww.Logger.Printf("No active schedule found for sender %d - stopping warmup", sender.ID)
			return ww.DB.Model(&sender).Updates(map[string]interface{}{
				"is_warming_up":       false,
				"warmup_completed_at": time.Now(),
				"last_error":          "no active schedule found",
			}).Error
		}
		return fmt.Errorf("database error: %v", err)
	}

	ww.Logger.Printf("Processing warmup for sender %d (stage %d)", sender.ID, sender.WarmupStage)

	// Check if we need to reset daily counters
	if sender.WarmupStartedAt != nil && isNewDay(*sender.WarmupStartedAt) {
		if err := ww.DB.Model(&sender).Update("warmup_sent_today", 0).Error; err != nil {
			return err
		}
		sender.WarmupSentToday = 0
	}

	// Find current stage
	currentStage := ww.getCurrentStage(sender.WarmupStage, schedule.Stages)
	if currentStage == nil {
		return fmt.Errorf("no matching stage found for stage %d", sender.WarmupStage)
	}

	// Rest of your function remains the same...
	// Calculate emails to send in this batch
	emailsToSend := currentStage.EmailsPerDay - sender.WarmupSentToday
	if emailsToSend <= 0 {
		return nil
	}

	// Limit to max 5 emails per batch to avoid hitting rate limits
	if emailsToSend > 5 {
		emailsToSend = 5
	}

	// Send warmup emails
	for i := 0; i < emailsToSend; i++ {
		if err := ww.WarmupMailer.SendWarmupEmail(sender.ID, sender.FromEmail, sender.FromName); err != nil {
			ww.Logger.Printf("Error sending warmup email for sender %d: %v", sender.ID, err)
			continue
		}

		// Update counters
		if err := ww.DB.Model(&sender).Updates(map[string]interface{}{
			"warmup_sent_today": gorm.Expr("warmup_sent_today + ?", 1),
			"sent_today":        gorm.Expr("sent_today + ?", 1),
			"total_sent":        gorm.Expr("total_sent + ?", 1),
		}).Error; err != nil {
			return err
		}
	}

	// Check if we should advance to next stage
	return ww.checkStageAdvancement(sender, schedule, *currentStage)
}

// Helper functions...

func (ww *WarmupWorker) getCurrentStage(stageNumber int, stages []models.WarmupStage) *models.WarmupStage {
	for _, stage := range stages {
		if stage.StageNumber == stageNumber {
			return &stage
		}
	}
	return nil
}

func (ww *WarmupWorker) updateSenderError(senderID uint, errorMsg string) {
	ww.DB.Model(&models.Sender{}).Where("id = ?", senderID).
		Updates(map[string]interface{}{
			"last_error":     errorMsg,
			"last_tested_at": time.Now(),
		})
}

func isNewDay(lastUpdate time.Time) bool {
	return time.Now().Day() != lastUpdate.Day()
}