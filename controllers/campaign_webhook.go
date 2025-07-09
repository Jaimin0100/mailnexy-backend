package controller

import (
	"time"

	"mailnexy/models"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// HandleCampaignWebhook processes events (opens, clicks, replies) for campaigns
func (cc *CampaignController) HandleCampaignWebhook(c *fiber.Ctx) error {
	var input struct {
		EventType string `json:"event_type"` // open, click, reply
		MessageID string `json:"message_id"`
		Email     string `json:"email"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find the activity record for this message
	var activity models.CampaignActivity
	if err := cc.DB.Where("message_id = ?", input.MessageID).First(&activity).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Activity not found",
		})
	}

	// Update activity based on event type
	switch input.EventType {
	case "open":
		if activity.OpenedAt == nil {
			activity.OpenedAt = utils.Pointer(time.Unix(input.Timestamp, 0))
		}
		activity.OpenCount++
	case "click":
		if activity.ClickedAt == nil {
			activity.ClickedAt = utils.Pointer(time.Unix(input.Timestamp, 0))
		}
		activity.ClickCount++
	case "reply":
		if activity.RepliedAt == nil {
			activity.RepliedAt = utils.Pointer(time.Unix(input.Timestamp, 0))
		}
	}

	if err := cc.DB.Save(&activity).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update activity",
		})
	}

	// Update campaign execution if this affects a condition node
	var execution models.CampaignExecution
	if err := cc.DB.Where("campaign_id = ?", activity.CampaignID).First(&execution).Error; err == nil {
		var flow models.CampaignFlow
		if err := cc.DB.First(&flow, execution.FlowID).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch flow",
			})
		}

		// Find current node
		var currentNode *models.CampaignNode
		for _, node := range flow.Nodes {
			if node.ID == execution.CurrentNodeID {
				currentNode = &node
				break
			}
		}

		if currentNode != nil && currentNode.Type == "condition" {
			// Check if condition is met
			conditionMet := false
			switch currentNode.Data.ConditionType {
			case "opened":
				conditionMet = activity.OpenCount > 0
			case "clicked":
				conditionMet = activity.ClickCount > 0
			case "replied":
				conditionMet = activity.RepliedAt != nil
			}

			if conditionMet {
				nextNodeID := cc.getNextNodeID(flow, currentNode.ID, "true")
				if nextNodeID != "" {
					execution.CurrentNodeID = nextNodeID
					execution.NextRunAt = utils.Pointer(time.Now())
					if err := cc.DB.Save(&execution).Error; err != nil {
						cc.Logger.Printf("Failed to update execution: %v", err)
					}
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"message": "Webhook processed successfully",
	})
}

// Add these to CampaignController
func (cc *CampaignController) HandleOpenTracking(c *fiber.Ctx) error {
	messageID := c.Params("messageID")
	token := c.Params("token")

	// Validate token
	if !isValidToken(messageID, token) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid token")
	}

	// Update open stats in database
	cc.updateOpenStats(messageID)

	// Return transparent pixel
	return c.Type("gif").Send(transparentPixel())
}

func (cc *CampaignController) HandleClickTracking(c *fiber.Ctx) error {
	messageID := c.Params("messageID")
	token := c.Params("token")
	originalURL := c.Query("url")

	// Validate token
	if !isValidToken(messageID, token) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid token")
	}

	// Update click stats in database
	cc.updateClickStats(messageID)

	// Redirect to original URL
	return c.Redirect(originalURL, fiber.StatusFound)
}

func (cc *CampaignController) updateOpenStats(messageID string) {
	cc.DB.Model(&models.CampaignActivity{}).
		Where("message_id = ?", messageID).
		Updates(map[string]interface{}{
			"open_count": gorm.Expr("open_count + 1"),
			"opened_at":  time.Now(),
		})
}

func (cc *CampaignController) updateClickStats(messageID string) {
	cc.DB.Model(&models.CampaignActivity{}).
		Where("message_id = ?", messageID).
		Updates(map[string]interface{}{
			"click_count": gorm.Expr("click_count + 1"),
			"clicked_at":  time.Now(),
		})
}

func transparentPixel() []byte {
	// 1x1 transparent GIF
	return []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
		0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
		0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
		0x01, 0x00, 0x3b,
	}
}
