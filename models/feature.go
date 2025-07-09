package models

import (
	"time"

	"gorm.io/gorm"
)

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
