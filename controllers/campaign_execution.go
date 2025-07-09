package controller

import (
	"time"

	"mailnexy/models"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

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
			err = cc.sendEmailToLead(sender, lead, currentNode.Data, &campaign)
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
func (cc *CampaignController) sendEmailToLead(sender *models.Sender, lead *models.Lead, nodeData models.NodeData, campaign *models.Campaign) error {
	// Implement your email sending logic here
	// Use the MailService to send the email
	messageID := uuid.New().String()
	baseURL := "https://yourdomain.com" // Change to your actual domain
	trackedBody := utils.InjectTracking(nodeData.Body, baseURL, messageID)
	email := utils.Email{
		From:    sender.FromEmail,
		To:      lead.Email,
		Subject: nodeData.Subject,
		Body:    trackedBody,
	}
	// if err := cc.MailService.Send(email); err != nil {
	// 	return err
	// }

	// In sendEmailToLead method
	returnedMsgID, err := cc.MailService.Send(email)
	if err != nil {
		return err
	}

	// Use either generated or returned message ID
	if returnedMsgID != "" {
		messageID = returnedMsgID
	}

	// Record the activity with the messageID
	activity := models.CampaignActivity{
		CampaignID: campaign.ID, // Set CampaignID
		LeadID:     lead.ID,
		UserID:     campaign.UserID,
		SenderID:   sender.ID,
		SentAt:     utils.Pointer(time.Now()),
		MessageID:  messageID, // Store the message ID for tracking
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
