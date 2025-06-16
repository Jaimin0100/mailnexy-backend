package utils

import (
	"errors"
	"log"
	"time"

	"mailnexy/models"
	"gorm.io/gorm"
)

type CampaignSender struct {
	DB     *gorm.DB
	Logger *log.Logger
}
type MailServiceInterface interface {
    Send(email Email) (string, error)
}

type Email struct {
    From    string
    To      string
    Subject string
    Body    string
}

func NewCampaignSender(db *gorm.DB, logger *log.Logger) *CampaignSender {
	return &CampaignSender{
		DB:     db,
		Logger: logger,
	}
}

// RotateSender selects the next available sender for a campaign
func (cs *CampaignSender) RotateSender(userID uint) (*models.Sender, error) {
	var senders []models.Sender
	if err := cs.DB.Where("user_id = ? AND is_active = ?", userID, true).Find(&senders).Error; err != nil {
		return nil, err
	}

	if len(senders) == 0 {
		return nil, errors.New("no active senders available")
	}

	// Find sender with most available capacity today
	var bestSender *models.Sender
	maxAvailable := 0

	for i := range senders {
		available := senders[i].DailyLimit - senders[i].SentToday
		if available > maxAvailable {
			maxAvailable = available
			bestSender = &senders[i]
		}
	}

	if bestSender == nil || maxAvailable <= 0 {
		return nil, errors.New("no senders with available capacity")
	}

	return bestSender, nil
}

// UpdateSenderUsage increments the sender's usage count
func (cs *CampaignSender) UpdateSenderUsage(senderID uint) error {
	return cs.DB.Model(&models.Sender{}).
		Where("id = ?", senderID).
		Update("sent_today", gorm.Expr("sent_today + ?", 1)).
		Error
}

// ResetDailyCounters resets all sender counters at midnight
func (cs *CampaignSender) ResetDailyCounters() {
	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		time.Sleep(time.Until(nextMidnight))

		if err := cs.DB.Model(&models.Sender{}).
			Where("sent_today > 0").
			Update("sent_today", 0).
			Error; err != nil {
			cs.Logger.Printf("Failed to reset sender counters: %v", err)
		} else {
			cs.Logger.Println("Successfully reset sender daily counters")
		}
	}
}