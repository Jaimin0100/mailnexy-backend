package models

import (
	"time"

	"gorm.io/gorm"
)

// UniboxEmail represents an email in the unified inbox
type UniboxEmail struct {
	gorm.Model
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	SenderID    uint      `gorm:"not null;index" json:"sender_id"`
	MessageID   string    `gorm:"not null;index" json:"message_id"`
	ThreadID    string    `gorm:"index" json:"thread_id"`
	From        string    `gorm:"not null" json:"from"`
	To          string    `gorm:"not null" json:"to"`
	Subject     string    `json:"subject"`
	Body        string    `gorm:"type:text" json:"body"`
	BodyHTML    string    `gorm:"type:text" json:"body_html"`
	Date        time.Time `gorm:"not null" json:"date"`
	IsRead      bool      `gorm:"default:false" json:"is_read"`
	IsStarred   bool      `gorm:"default:false" json:"is_starred"`
	IsImportant bool      `gorm:"default:false" json:"is_important"`
	Labels      []string  `gorm:"type:text[]" json:"labels"`
	Attachments []string  `gorm:"type:text[]" json:"attachments"`
	InReplyTo   string    `json:"in_reply_to"`
	References  string    `json:"references"`
	Size        int       `json:"size"`

	// Relations
	User   User   `json:"-"`
	Sender Sender `json:"sender"`
}


// UniboxFolder represents a folder/category in the unified inbox
type UniboxFolder struct {
	gorm.Model
	UserID uint   `gorm:"not null;index" json:"user_id"`
	Name   string `gorm:"not null" json:"name"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
	System bool   `gorm:"default:false" json:"system"` // System folders can't be deleted

	// Relations
	User User `json:"-"`
}



// UniboxEmailFolder joins emails to folders
type UniboxEmailFolder struct {
	gorm.Model
	EmailID  uint `gorm:"not null;index" json:"email_id"`
	FolderID uint `gorm:"not null;index" json:"folder_id"`

	// Relations
	Email  UniboxEmail  `json:"-"`
	Folder UniboxFolder `json:"-"`
}
