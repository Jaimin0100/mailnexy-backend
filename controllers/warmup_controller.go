package controller

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/models"
	"gorm.io/gorm"
)

const (
	ErrInvalidSenderID      = "invalid sender ID"
	ErrSenderNotFound       = "sender not found"
	ErrSMTPNotVerified      = "sender SMTP configuration must be verified before starting warmup"
	ErrWarmupAlreadyRunning = "warmup is already running for this sender"
	ErrWarmupNotRunning     = "warmup is not running for this sender"
	ErrInvalidRequestBody   = "invalid request body"
	ErrScheduleNotFound     = "schedule not found"
	ErrMessageIDRequired    = "message ID is required"
)

type WarmupController struct {
	Logger *log.Logger
}

func NewWarmupController(logger *log.Logger) *WarmupController {
	return &WarmupController{
		Logger: logger,
	}
}

func createDefaultWarmupSchedule(senderID uint, userID uint) error {
	defaultSchedule := models.WarmupSchedule{
		UserID:   userID,
		SenderID: senderID,
		Name:     "Default Warmup",
		IsActive: true,
		Stages: []models.WarmupStage{
			{
				StageNumber:  1,
				EmailsPerDay: 10,
				DurationDays: 3,
				ReplyTarget:  5,
			},
			{
				StageNumber:  2,
				EmailsPerDay: 20,
				DurationDays: 3,
				ReplyTarget:  10,
			},
			// Add more stages as needed
		},
	}

	return config.DB.Create(&defaultSchedule).Error
}

// StartWarmup starts the warmup process for a sender
func (wc *WarmupController) StartWarmup(c *fiber.Ctx) error {

	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, userID).First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrSenderNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching sender: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	if !sender.SMTPVerified {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrSMTPNotVerified,
		})
	}

	if sender.IsWarmingUp {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrWarmupAlreadyRunning,
		})
	}

	now := time.Now().UTC()
	updateData := map[string]interface{}{
		"is_warming_up":     true,
		"warmup_started_at": now,
		"warmup_stage":      1,
		"warmup_sent_today": 0,
		"last_error":        nil,
	}

	if err := config.DB.Model(&sender).Updates(updateData).Error; err != nil {
		wc.Logger.Printf("Failed to start warmup for sender %d: %v", senderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to start warmup",
		})
	}
	var schedule models.WarmupSchedule
	if err := config.DB.Where("sender_id = ? AND is_active = ?", senderID, true).
		Preload("Stages").
		First(&schedule).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			// Create default schedule if none exists
			if err := createDefaultWarmupSchedule(uint(senderID), userID); err != nil {
				wc.Logger.Printf("Failed to create default schedule: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "failed to create warmup schedule",
				})
			}
			// Reload the schedule
			if err := config.DB.Where("sender_id = ? AND is_active = ?", senderID, true).
				Preload("Stages").
				First(&schedule).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "failed to load warmup schedule",
				})
			}
		} else {
			wc.Logger.Printf("Database error fetching schedule: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}
	}

	if err := config.DB.Model(&sender).Updates(updateData).Error; err != nil {
		wc.Logger.Printf("Failed to start warmup for sender %d: %v", senderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to start warmup",
		})
	}

	wc.Logger.Printf("Warmup started for sender %d (user %d)", senderID, userID)
	return c.JSON(fiber.Map{
		"message": "warmup started successfully",
		"data": fiber.Map{
			"is_warming_up":     true,
			"warmup_started_at": now,
			"schedule_id":       schedule.ID,
		},
	})
}

// StopWarmup stops the warmup process for a sender
func (wc *WarmupController) StopWarmup(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, userID).First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrSenderNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching sender: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	if !sender.IsWarmingUp {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrWarmupNotRunning,
		})
	}

	now := time.Now().UTC()
	if err := config.DB.Model(&sender).Updates(map[string]interface{}{
		"is_warming_up":       false,
		"warmup_completed_at": now,
	}).Error; err != nil {
		wc.Logger.Printf("Failed to stop warmup for sender %d: %v", senderID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to stop warmup",
		})
	}

	return c.JSON(fiber.Map{
		"message": "warmup stopped successfully",
		"data": fiber.Map{
			"is_warming_up":       false,
			"warmup_completed_at": now,
		},
	})
}

