package controller

import (
	"strconv"
	"time"

	"mailnexy/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GetCampaigns returns a list of all campaigns for the user
func (cc *CampaignController) GetCampaigns(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var campaigns []models.Campaign
	if err := cc.DB.Where("user_id = ?", user.ID).Find(&campaigns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch campaigns",
		})
	}

	type CampaignResponse struct {
		ID              uint   `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description"`
		Status          string `json:"status"`
		SentCount       int    `json:"sent_count"`
		OpenCount       int    `json:"open_count"`
		ClickCount      int    `json:"click_count"`
		ReplyCount      int    `json:"reply_count"`
		BounceCount     int    `json:"bounce_count"`
		InterestedCount int    `json:"interested_count"`
		Flow            struct {
			Nodes []models.CampaignNode `json:"nodes"`
			Edges []models.CampaignEdge `json:"edges"`
		} `json:"flow"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	response := make([]CampaignResponse, len(campaigns))
	for i, campaign := range campaigns {
		var flow models.CampaignFlow
		err := cc.DB.Where("campaign_id = ?", campaign.ID).First(&flow).Error

		// Handle flow not found
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				response[i] = CampaignResponse{
					ID:          campaign.ID,
					Name:        campaign.Name,
					Description: campaign.Description,
					Status:      campaign.Status,
					Flow: struct {
						Nodes []models.CampaignNode `json:"nodes"`
						Edges []models.CampaignEdge `json:"edges"`
					}{},
					CreatedAt: campaign.CreatedAt,
					UpdatedAt: campaign.UpdatedAt,
				}
				continue
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch campaign flow",
			})
		}

		// Ensure Nodes and Edges are not nil
		if flow.Nodes == nil {
			flow.Nodes = []models.CampaignNode{}
		}
		if flow.Edges == nil {
			flow.Edges = []models.CampaignEdge{}
		}

		// Fetch execution data
		var execution models.CampaignExecution
		sentCount := campaign.SentCount
		openCount := campaign.OpenCount
		clickCount := campaign.ClickCount
		replyCount := campaign.ReplyCount
		bounceCount := campaign.BounceCount

		err = cc.DB.Where("campaign_id = ?", campaign.ID).First(&execution).Error
		if err == nil {
			// Override with execution data if available
			sentCount = execution.EmailsSent
			openCount = execution.Opens
			clickCount = execution.Clicks
			replyCount = execution.Replies
			// BounceCount might need separate aggregation; use model value for now
		} else if err != gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch campaign execution",
			})
		}

		response[i] = CampaignResponse{
			ID:              campaign.ID,
			Name:            campaign.Name,
			Description:     campaign.Description,
			Status:          campaign.Status,
			SentCount:       sentCount,
			OpenCount:       openCount,
			ClickCount:      clickCount,
			ReplyCount:      replyCount,
			BounceCount:     bounceCount,
			InterestedCount: 0,
			Flow: struct {
				Nodes []models.CampaignNode `json:"nodes"`
				Edges []models.CampaignEdge `json:"edges"`
			}{
				Nodes: flow.Nodes,
				Edges: flow.Edges,
			},
			CreatedAt: campaign.CreatedAt,
			UpdatedAt: campaign.UpdatedAt,
		}
	}

	return c.JSON(response)
}

// GetCampaign returns a single campaign with its flow
func (cc *CampaignController) GetCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	var flow models.CampaignFlow
	if err := cc.DB.Where("campaign_id = ?", campaign.ID).First(&flow).Error; err != nil {
		cc.Logger.Printf("Flow fetch error: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	var execution models.CampaignExecution
	cc.DB.Where("campaign_id = ?", campaign.ID).First(&execution)

	cc.Logger.Printf("Fetched flow: nodes=%d, edges=%d", len(flow.Nodes), len(flow.Edges))

	return c.JSON(fiber.Map{
		"campaign":  campaign,
		"flow":      flow,
		"execution": execution,
	})
}

// GetCampaignFlow returns the flow for a campaign
func (cc *CampaignController) GetCampaignFlow(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignIDStr := c.Params("id")

	// Validate that campaignIDStr is a valid integer
	if _, err := strconv.ParseUint(campaignIDStr, 10, 64); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid campaign ID",
		})
	}

	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignIDStr, user.ID).First(&campaign).Error; err != nil {
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

	return c.JSON(flow)
}

// GetCampaignLeads returns leads for a campaign
func (cc *CampaignController) GetCampaignLeads(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	var leads []models.Lead
	err := cc.DB.Raw(`
        SELECT l.* FROM leads l
        JOIN lead_list_memberships llm ON l.id = llm.lead_id
        JOIN campaign_lead_lists cll ON llm.lead_list_id = cll.lead_list_id
        WHERE cll.campaign_id = ?
    `, campaignID).Scan(&leads).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch campaign leads",
		})
	}

	return c.JSON(leads)
}
