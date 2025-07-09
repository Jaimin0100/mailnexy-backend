package models

import "gorm.io/gorm"


// Sequence represents automated email sequences
type Sequence struct {
	gorm.Model
	UserID   uint `gorm:"not null;index" json:"user_id"`
	SenderID uint `gorm:"not null;index" json:"sender_id"`

	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Status      string `gorm:"default:'draft'" json:"status"` // draft, active, paused

	// Settings
	MaxEmailsPerDay int `gorm:"default:100" json:"max_emails_per_day"`
	SendInterval    int `gorm:"default:2" json:"send_interval"` // Days between emails

	// Relations
	Steps []SequenceStep `gorm:"foreignKey:SequenceID" json:"steps,omitempty"`
}


// SequenceStep represents steps in an email sequence
type SequenceStep struct {
	gorm.Model
	SequenceID uint `gorm:"not null;index" json:"sequence_id"`
	TemplateID uint `gorm:"not null;index" json:"template_id"`

	StepNumber int `gorm:"not null" json:"step_number"`
	DelayDays  int `gorm:"not null" json:"delay_days"`

	// Tracking
	SentCount int     `gorm:"default:0" json:"sent_count"`
	OpenRate  float64 `gorm:"default:0" json:"open_rate"`
	ReplyRate float64 `gorm:"default:0" json:"reply_rate"`

	// Relations
	Template Template `json:"-"`
}