// GetWarmupStatus returns the current warmup status for a sender
func (wc *WarmupController) GetWarmupStatus(c *fiber.Ctx) error {

	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, userID).First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrSenderNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching sender: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"is_warming_up":       sender.IsWarmingUp,
			"warmup_started_at":   sender.WarmupStartedAt,
			"warmup_completed_at": sender.WarmupCompletedAt,
			"warmup_stage":        sender.WarmupStage,
			"warmup_sent_today":   sender.WarmupSentToday,
		},
	})
}

// CreateWarmupSchedule creates a new warmup schedule
func (wc *WarmupController) CreateWarmupSchedule(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var input struct {
		Name     string `json:"name" validate:"required,min=3,max=50"`
		IsActive bool   `json:"is_active"`
		Stages   []struct {
			EmailsPerDay int `json:"emails_per_day" validate:"required,min=1,max=100"`
			DurationDays int `json:"duration_days" validate:"required,min=1,max=30"`
			ReplyTarget  int `json:"reply_target" validate:"min=0,max=100"`
		} `json:"stages" validate:"required,min=1,dive"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidRequestBody,
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, userID).First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrSenderNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching sender: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	schedule := models.WarmupSchedule{
		UserID:      userID,
		SenderID:    uint(senderID),
		Name:        input.Name,
		IsActive:    input.IsActive,
		TotalStages: len(input.Stages),
	}

	var stages []models.WarmupStage
	for i, stage := range input.Stages {
		stages = append(stages, models.WarmupStage{
			ScheduleID:   schedule.ID,
			StageNumber:  i + 1,
			EmailsPerDay: stage.EmailsPerDay,
			DurationDays: stage.DurationDays,
			ReplyTarget:  stage.ReplyTarget,
		})
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&schedule).Error; err != nil {
			return err
		}

		for i := range stages {
			stages[i].ScheduleID = schedule.ID
		}

		return tx.Create(&stages).Error
	})

	if err != nil {
		wc.Logger.Printf("Failed to create warmup schedule: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create warmup schedule",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "warmup schedule created successfully",
		"data":    schedule,
	})
}

// GetWarmupSchedules returns all warmup schedules for a sender
func (wc *WarmupController) GetWarmupSchedules(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var schedules []models.WarmupSchedule
	if err := config.DB.Where("sender_id = ? AND user_id = ?", senderID, userID).
		Preload("Stages").
		Find(&schedules).Error; err != nil {
		wc.Logger.Printf("Database error fetching schedules: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch warmup schedules",
		})
	}

	return c.JSON(fiber.Map{
		"data": schedules,
	})
}

// UpdateWarmupSchedule updates a warmup schedule
func (wc *WarmupController) UpdateWarmupSchedule(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	scheduleID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid schedule ID",
		})
	}

	var input struct {
		Name     *string `json:"name" validate:"omitempty,min=3,max=50"`
		IsActive *bool   `json:"is_active"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidRequestBody,
		})
	}

	var schedule models.WarmupSchedule
	if err := config.DB.Where("id = ? AND user_id = ?", scheduleID, userID).First(&schedule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrScheduleNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching schedule: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no fields to update",
		})
	}

	if err := config.DB.Model(&schedule).Updates(updates).Error; err != nil {
		wc.Logger.Printf("Failed to update schedule: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update schedule",
		})
	}

	return c.JSON(fiber.Map{
		"message": "schedule updated successfully",
		"data":    schedule,
	})
}

