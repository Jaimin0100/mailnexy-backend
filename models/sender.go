package models

import (
    "time"
    "gorm.io/gorm"
)


// Sender represents email sending and receiving credentials
type Sender struct {
	gorm.Model
	UserID uint `gorm:"not null;index" json:"user_id"`

	// Basic identification
	Name      string `gorm:"not null" json:"name"`
	FromEmail string `gorm:"not null" json:"from_email"`
	FromName  string `gorm:"not null" json:"from_name"`

	// Connection Type
	ProviderType string `gorm:"not null" json:"provider_type"` // smtp, gmail, outlook, yahoo, etc.

	// ========= SMTP Configuration =========
	SMTPHost     string `gorm:"not null" json:"smtp_host"`
	SMTPPort     int    `gorm:"not null" json:"smtp_port"`
	SMTPUsername string `gorm:"not null" json:"smtp_username"`
	SMTPPassword string `gorm:"not null" json:"-"`          // Encrypted in application layer
	Encryption   string `gorm:"not null" json:"encryption"` // SSL, TLS, STARTTLS

	// ========= IMAP Configuration =========
	IMAPHost       string `json:"imap_host"`
	IMAPPort       int    `json:"imap_port" gorm:"default:993"`
	IMAPUsername   string `json:"imap_username"`
	IMAPPassword   string `json:"-"` // Encrypted in application layer
	IMAPEncryption string `json:"imap_encryption" gorm:"default:'SSL'"`
	IMAPMailbox    string `json:"imap_mailbox" gorm:"default:'INBOX'"`

	// ========= OAuth Configuration =========
	OAuthProvider     string    `gorm:"column:oauth_provider" json:"oauth_provider"` // google, microsoft, etc.
	OAuthToken        string    `gorm:"column:oauth_token" json:"-"`                 // Encrypted
	OAuthRefreshToken string    `gorm:"column:oauth_refresh_token" json:"-"`         // Encrypted
	OAuthExpiry       time.Time `gorm:"column:oauth_expiry" json:"oauth_expiry"`

	// ========= Warmup Configuration =========
	IsWarmingUp       bool       `gorm:"default:false" json:"is_warming_up"`
	WarmupStartedAt   *time.Time `json:"warmup_started_at"`
	WarmupCompletedAt *time.Time `json:"warmup_completed_at"`
	WarmupSentToday   int        `gorm:"default:0" json:"warmup_sent_today"`
	WarmupStage       int        `gorm:"default:0" json:"warmup_stage"`

	// ========= Tracking Settings =========
	TrackOpens           bool   `gorm:"default:true" json:"track_opens"`
	TrackClicks          bool   `gorm:"default:true" json:"track_clicks"`
	TrackReplies         bool   `gorm:"default:true" json:"track_replies"`
	CustomTrackingDomain string `json:"custom_tracking_domain"`

	// ========= Email Authentication =========
	DKIMPrivateKey string `json:"dkim_private_key"` // Encrypted in application layer
	DKIMSelector   string `json:"dkim_selector"`
	DMARCPolicy    string `json:"dmarc_policy"`
	SPFRecord      string `json:"spf_record"`

	// ========= Status & Verification =========
	SMTPVerified bool       `json:"smtp_verified" gorm:"default:false"`
	IMAPVerified bool       `json:"imap_verified" gorm:"default:false"`
	LastTestedAt *time.Time `json:"last_tested_at"`
	LastError    *string    `json:"last_error"`

	// ========= Usage Metrics =========
	DailyLimit int     `gorm:"default:500" json:"daily_limit"`
	SentToday  int     `gorm:"default:0" json:"sent_today"`
	TotalSent  int     `gorm:"default:0" json:"total_sent"`
	ReplyCount int     `gorm:"default:0" json:"reply_count"`
	OpenRate   float64 `gorm:"default:0" json:"open_rate"`
	ClickRate  float64 `gorm:"default:0" json:"click_rate"`
	BounceRate float64 `gorm:"default:0" json:"bounce_rate"`

	// Relations
	WarmupSchedules []WarmupSchedule `gorm:"foreignKey:SenderID" json:"warmup_schedules,omitempty"`
	// Campaigns       []Campaign       `gorm:"foreignKey:SenderID" json:"campaigns,omitempty"`
}


// Add this method to your Sender model
func (s *Sender) Sanitize() {
	s.SMTPPassword = ""
	s.IMAPPassword = ""
	s.OAuthToken = ""
	s.OAuthRefreshToken = ""
	s.DKIMPrivateKey = ""
}


// WarmupSchedule represents structured warmup configuration
type WarmupSchedule struct {
	gorm.Model
	UserID   uint `gorm:"not null;index" json:"user_id"`
	SenderID uint `gorm:"not null;index" json:"sender_id"`

	Name     string `gorm:"not null" json:"name"`
	IsActive bool   `gorm:"default:true" json:"is_active"`

	// Progress tracking
	CurrentStage int `gorm:"default:0" json:"current_stage"`
	EmailsSent   int `gorm:"default:0" json:"emails_sent"`
	TotalStages  int `gorm:"default:0" json:"total_stages"`

	// Dates
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Stages (normalized instead of JSON blob)
	Stages []WarmupStage `gorm:"foreignKey:ScheduleID" json:"stages"`
}

// WarmupStage represents individual warmup stages
type WarmupStage struct {
	gorm.Model
	ScheduleID uint `gorm:"not null;index" json:"schedule_id"`

	StageNumber  int `gorm:"not null" json:"stage_number"`
	EmailsPerDay int `gorm:"not null" json:"emails_per_day"`
	DurationDays int `gorm:"not null" json:"duration_days"`
	ReplyTarget  int `gorm:"default:5" json:"reply_target"` // Target reply percentage to advance
}


// EmailTracking tracks email engagement
type EmailTracking struct {
	gorm.Model
	SenderID  uint       `gorm:"not null;index" json:"sender_id"`
	Recipient string     `gorm:"not null" json:"recipient"`
	Subject   string     `gorm:"not null" json:"subject"`
	MessageID string     `gorm:"not null;uniqueIndex" json:"message_id"`
	OpenedAt  *time.Time `json:"opened_at"`
	ClickedAt *time.Time `json:"clicked_at"`
	RepliedAt *time.Time `json:"replied_at"`
	BouncedAt *time.Time `json:"bounced_at"`
	IsWarmup  bool       `gorm:"default:false" json:"is_warmup"`
}
