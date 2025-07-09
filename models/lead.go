package models

import (
    "time"
    "gorm.io/gorm"
)


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
	Leads               []Lead               `gorm:"foreignKey:LeadListID" json:"leads"`
}


// Lead represents a single contact/lead
type Lead struct {
	gorm.Model
	// Foreign key to LeadList - REQUIRED for creation
	LeadListID uint `gorm:"not null;index" json:"lead_list_id"` // Ensures lead belongs to a list

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
	LeadList            LeadList             `gorm:"foreignKey:LeadListID" json:"lead_list"`
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
