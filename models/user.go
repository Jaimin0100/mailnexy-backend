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

	// Relations
	Senders            []Sender            `gorm:"foreignKey:UserID" json:"senders,omitempty"`
	Campaigns          []Campaign          `gorm:"foreignKey:UserID" json:"campaigns,omitempty"`
	LeadLists          []LeadList          `gorm:"foreignKey:UserID" json:"lead_lists,omitempty"`
	EmailVerifications []EmailVerification `gorm:"foreignKey:UserID" json:"email_verifications,omitempty"`
	UserFeatures       []UserFeature       `gorm:"foreignKey:UserID" json:"features,omitempty"`
	Transactions       []CreditTransaction `gorm:"foreignKey:UserID" json:"transactions,omitempty"`
	APIKeys            []APIKey            `gorm:"foreignKey:UserID" json:"api_keys,omitempty"`
}

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
	Campaigns       []Campaign       `gorm:"foreignKey:SenderID" json:"campaigns,omitempty"`
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

// Campaign represents an email campaign
type Campaign struct {
	gorm.Model
	UserID   uint `gorm:"not null;index" json:"user_id"`
	SenderID uint `gorm:"not null;index" json:"sender_id"`

	// Campaign details
	Name        string `gorm:"not null" json:"name"`
	Subject     string `gorm:"not null" json:"subject"`
	Description string `json:"description"`
	PreviewText string `json:"preview_text"`
	ContentRef  string `json:"content_ref"` // Reference to S3/storage for large content

	// Scheduling
	Status      string     `gorm:"default:'draft'" json:"status"` // draft, scheduled, sending, sent, paused, canceled
	ScheduledAt *time.Time `json:"scheduled_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Tracking settings
	TrackOpens      bool `gorm:"default:true" json:"track_opens"`
	TrackClicks     bool `gorm:"default:true" json:"track_clicks"`
	TrackReplies    bool `gorm:"default:true" json:"track_replies"`
	UnsubscribeLink bool `gorm:"default:true" json:"unsubscribe_link"`

	// Statistics (denormalized for performance)
	TotalRecipients  int `gorm:"default:0" json:"total_recipients"`
	SentCount        int `gorm:"default:0" json:"sent_count"`
	OpenCount        int `gorm:"default:0" json:"open_count"`
	UniqueOpenCount  int `gorm:"default:0" json:"unique_open_count"`
	ClickCount       int `gorm:"default:0" json:"click_count"`
	UniqueClickCount int `gorm:"default:0" json:"unique_click_count"`
	ReplyCount       int `gorm:"default:0" json:"reply_count"`
	BounceCount      int `gorm:"default:0" json:"bounce_count"`
	UnsubscribeCount int `gorm:"default:0" json:"unsubscribe_count"`

	// Relations
	CampaignLeadLists []CampaignLeadList `gorm:"foreignKey:CampaignID" json:"lead_lists,omitempty"`
	Activities        []CampaignActivity `gorm:"foreignKey:CampaignID" json:"activities,omitempty"`
	Flows             []CampaignFlow     `gorm:"foreignKey:CampaignID" json:"flows,omitempty"`
}

// CampaignFlow represents the flowchart/nodes structure of a campaign
type CampaignFlow struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	UserID     uint `gorm:"not null;index" json:"user_id"`

	// Flow structure stored as JSON
	Nodes []CampaignNode `gorm:"type:jsonb" json:"nodes"`
	Edges []CampaignEdge `gorm:"type:jsonb" json:"edges"`

	// Status
	IsActive    bool `gorm:"default:false" json:"is_active"`
	CurrentStep int  `gorm:"default:0" json:"current_step"`

	// Relations
	Campaign  Campaign           `json:"-"`
	User      User               `json:"-"`
	Execution *CampaignExecution `gorm:"foreignKey:FlowID" json:"execution,omitempty"`
}

// CampaignNode represents a node in the campaign flowchart
type CampaignNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // email, condition, delay, goal
	Position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
	Data NodeData `json:"data"`
}

// NodeData contains node-specific data
type NodeData struct {
	// Common fields
	Label string `json:"label"`

	// Email node fields
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
	TemplateID *uint  `json:"template_id,omitempty"`

	// Condition node fields
	ConditionType string `json:"condition_type,omitempty"` // opened, clicked, replied
	MatchValue    string `json:"match_value,omitempty"`    // any, none, specific

	// Delay node fields
	DelayAmount int    `json:"delay_amount,omitempty"`
	DelayUnit   string `json:"delay_unit,omitempty"` // hours, days

	// Goal node fields
	GoalType string `json:"goal_type,omitempty"` // conversion, reply, etc.
}

// CampaignEdge represents connections between nodes
type CampaignEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Condition string `json:"condition,omitempty"` // for conditional branches
}

// CampaignExecution tracks the current state of campaign execution
type CampaignExecution struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	FlowID     uint `gorm:"not null;index" json:"flow_id"`

	// Current state
	CurrentNodeID string     `json:"current_node_id"`
	NextRunAt     *time.Time `json:"next_run_at"`

	// Statistics
	EmailsSent int `gorm:"default:0" json:"emails_sent"`
	Replies    int `gorm:"default:0" json:"replies"`
	Opens      int `gorm:"default:0" json:"opens"`
	Clicks     int `gorm:"default:0" json:"clicks"`

	// Relations
	Campaign Campaign     `json:"-"`
	Flow     CampaignFlow `json:"-"`
}

// CampaignLeadList joins campaigns to lead lists
type CampaignLeadList struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	LeadListID uint `gorm:"not null;index" json:"lead_list_id"`
}

// LeadList represents a list of leads/contacts
type LeadList struct {
	gorm.Model
	UserID uint `gorm:"not null;index" json:"user_id"`

	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`                         // manual, csv, api, etc.
	IsPublic    bool   `gorm:"default:false" json:"is_public"` // For team sharing

	// Statistics
	LeadCount    int `gorm:"default:0" json:"lead_count"`
	ActiveCount  int `gorm:"default:0" json:"active_count"`
	BouncedCount int `gorm:"default:0" json:"bounced_count"`

	// Relations
	LeadListMemberships []LeadListMembership `gorm:"foreignKey:LeadListID" json:"memberships,omitempty"`
}

