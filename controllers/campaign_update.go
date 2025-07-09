package controller

import (
	"strconv"
	"time"

	"mailnexy/models"

	"github.com/gofiber/fiber/v2"
)

// UpdateCampaign updates campaign details and flow
func (cc *CampaignController) UpdateCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	// Define input structure with pointers for partial updates
	var input struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		Flow        *struct {
			Nodes []models.CampaignNode `json:"nodes"`
			Edges []models.CampaignEdge `json:"edges"`
		} `json:"flow"`
	}

	// Parse request body
	if err := c.BodyParser(&input); err != nil {
		cc.Logger.Printf("Error parsing request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Start database transaction
	tx := cc.DB.Begin()

	// Find existing campaign
	var campaign models.Campaign
	if err := tx.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Campaign not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	// Find existing flow
	var flow models.CampaignFlow
	if err := tx.Where("campaign_id = ?", campaign.ID).First(&flow).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Flow not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	// Apply partial updates to campaign
	if input.Name != nil {
		campaign.Name = *input.Name
	}
	if input.Description != nil {
		campaign.Description = *input.Description
	}
	if input.Status != nil {
		campaign.Status = *input.Status
	}

	// Update flow if provided
	if input.Flow != nil {
		flow.Nodes = input.Flow.Nodes
		flow.Edges = input.Flow.Edges
		flow.UpdatedAt = time.Now()

		if err := tx.Save(&flow).Error; err != nil {
			tx.Rollback()
			cc.Logger.Printf("Failed to update flow: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update campaign flow",
			})
		}
	}

	// Update campaign
	campaign.UpdatedAt = time.Now()
	if err := tx.Save(&campaign).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to update campaign: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update campaign",
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		cc.Logger.Printf("Transaction commit failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete update",
		})
	}

	// Return updated campaign data
	return c.JSON(fiber.Map{
		"message": "Campaign updated successfully",
		"campaign": fiber.Map{
			"id":          campaign.ID,
			"name":        campaign.Name,
			"description": campaign.Description,
			"status":      campaign.Status,
			"updated_at":  campaign.UpdatedAt,
		},
		"flow": fiber.Map{
			"nodes": flow.Nodes,
			"edges": flow.Edges,
		},
	})
}

// UpdateCampaignFlow updates the campaign flow
func (cc *CampaignController) UpdateCampaignFlow(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	var input struct {
		Nodes []models.CampaignNode `json:"nodes"`
		Edges []models.CampaignEdge `json:"edges"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	var flow models.CampaignFlow
	if err := cc.DB.Where("campaign_id = ?", campaign.ID).First(&flow).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	flow.Nodes = input.Nodes
	flow.Edges = input.Edges

	if err := cc.DB.Save(&flow).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update campaign flow",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Campaign flow updated successfully",
		"flow":    flow,
	})
}

// campaign_controller.go
func (cc *CampaignController) UpdateCampaignLeadLists(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid campaign ID",
		})
	}

	var input struct {
		LeadListIDs []uint `json:"leadListIds"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Start transaction
	tx := cc.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := tx.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	// Delete existing associations
	if err := tx.Where("campaign_id = ?", campaignID).Delete(&models.CampaignLeadList{}).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to delete lead list associations: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update lead lists",
		})
	}

	// Create new associations
	for _, listID := range input.LeadListIDs {
		association := models.CampaignLeadList{
			CampaignID: uint(campaignID),
			LeadListID: listID,
		}
		if err := tx.Create(&association).Error; err != nil {
			tx.Rollback()
			cc.Logger.Printf("Failed to create association: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update lead lists",
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		cc.Logger.Printf("Transaction commit failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete update",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Lead lists updated successfully",
	})
}

// In campaign_controller.go
func (cc *CampaignController) UpdateCampaignSettings(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid campaign ID",
		})
	}

	var input struct {
		TrackOpens      bool   `json:"trackOpens"`
		TrackClicks     bool   `json:"trackClicks"`
		EmailAccountIDs []uint `json:"emailAccountIds"`
		// Add other settings fields here
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	// Verify user owns all sender accounts
	for _, accountID := range input.EmailAccountIDs {
		var sender models.Sender
		if err := cc.DB.Where("id = ? AND user_id = ?", accountID, user.ID).First(&sender).Error; err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid sender account",
			})
		}
	}

	// Start transaction
	tx := cc.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete existing sender associations
	if err := tx.Where("campaign_id = ?", campaignID).Delete(&models.CampaignSender{}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update sender accounts",
		})
	}

	// Create new associations
	for _, senderID := range input.EmailAccountIDs {
		association := models.CampaignSender{
			CampaignID: uint(campaignID),
			SenderID:   senderID,
		}
		if err := tx.Create(&association).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update sender accounts",
			})
		}
	}

	// Update campaign settings
	campaign.TrackOpens = input.TrackOpens
	campaign.TrackClicks = input.TrackClicks
	if err := tx.Save(&campaign).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update campaign settings",
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete update",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Campaign settings updated successfully",
	})
}
