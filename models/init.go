package models

import "gorm.io/gorm"

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
