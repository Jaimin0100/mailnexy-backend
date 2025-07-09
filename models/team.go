package models

import "gorm.io/gorm"


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
