package controller

import (
	"github.com/gofiber/fiber/v2"
	"mailnexy/models"
)

// DeleteCampaign deletes a campaign
func (cc *CampaignController) DeleteCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid campaign ID",
		})
	}

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	// Start transaction
	tx := cc.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete in proper order to respect foreign keys
	tables := []interface{}{
		&models.CampaignActivity{},
		&models.CampaignExecution{},
		&models.CampaignLeadList{},
		&models.CampaignFlow{},
	}

	for _, table := range tables {
		if err := tx.Where("campaign_id = ?", campaign.ID).Delete(table).Error; err != nil {
			tx.Rollback()
			cc.Logger.Printf("Failed to delete related records: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete campaign dependencies",
			})
		}
	}

	// Finally delete campaign
	if err := tx.Delete(&campaign).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to delete campaign: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete campaign",
		})
	}

	if err := tx.Commit().Error; err != nil {
		cc.Logger.Printf("Transaction commit failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete deletion",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Campaign deleted successfully",
	})
}

// Helper functions
func isValidToken(messageID, token string) bool {
	// Implement your token validation logic
	// Compare with generated token using same algorithm
	return true
}
