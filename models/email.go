package models

import (
    "time"
    "gorm.io/gorm"
)


// EmailVerification represents email verification tasks
type EmailVerification struct {
	gorm.Model
	UserID uint `gorm:"not null;index" json:"user_id"`

	// Verification parameters
	Name        string     `json:"name"`
	Status      string     `gorm:"default:'pending'" json:"status"` // pending, processing, completed, failed
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Results
	ValidCount      int `gorm:"default:0" json:"valid_count"`
	InvalidCount    int `gorm:"default:0" json:"invalid_count"`
	DisposableCount int `gorm:"default:0" json:"disposable_count"`
	CatchAllCount   int `gorm:"default:0" json:"catch_all_count"`
	UnknownCount    int `gorm:"default:0" json:"unknown_count"`

	// Relations
	VerificationResults []VerificationResult `gorm:"foreignKey:VerificationID" json:"results"`
}


// VerificationResult stores individual email verification results
type VerificationResult struct {
	gorm.Model
	VerificationID uint   `gorm:"not null;index" json:"verification_id"`
	Email          string `gorm:"not null" json:"email"`
	Status         string `gorm:"not null" json:"status"` // valid, invalid, disposable, catch-all, unknown
	Details        string `json:"details"`                // JSON with detailed verification info
	IsReachable    bool   `gorm:"default:false" json:"is_reachable"`
	IsBounceRisk   bool   `gorm:"default:false" json:"is_bounce_risk"`
}


// Template represents email templates for campaigns
type Template struct {
	gorm.Model
	UserID uint `gorm:"not null;index" json:"user_id"`

	Name        string `gorm:"not null" json:"name"`
	Subject     string `gorm:"not null" json:"subject"`
	HTMLContent string `gorm:"type:text" json:"html_content"`
	TextContent string `gorm:"type:text" json:"text_content"`
	Thumbnail   string `json:"thumbnail"` // Base64 or URL

	// Category
	Category string `json:"category"`
	IsPublic bool   `gorm:"default:false" json:"is_public"` // For template marketplace

	// Relations
	User User `json:"-"`
}


// Unsubscribe represents unsubscribe requests
type Unsubscribe struct {
	gorm.Model
	Email      string `gorm:"not null;index" json:"email"`
	CampaignID *uint  `json:"campaign_id,omitempty"`
	SenderID   *uint  `json:"sender_id,omitempty"`
	UserID     *uint  `json:"user_id,omitempty"`

	Reason    string `json:"reason"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`

	// Relations
	Campaign *Campaign `json:"campaign,omitempty"`
	Sender   *Sender   `json:"sender,omitempty"`
	User     *User     `json:"user,omitempty"`
}


// Bounce represents email bounce records
type Bounce struct {
	gorm.Model
	Email      string `gorm:"not null;index" json:"email"`
	CampaignID *uint  `json:"campaign_id,omitempty"`
	SenderID   uint   `gorm:"not null;index" json:"sender_id"`

	Type           string `gorm:"not null" json:"type"` // hard, soft, block, etc.
	Code           string `json:"code"`
	Message        string `json:"message"`
	DiagnosticCode string `json:"diagnostic_code"`

	// Relations
	Campaign *Campaign `json:"campaign,omitempty"`
	Sender   Sender    `json:"-"`
}