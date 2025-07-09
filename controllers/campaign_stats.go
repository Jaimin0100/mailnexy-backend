package controller

import (
	"mailnexy/models"

	"github.com/gofiber/fiber/v2"
)

// GetCampaignStats returns statistics for a campaign
func (cc *CampaignController) GetCampaignStats(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	var execution models.CampaignExecution
	if err := cc.DB.Where("campaign_id = ?", campaign.ID).First(&execution).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Execution record not found",
		})
	}

	// Get all activities for this campaign
	var activities []models.CampaignActivity
	if err := cc.DB.Where("campaign_id = ?", campaign.ID).Find(&activities).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch campaign activities",
		})
	}

	// Calculate stats
	stats := fiber.Map{
		"emails_sent":  execution.EmailsSent,
		"opens":        execution.Opens,
		"clicks":       execution.Clicks,
		"replies":      execution.Replies,
		"current_node": execution.CurrentNodeID,
		"next_run_at":  execution.NextRunAt,
	}

	return c.JSON(stats)
}

// GetCampaignLeadStats returns lead statistics for a campaign
func (cc *CampaignController) GetCampaignLeadStats(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	var stats struct {
		TotalLeads        int `json:"total_leads"`
		EmailsSent        int `json:"emails_sent"`
		LeadsReplied      int `json:"leads_replied"`
		LeadsOpened       int `json:"leads_opened"`
		LeadsClicked      int `json:"leads_clicked"`
		LeadsBounced      int `json:"leads_bounced"`
		LeadsUnsubscribed int `json:"leads_unsubscribed"`
	}

	// Get total leads in campaign
	cc.DB.Raw(`
        SELECT COUNT(DISTINCT l.id) 
        FROM leads l
        JOIN lead_list_memberships llm ON l.id = llm.lead_id
        JOIN campaign_lead_lists cll ON llm.lead_list_id = cll.lead_list_id
        WHERE cll.campaign_id = ?
    `, campaignID).Scan(&stats.TotalLeads)

	// Get other stats
	cc.DB.Raw(`
        SELECT 
            COUNT(DISTINCT ca.lead_id) as emails_sent,
            SUM(CASE WHEN ca.replied_at IS NOT NULL THEN 1 ELSE 0 END) as leads_replied,
            SUM(CASE WHEN ca.opened_at IS NOT NULL THEN 1 ELSE 0 END) as leads_opened,
            SUM(CASE WHEN ca.clicked_at IS NOT NULL THEN 1 ELSE 0 END) as leads_clicked,
            SUM(CASE WHEN l.is_bounced = true THEN 1 ELSE 0 END) as leads_bounced,
            SUM(CASE WHEN l.is_unsubscribed = true THEN 1 ELSE 0 END) as leads_unsubscribed
        FROM campaign_activities ca
        JOIN leads l ON ca.lead_id = l.id
        WHERE ca.campaign_id = ?
    `, campaignID).Scan(&stats)

	return c.JSON(stats)
}


func (cc *CampaignController) GetTrackingStats(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	// Verify user owns the campaign
	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	// Get tracking stats
	var stats struct {
		TotalEmails  int `json:"totalEmails"`
		Opens        int `json:"opens"`
		UniqueOpens  int `json:"uniqueOpens"`
		Clicks       int `json:"clicks"`
		UniqueClicks int `json:"uniqueClicks"`
	}

	// Query database for stats
	cc.DB.Raw(`
        SELECT 
            COUNT(*) as total_emails,
            SUM(open_count) as opens,
            COUNT(DISTINCT CASE WHEN open_count > 0 THEN lead_id END) as unique_opens,
            SUM(click_count) as clicks,
            COUNT(DISTINCT CASE WHEN click_count > 0 THEN lead_id END) as unique_clicks
        FROM campaign_activities
        WHERE campaign_id = ?
    `, campaignID).Scan(&stats)

	return c.JSON(stats)
}
