package models

import (
    "time"
    "gorm.io/gorm"
)


// Campaign represents an email campaign
type Campaign struct {
	gorm.Model
	UserID uint `gorm:"not null;index" json:"user_id"`
	// SenderID uint `gorm:"not null;index" json:"sender_id"`

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
	Nodes []CampaignNode `json:"nodes" gorm:"type:jsonb;serializer:json"`
	Edges []CampaignEdge `json:"edges" gorm:"type:jsonb;serializer:json"`

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

// CampaignEdge represents connections between nodes
type CampaignEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle"`
	Condition    string `json:"condition"` // for conditional branches
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


// CampaignActivity tracks interactions with a campaign
type CampaignActivity struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	UserID     uint `gorm:"not null;index" json:"user_id"` // Add this line
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



// NodeData contains node-specific data
type NodeData struct {
	// Common fields
	Label string `json:"label"`

	// Email node fields
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
	TemplateID *uint  `json:"template_id,omitempty"`

	// Condition node fields
	OpenedEmailEnabled     bool   `json:"openedEmailEnabled,omitempty"`
	ClickedLinkEnabled     bool   `json:"clickedLinkEnabled,omitempty"`
	OpenedEmailWaitingTime string `json:"openedEmailWaitingTime,omitempty"`
	ClickedLinkWaitingTime string `json:"clickedLinkWaitingTime,omitempty"`

	// Delay node fields
	WaitingTime string `json:"waitingTime,omitempty"`

	// Condition node fields
	ConditionType string `json:"condition_type,omitempty"` // opened, clicked, replied
	MatchValue    string `json:"match_value,omitempty"`    // any, none, specific

	// Delay node fields
	DelayAmount int    `json:"delay_amount,omitempty"`
	DelayUnit   string `json:"delay_unit,omitempty"` // hours, days

	// Goal node fields
	GoalType string `json:"goal_type,omitempty"` // conversion, reply, etc.
}


// CampaignLeadList joins campaigns to lead lists
type CampaignLeadList struct {
	gorm.Model
	CampaignID uint `gorm:"not null;index" json:"campaign_id"`
	LeadListID uint `gorm:"not null;index" json:"lead_list_id"`
}


// In your models package
type CampaignSender struct {
    gorm.Model
    CampaignID uint `gorm:"index"`
    SenderID   uint `gorm:"index"`
}