// Lead represents a single contact/lead
type Lead struct {
	gorm.Model
	Email     string `gorm:"not null;index" json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Company   string `json:"company"`
	Position  string `json:"position"`
	Phone     string `json:"phone"`
	Website   string `json:"website"`

	// Status
	IsVerified     bool `gorm:"default:false" json:"is_verified"`
	IsBounced      bool `gorm:"default:false" json:"is_bounced"`
	IsUnsubscribed bool `gorm:"default:false" json:"is_unsubscribed"`
	IsDoNotContact bool `gorm:"default:false" json:"is_do_not_contact"`

	// Metadata
	Source      string     `json:"source"`
	LastContact *time.Time `json:"last_contact"`
	UserID      uint       `gorm:"index" json:"user_id"`

	// Relations
	LeadListMemberships []LeadListMembership `gorm:"foreignKey:LeadID" json:"lists,omitempty"`
	LeadTags            []LeadTag            `gorm:"foreignKey:LeadID" json:"tags,omitempty"`
	CustomFields        []LeadCustomField    `gorm:"foreignKey:LeadID" json:"custom_fields,omitempty"`
	Activities          []LeadActivity       `gorm:"foreignKey:LeadID" json:"activities,omitempty"`
}

// LeadListMembership joins leads to lists
type LeadListMembership struct {
	gorm.Model
	LeadID     uint `gorm:"not null;index" json:"lead_id"`
	LeadListID uint `gorm:"not null;index" json:"lead_list_id"`
}

// LeadTag represents tags for leads (normalized)
type LeadTag struct {
	gorm.Model
	LeadID uint   `gorm:"not null;index" json:"lead_id"`
	Tag    string `gorm:"not null;index" json:"tag"`
}

// LeadCustomField represents custom fields for leads
type LeadCustomField struct {
	gorm.Model
	LeadID uint   `gorm:"not null;index" json:"lead_id"`
	Name   string `gorm:"not null;index" json:"name"`
	Value  string `gorm:"type:text" json:"value"`
}

// CampaignActivity tracks interactions with a campaign
type CampaignActivity struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	LeadID     uint `gorm:"not null;index" json:"lead_id"`

	// Activity types
	SentAt         *time.Time `json:"sent_at"`
	OpenedAt       *time.Time `json:"opened_at"`
	OpenCount      int        `gorm:"default:0" json:"open_count"`
	ClickedAt      *time.Time `json:"clicked_at"`
	ClickCount     int        `gorm:"default:0" json:"click_count"`
	RepliedAt      *time.Time `json:"replied_at"`
	BouncedAt      *time.Time `json:"bounced_at"`
	BounceType     string     `json:"bounce_type"` // hard, soft, block, etc.
	UnsubscribedAt *time.Time `json:"unsubscribed_at"`

	// Device and location info
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	Location   string `json:"location"`
	DeviceType string `json:"device_type"` // desktop, mobile, tablet
	SenderID   uint   `gorm:"not null;index" json:"sender_id"`
	MessageID  string `json:"message_id"`

	// Relations
	Campaign    Campaign     `json:"-"`
	Lead        Lead         `json:"-"`
	ClickEvents []ClickEvent `gorm:"foreignKey:ActivityID" json:"click_events,omitempty"`
}

// ClickEvent tracks individual click events (normalized from JSON array)
type ClickEvent struct {
	gorm.Model
	ActivityID uint      `gorm:"not null;index" json:"activity_id"`
	URL        string    `gorm:"not null" json:"url"`
	ClickedAt  time.Time `gorm:"not null" json:"clicked_at"`
	Count      int       `gorm:"default:1" json:"count"`
}

// LeadActivity tracks all activities for a lead across campaigns
type LeadActivity struct {
	gorm.Model
	LeadID     uint  `gorm:"not null;index" json:"lead_id"`
	CampaignID *uint `json:"campaign_id,omitempty"`
	SenderID   *uint `json:"sender_id,omitempty"`

	ActivityType string    `gorm:"not null" json:"activity_type"` // sent, opened, clicked, replied, bounced, etc.
	ActivityAt   time.Time `gorm:"not null" json:"activity_at"`
	Details      string    `gorm:"type:text" json:"details"` // JSON details if needed

	// Relations
	Lead     Lead      `json:"-"`
	Campaign *Campaign `json:"campaign,omitempty"`
	Sender   *Sender   `json:"sender,omitempty"`
}

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

// Team represents user teams for collaboration
type Team struct {
	gorm.Model
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`

	// Relations
	Members []TeamMember `gorm:"foreignKey:TeamID" json:"members,omitempty"`
}

