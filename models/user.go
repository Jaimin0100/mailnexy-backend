package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user account in the system
type User struct {
	gorm.Model

	// Authentication fields
	Email         string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash  string `gorm:"not null" json:"-"`
	EmailVerified bool   `gorm:"default:false" json:"email_verified"`
	OTP           string
	OTPExpiresAt  time.Time
	OTPVerified   bool `gorm:"default:false"`

	// Google OAuth fields
	GoogleID       *string `gorm:"uniqueIndex" json:"google_id,omitempty"`
	GoogleImageURL *string `json:"google_image_url,omitempty"`

	// Profile information
	Name      *string `json:"name,omitempty"`
	Company   *string `json:"company,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Timezone  string  `gorm:"default:'UTC'" json:"timezone"`
	Language  string  `gorm:"default:'en'" json:"language"`

	// Account status
	IsActive bool `gorm:"default:true" json:"is_active"`
	IsAdmin  bool `gorm:"default:false" json:"is_admin"`

	// Credit-based plan information
	PlanID          *uint      `json:"plan_id,omitempty"`
	PlanName        string     `gorm:"default:'free'" json:"plan_name"`   // free, starter, grow, enterprise
	EmailCredits    int        `gorm:"default:5000" json:"email_credits"` // 5000 free credits for new users
	VerifyCredits   int        `gorm:"default:0" json:"verify_credits"`
	CreditsEarned   int        `gorm:"default:0" json:"credits_earned"` // For referral programs
	CreditsConsumed int        `gorm:"default:0" json:"credits_consumed"`
	LastCreditReset *time.Time `json:"last_credit_reset"` // For monthly credit resets if needed

	// Stripe integration
	StripeCustomerID    *string `gorm:"index" json:"stripe_customer_id,omitempty"`
	StripePaymentMethod *string `json:"stripe_payment_method,omitempty"`
	DefaultCurrency     string  `gorm:"default:'usd'" json:"default_currency"`
	TokenVersion        uint    `gorm:"default:0" json:"-"`
	ResetToken          *string `gorm:"size:255"`
	ResetTokenExpiresAt *time.Time

	// Relations
	Senders            []Sender            `gorm:"foreignKey:UserID" json:"senders,omitempty"`
	Campaigns          []Campaign          `gorm:"foreignKey:UserID" json:"campaigns,omitempty"`
	LeadLists          []LeadList          `gorm:"foreignKey:UserID" json:"lead_lists,omitempty"`
	EmailVerifications []EmailVerification `gorm:"foreignKey:UserID" json:"email_verifications,omitempty"`
	UserFeatures       []UserFeature       `gorm:"foreignKey:UserID" json:"features,omitempty"`
	Transactions       []CreditTransaction `gorm:"foreignKey:UserID" json:"transactions,omitempty"`
	APIKeys            []APIKey            `gorm:"foreignKey:UserID" json:"api_keys,omitempty"`
}

type RefreshToken struct {
	gorm.Model
	UserID    uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"not null"`
	SessionID string    `gorm:"index;not null"`
	UserAgent string    `gorm:"size:512"`
	IPAddress string    `gorm:"size:45"` // Supports IPv6
	ExpiresAt time.Time `gorm:"not null"`
	IsRevoked bool      `gorm:"default:false;not null"`
}



// APIKey represents user API keys for integration
type APIKey struct {
	gorm.Model
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Key       string     `gorm:"uniqueIndex;not null" json:"key"`
	Name      string     `gorm:"not null" json:"name"`
	LastUsed  *time.Time `json:"last_used"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`

	// Permissions
	CanSendEmails   bool `gorm:"default:true" json:"can_send_emails"`
	CanVerifyEmails bool `gorm:"default:true" json:"can_verify_emails"`
	CanManageLists  bool `gorm:"default:false" json:"can_manage_lists"`

	// Relations
	User User `json:"-"`
}


