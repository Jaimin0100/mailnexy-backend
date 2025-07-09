package models

import "gorm.io/gorm"


// Plan represents available credit packages
type Plan struct {
	gorm.Model
	Name        string `gorm:"not null;uniqueIndex" json:"name"` // free, starter, grow, enterprise
	Description string `json:"description"`

	// Email credits
	EmailCredits int `gorm:"not null" json:"email_credits"`
	EmailPrice   int `gorm:"not null" json:"email_price"` // in cents

	// Verification credits
	VerifyCredits int `gorm:"not null" json:"verify_credits"`
	VerifyPrice   int `gorm:"not null" json:"verify_price"` // in cents

	// Features
	WarmupEnabled   bool `gorm:"default:true" json:"warmup_enabled"`
	TrackingEnabled bool `gorm:"default:true" json:"tracking_enabled"`
	MaxSenders      int  `gorm:"default:1" json:"max_senders"`
	DailySendLimit  int  `gorm:"default:500" json:"daily_send_limit"`
	CustomDomain    bool `gorm:"default:false" json:"custom_domain"`

	// For display purposes
	DisplayPrice string `gorm:"-" json:"display_price"` // e.g. "$20"
	IsPopular    bool   `gorm:"default:false" json:"is_popular"`
	Recommended  bool   `gorm:"default:false" json:"recommended"`

	StripePriceID   string `json:"stripe_price_id"`                            // price_xxx from Stripe dashboard
	BillingInterval string `json:"billing_interval" gorm:"default:'one_time'"` // one_time, monthly, yearly
}


// CreditTransaction records credit purchases and usage
type CreditTransaction struct {
	gorm.Model
	UserID uint  `gorm:"not null;index" json:"user_id"`
	PlanID *uint `json:"plan_id,omitempty"`

	// Credit changes
	EmailCredits  int `gorm:"not null" json:"email_credits"` // Positive for purchases, negative for usage
	VerifyCredits int `gorm:"not null" json:"verify_credits"`

	// Financial information
	Amount        int    `json:"amount"` // in cents
	Currency      string `gorm:"default:'USD'" json:"currency"`
	PaymentMethod string `json:"payment_method"`
	PaymentStatus string `gorm:"default:'pending'" json:"payment_status"` // pending, completed, failed, refunded

	// Metadata
	Description   string `json:"description"`
	ReferenceID   string `json:"reference_id"` // For payment processors
	InvoiceNumber string `json:"invoice_number"`

	StripePaymentIntentID string `json:"stripe_payment_intent_id"`
	StripeChargeID        string `json:"stripe_charge_id"`
	StripeInvoiceID       string `json:"stripe_invoice_id,omitempty"`
	ReceiptURL            string `json:"receipt_url,omitempty"`

	// Relations
	User User  `json:"-"`
	Plan *Plan `json:"plan,omitempty"`
}


// CreditUsage tracks detailed credit consumption
type CreditUsage struct {
	gorm.Model
	UserID         uint  `gorm:"not null;index" json:"user_id"`
	TransactionID  *uint `json:"transaction_id,omitempty"`
	CampaignID     *uint `json:"campaign_id,omitempty"`
	VerificationID *uint `json:"verification_id,omitempty"`
	SenderID       *uint `json:"sender_id,omitempty"`

	// Usage details
	CreditType  string `gorm:"not null" json:"credit_type"` // email or verify
	Amount      int    `gorm:"not null" json:"amount"`      // Always positive
	Action      string `gorm:"not null" json:"action"`      // send_email, verify_email, followup, etc.
	TargetEmail string `json:"target_email,omitempty"`
	IsFollowUp  bool   `gorm:"default:false" json:"is_follow_up"` // For free followup emails

	// Relations
	User         User               `json:"-"`
	Transaction  *CreditTransaction `json:"transaction,omitempty"`
	Campaign     *Campaign          `json:"campaign,omitempty"`
	Verification *EmailVerification `json:"verification,omitempty"`
	Sender       *Sender            `json:"sender,omitempty"`
}
