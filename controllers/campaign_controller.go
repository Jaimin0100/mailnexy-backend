package controller

import (
	"log"
	"time"

	"mailnexy/models"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"gorm.io/gorm"
)

type CampaignController struct {
	DB          *gorm.DB
	Logger      *log.Logger
	MailService utils.MailServiceInterface
}

func NewCampaignController(db *gorm.DB, logger *log.Logger) *CampaignController {
	return &CampaignController{
		DB:     db,
		Logger: logger,
	}
}

// Enhanced CreateCampaign with lead list support
func (cc *CampaignController) CreateCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var input struct {
		Name        string                `json:"name"`
		Description string                `json:"description"`
		LeadListIDs []uint                `json:"lead_list_ids"` // Add lead list IDs
		Nodes       []models.CampaignNode `json:"nodes"`
		Edges       []models.CampaignEdge `json:"edges"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Start transaction
	tx := cc.DB.Begin()

	// Create base campaign
	campaign := models.Campaign{
		UserID:      user.ID,
		Name:        input.Name,
		Description: input.Description,
		Subject:     "Custom Campaign",
		Status:      "draft",
	}

	if err := tx.Create(&campaign).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create campaign",
		})
	}

	// Associate lead lists with campaign
	for _, listID := range input.LeadListIDs {
		if err := tx.Create(&models.CampaignLeadList{
			CampaignID: campaign.ID,
			LeadListID: listID,
		}).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to associate lead list with campaign",
			})
		}
	}

	// Create campaign flow
	flow := models.CampaignFlow{
		CampaignID: campaign.ID,
		UserID:     user.ID,
		Nodes:      input.Nodes,
		Edges:      input.Edges,
	}

	if err := tx.Create(&flow).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create campaign flow",
		})
	}

	tx.Commit()

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Campaign created successfully",
		"campaign": campaign,
		"flow":     flow,
	})
}

// GetCampaigns returns a list of all campaigns for the user
func (cc *CampaignController) GetCampaigns(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var campaigns []models.Campaign
	if err := cc.DB.Where("user_id = ?", user.ID).Find(&campaigns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch campaigns",
		})
	}

	return c.JSON(campaigns)
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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	var execution models.CampaignExecution
	cc.DB.Where("campaign_id = ?", campaign.ID).First(&execution)

	return c.JSON(fiber.Map{
		"campaign":  campaign,
		"flow":      flow,
		"execution": execution,
	})
}

// UpdateCampaign updates campaign details
func (cc *CampaignController) UpdateCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
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

	campaign.Name = input.Name
	campaign.Description = input.Description

	if err := cc.DB.Save(&campaign).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update campaign",
		})
	}

	return c.JSON(fiber.Map{
		"message":  "Campaign updated successfully",
		"campaign": campaign,
	})
}

// GetCampaignFlow returns the flow for a campaign
func (cc *CampaignController) GetCampaignFlow(c *fiber.Ctx) error {
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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	return c.JSON(flow)
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

// StartCampaign begins executing a campaign
func (cc *CampaignController) StartCampaign(c *fiber.Ctx) error {
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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign flow not found",
		})
	}

	// Check if campaign is already running
	if campaign.Status == "sending" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Campaign is already running",
		})
	}

	// Create execution record
	execution := models.CampaignExecution{
		CampaignID:    campaign.ID,
		FlowID:        flow.ID,
		CurrentNodeID: flow.Nodes[0].ID, // Start with first node
		NextRunAt:     utils.Pointer(time.Now()),
	}

	if err := cc.DB.Create(&execution).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start campaign execution",
		})
	}

	// Update campaign status
	campaign.Status = "sending"
	campaign.StartedAt = utils.Pointer(time.Now())
	if err := cc.DB.Save(&campaign).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update campaign status",
		})
	}

	// Start campaign worker in background
	go cc.runCampaignWorker(campaign.ID, flow.ID, execution.ID)

	return c.JSON(fiber.Map{
		"message": "Campaign started successfully",
	})
}

// StopCampaign stops a running campaign
func (cc *CampaignController) StopCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	campaignID := c.Params("id")

	var campaign models.Campaign
	if err := cc.DB.Where("id = ? AND user_id = ?", campaignID, user.ID).First(&campaign).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Campaign not found",
		})
	}

	if campaign.Status != "sending" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Campaign is not running",
		})
	}

	campaign.Status = "paused"
	campaign.CompletedAt = utils.Pointer(time.Now())

	if err := cc.DB.Save(&campaign).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to stop campaign",
		})
	}

	// Delete execution record
	if err := cc.DB.Where("campaign_id = ?", campaign.ID).Delete(&models.CampaignExecution{}).Error; err != nil {
		cc.Logger.Printf("Failed to delete execution record: %v", err)
	}

	return c.JSON(fiber.Map{
		"message": "Campaign stopped successfully",
	})
}

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

// Enhanced runCampaignWorker with lead processing
func (cc *CampaignController) runCampaignWorker(campaignID, flowID, executionID uint) {
	campaignSender := utils.NewCampaignSender(cc.DB, cc.Logger)

	for {
		// Check if campaign is still active
		var execution models.CampaignExecution
		if err := cc.DB.First(&execution, executionID).Error; err != nil {
			cc.Logger.Printf("Execution record not found: %v", err)
			return
		}

		var campaign models.Campaign
		if err := cc.DB.First(&campaign, campaignID).Error; err != nil {
			cc.Logger.Printf("Campaign not found: %v", err)
			return
		}

		if campaign.Status != "sending" {
			cc.Logger.Printf("Campaign %d is no longer active", campaignID)
			return
		}

		// Get the flow
		var flow models.CampaignFlow
		if err := cc.DB.First(&flow, flowID).Error; err != nil {
			cc.Logger.Printf("Flow not found: %v", err)
			return
		}

		// Find current node
		var currentNode *models.CampaignNode
		for _, node := range flow.Nodes {
			if node.ID == execution.CurrentNodeID {
				currentNode = &node
				break
			}
		}

		if currentNode == nil {
			cc.Logger.Printf("Current node %s not found in flow", execution.CurrentNodeID)
			return
		}

		// Process node based on type
		switch currentNode.Type {
		case "email":
			// Get sender with available capacity
			sender, err := campaignSender.RotateSender(campaign.UserID)
			if err != nil {
				cc.Logger.Printf("No available sender: %v", err)
				time.Sleep(1 * time.Hour) // Wait and try again
				continue
			}

			// Get next lead to process
			lead, err := cc.getNextLead(campaignID)
			if err != nil {
				cc.Logger.Printf("Error getting next lead: %v", err)
				time.Sleep(5 * time.Minute) // Wait before retrying
				continue
			}

			if lead == nil {
				cc.Logger.Printf("No more leads to process for campaign %d", campaignID)
				campaign.Status = "completed"
				campaign.CompletedAt = utils.Pointer(time.Now())
				if err := cc.DB.Save(&campaign).Error; err != nil {
					cc.Logger.Printf("Failed to mark campaign as completed: %v", err)
				}
				return
			}

			// Send email to lead
			err = cc.sendEmailToLead(sender, lead, currentNode.Data)
			if err != nil {
				cc.Logger.Printf("Failed to send email to lead %d: %v", lead.ID, err)
				// Mark lead as failed (you might want to retry later)
				continue
			}

			// Record activity
			if err := cc.recordCampaignActivity(campaign.ID, lead.ID, sender.ID); err != nil {
				cc.Logger.Printf("Failed to record campaign activity: %v", err)
			}

			// Update execution stats
			execution.EmailsSent++
			if err := cc.DB.Save(&execution).Error; err != nil {
				cc.Logger.Printf("Failed to update execution: %v", err)
			}

			// Move to next node after sending
			nextNodeID := cc.getNextNodeID(flow, currentNode.ID, "")
			execution.CurrentNodeID = nextNodeID
			execution.NextRunAt = utils.Pointer(time.Now().Add(1 * time.Minute))

		case "delay":
			delayDuration := time.Duration(currentNode.Data.DelayAmount)
			switch currentNode.Data.DelayUnit {
			case "hours":
				delayDuration *= time.Hour
			case "days":
				delayDuration *= 24 * time.Hour
			default:
				delayDuration *= time.Hour
			}

			cc.Logger.Printf("Delaying for %v", delayDuration)
			execution.NextRunAt = utils.Pointer(time.Now().Add(delayDuration))
			nextNodeID := cc.getNextNodeID(flow, currentNode.ID, "")
			execution.CurrentNodeID = nextNodeID

		case "condition":
			nextNodeID := cc.getNextNodeID(flow, currentNode.ID, "")
			execution.CurrentNodeID = nextNodeID
			execution.NextRunAt = utils.Pointer(time.Now().Add(1 * time.Minute))

		case "goal":
			cc.Logger.Printf("Campaign reached goal node")
			campaign.Status = "completed"
			campaign.CompletedAt = utils.Pointer(time.Now())
			if err := cc.DB.Save(&campaign).Error; err != nil {
				cc.Logger.Printf("Failed to mark campaign as completed: %v", err)
			}
			return
		}

		// Save execution state
		if err := cc.DB.Save(&execution).Error; err != nil {
			cc.Logger.Printf("Failed to save execution state: %v", err)
			return
		}

		// Wait until next run time
		now := time.Now()
		if execution.NextRunAt != nil && execution.NextRunAt.After(now) {
			sleepDuration := execution.NextRunAt.Sub(now)
			cc.Logger.Printf("Waiting for %v until next run", sleepDuration)
			time.Sleep(sleepDuration)
		}
	}
}

// getNextNodeID finds the next node based on edges and conditions
func (cc *CampaignController) getNextNodeID(flow models.CampaignFlow, currentNodeID, condition string) string {
	for _, edge := range flow.Edges {
		if edge.Source == currentNodeID {
			if edge.Condition == "" || edge.Condition == condition {
				return edge.Target
			}
		}
	}
	return "" // No next node found
}

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

// getNextLead gets the next lead to process for the campaign
func (cc *CampaignController) getNextLead(campaignID uint) (*models.Lead, error) {
	var lead models.Lead
	err := cc.DB.Raw(`
        SELECT l.* FROM leads l
        JOIN lead_list_memberships llm ON l.id = llm.lead_id
        JOIN campaign_lead_lists cll ON llm.lead_list_id = cll.lead_list_id
        LEFT JOIN campaign_activities ca ON l.id = ca.lead_id AND ca.campaign_id = ?
        WHERE cll.campaign_id = ?
        AND (ca.id IS NULL OR ca.replied_at IS NOT NULL)
        AND l.is_bounced = false
        AND l.is_unsubscribed = false
        AND l.is_do_not_contact = false
        LIMIT 1
    `, campaignID, campaignID).Scan(&lead).Error

	if err != nil {
		return nil, err
	}

	if lead.ID == 0 {
		return nil, nil // No more leads
	}

	return &lead, nil
}

// sendEmailToLead sends an email to a lead
func (cc *CampaignController) sendEmailToLead(sender *models.Sender, lead *models.Lead, nodeData models.NodeData) error {
	// Implement your email sending logic here
	// Use the MailService to send the email
	email := utils.Email{
		From:    sender.FromEmail,
		To:      lead.Email,
		Subject: nodeData.Subject,
		Body:    nodeData.Body,
	}

	// In sendEmailToLead method
	messageID, err := cc.MailService.Send(email)
	if err != nil {
		return err
	}

	// Record the activity with the messageID
	activity := models.CampaignActivity{
		LeadID:    lead.ID,
		SenderID:  sender.ID,
		SentAt:    utils.Pointer(time.Now()),
		MessageID: messageID, // Store the message ID for tracking
	}

	if err := cc.DB.Create(&activity).Error; err != nil {
		return err
	}

	// You might want to store the messageID for tracking
	return nil
}

// recordCampaignActivity records a campaign activity
func (cc *CampaignController) recordCampaignActivity(campaignID, leadID, senderID uint) error {
	activity := models.CampaignActivity{
		CampaignID: campaignID,
		LeadID:     leadID,
		SenderID:   senderID,
		SentAt:     utils.Pointer(time.Now()),
	}

	return cc.DB.Create(&activity).Error
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

// campaign_controller.go
func HandleCampaignProgressWS(c *websocket.Conn) {
	defer c.Close()

	var input struct {
		CampaignName string `json:"campaignName"`
		Action       string `json:"action"`
	}

	// Read JSON message
	if err := c.ReadJSON(&input); err != nil {
		log.Printf("Error reading JSON: %v", err)
		return
	}

	// Simulate campaign progress
	if input.Action == "simulate" {
		stages := []string{
			"Sending initial emails...",
			"Waiting for responses...",
			"Sending follow-ups...",
			"Tracking opens and clicks...",
			"Processing replies...",
			"Campaign completed!",
		}

		for i, stage := range stages {
			time.Sleep(2 * time.Second)
			progress := struct {
				Message string `json:"message"`
				Percent int    `json:"percent"`
				Status  string `json:"status"`
			}{
				Message: stage,
				Percent: (i + 1) * 100 / len(stages),
				Status:  "running",
			}

			if i == len(stages)-1 {
				progress.Status = "completed"
			}

			// Write JSON message
			if err := c.WriteJSON(progress); err != nil {
				log.Printf("Error writing JSON: %v", err)
				break
			}
		}
	}
}