// TeamMember represents team members and their roles
type TeamMember struct {
	gorm.Model
	TeamID uint `gorm:"not null;index" json:"team_id"`
	UserID uint `gorm:"not null;index" json:"user_id"`

	Role    string `gorm:"default:'member'" json:"role"` // owner, admin, member
	CanSend bool   `gorm:"default:true" json:"can_send"`
	CanEdit bool   `gorm:"default:false" json:"can_edit"`

	// Relations
	Team Team `json:"-"`
	User User `json:"-"`
}

// Initialize default plans in your database migration
func CreateDefaultPlans(db *gorm.DB) error {
	defaultPlans := []Plan{
		{
			Name:           "free",
			Description:    "Free starter plan with 5,000 email credits",
			EmailCredits:   5000,
			EmailPrice:     0,
			VerifyCredits:  0,
			VerifyPrice:    0,
			WarmupEnabled:  true,
			MaxSenders:     1,
			DailySendLimit: 500,
		},
		{
			Name:           "starter",
			Description:    "Starter plan with 20,000 email and verification credits",
			EmailCredits:   20000,
			EmailPrice:     2000, // $20
			VerifyCredits:  20000,
			VerifyPrice:    2000, // $20
			WarmupEnabled:  true,
			MaxSenders:     3,
			DailySendLimit: 1000,
			DisplayPrice:   "$20",
		},
		{
			Name:           "grow",
			Description:    "Growth plan with 100,000 email and verification credits",
			EmailCredits:   100000,
			EmailPrice:     6000, // $60
			VerifyCredits:  100000,
			VerifyPrice:    6000, // $60
			WarmupEnabled:  true,
			MaxSenders:     10,
			DailySendLimit: 5000,
			DisplayPrice:   "$60",
			IsPopular:      true,
			Recommended:    true,
		},
		{
			Name:           "enterprise",
			Description:    "Custom plan for high-volume senders",
			EmailCredits:   500000,
			EmailPrice:     20000, // $200
			VerifyCredits:  500000,
			VerifyPrice:    20000, // $200
			WarmupEnabled:  true,
			MaxSenders:     50,
			DailySendLimit: 20000,
			DisplayPrice:   "$200",
			CustomDomain:   true,
		},
	}
	for _, plan := range defaultPlans {
		if err := db.FirstOrCreate(&plan, "name = ?", plan.Name).Error; err != nil {
			return err
		}
	}
	return nil
}

// UserFeature represents features enabled for a specific user
type UserFeature struct {
	gorm.Model
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	FeatureID uint       `gorm:"not null" json:"feature_id"`
	Name      string     `gorm:"not null" json:"name"`
	Enabled   bool       `gorm:"default:true" json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Relations
	User User `json:"-"`
}

// Feature represents system features that can be enabled for users
type Feature struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `gorm:"default:false" json:"is_default"`
	PlanLevel   string `json:"plan_level"` // minimum plan level that gets this feature
}