// TrackEmailOpen tracks when an email is opened
func (wc *WarmupController) TrackEmailOpen(c *fiber.Ctx) error {
	messageID := c.Query("message_id")
	if messageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrMessageIDRequired,
		})
	}

	if err := config.DB.Model(&models.EmailTracking{}).
		Where("message_id = ?", messageID).
		Update("opened_at", time.Now().UTC()).Error; err != nil {
		wc.Logger.Printf("Failed to track email open: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to track open",
		})
	}

	c.Set("Content-Type", "image/png")
	return c.SendFile("path/to/transparent_pixel.png")
}

// TrackEmailReply handles IMAP webhook for replies
func (wc *WarmupController) TrackEmailReply(c *fiber.Ctx) error {
	var input struct {
		MessageID string `json:"message_id" validate:"required"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidRequestBody,
		})
	}

	if err := config.DB.Model(&models.EmailTracking{}).
		Where("message_id = ?", input.MessageID).
		Update("replied_at", time.Now().UTC()).Error; err != nil {
		wc.Logger.Printf("Failed to track email reply: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to track reply",
		})
	}

	return c.JSON(fiber.Map{
		"message": "reply tracked successfully",
	})
}

// GetWarmupStats returns warmup statistics
func (wc *WarmupController) GetWarmupStats(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid user ID",
		})
	}

	senderID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": ErrInvalidSenderID,
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, userID).First(&sender).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": ErrSenderNotFound,
			})
		}
		wc.Logger.Printf("Database error fetching sender: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	stats := struct {
		TotalSent       int64   `json:"total_sent"`
		TotalOpened     int64   `json:"total_opened"`
		TotalReplied    int64   `json:"total_replied"`
		OpenRate        float64 `json:"open_rate"`
		ReplyRate       float64 `json:"reply_rate"`
		CurrentStage    int     `json:"current_stage"`
		DaysInStage     int     `json:"days_in_stage"`
		EmailsSent      int     `json:"emails_sent"`
		EmailsRemaining int     `json:"emails_remaining"`
	}{}

	// Execute all counts in parallel for better performance
	var wg sync.WaitGroup
	var sentErr, openedErr, repliedErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int64
		sentErr = config.DB.Model(&models.EmailTracking{}).
			Where("sender_id = ? AND is_warmup = ?", senderID, true).
			Count(&count).Error
		stats.TotalSent = count
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int64
		openedErr = config.DB.Model(&models.EmailTracking{}).
			Where("sender_id = ? AND is_warmup = ? AND opened_at IS NOT NULL", senderID, true).
			Count(&count).Error
		stats.TotalOpened = count
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int64
		repliedErr = config.DB.Model(&models.EmailTracking{}).
			Where("sender_id = ? AND is_warmup = ? AND replied_at IS NOT NULL", senderID, true).
			Count(&count).Error
		stats.TotalReplied = count
	}()

	wg.Wait()

	if sentErr != nil || openedErr != nil || repliedErr != nil {
		wc.Logger.Printf("Error fetching warmup stats: sentErr=%v, openedErr=%v, repliedErr=%v", sentErr, openedErr, repliedErr)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch statistics",
		})
	}

	// Calculate rates safely
	if stats.TotalSent > 0 {
		stats.OpenRate = float64(stats.TotalOpened) / float64(stats.TotalSent) * 100
		stats.ReplyRate = float64(stats.TotalReplied) / float64(stats.TotalSent) * 100
	}

	// Get current stage info
	var schedule models.WarmupSchedule
	if err := config.DB.Where("sender_id = ? AND is_active = ?", senderID, true).
		Preload("Stages").
		First(&schedule).Error; err == nil {
		stats.CurrentStage = schedule.CurrentStage
		if schedule.StartedAt != nil {
			stats.DaysInStage = int(time.Since(*schedule.StartedAt).Hours() / 24)
		}

		for _, stage := range schedule.Stages {
			if stage.StageNumber == stats.CurrentStage {
				stats.EmailsRemaining = stage.EmailsPerDay - sender.WarmupSentToday
				break
			}
		}
	}

	stats.EmailsSent = sender.WarmupSentToday

	return c.JSON(fiber.Map{
		"data": stats,
	})
}