package controller

import (
	"encoding/json"

	"mailnexy/models"

	"github.com/gofiber/fiber/v2"
)

func (cc *CampaignController) CreateCampaign(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		LeadListIDs []uint `json:"lead_list_ids"`
		Status      string `json:"status"`
		Flow        struct {
			Nodes []models.CampaignNode `json:"nodes"`
			Edges []models.CampaignEdge `json:"edges"`
		} `json:"flow"`
	}

	if err := c.BodyParser(&input); err != nil {
		cc.Logger.Printf("Error parsing request body: %v", err, c.Body())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
	}
	// Accept draft campaigns with empty flow
	// Set default status to draft if not provided
	status := input.Status
	if status == "" {
		status = "draft"
	}
	// Allow empty flow only for draft campaigns
	if len(input.Flow.Nodes) == 0 && status != "draft" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Flow cannot be empty for non-draft campaigns",
		})
	}
	if status != "draft" && len(input.Flow.Nodes) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Flow cannot be empty for non-draft campaigns",
		})
	}

	// Log input data for debugging
	cc.Logger.Printf("Received input: %+v", input)

	// Validate nodes and edges
	if len(input.Flow.Nodes) == 0 {
		cc.Logger.Printf("Nodes or edges are empty")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Nodes and edges cannot be empty",
		})
	}

	// Start transaction
	tx := cc.DB.Begin()

	if len(input.Flow.Nodes) == 0 {
		cc.Logger.Printf("Nodes or edges are empty")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Nodes and edges cannot be empty",
		})
	}

	// After - Allow empty edges
	if len(input.Flow.Nodes) == 0 {
		cc.Logger.Printf("Nodes are empty")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Nodes cannot be empty",
		})
	}

	// Create base campaign
	campaign := models.Campaign{
		UserID:      user.ID,
		Name:        input.Name,
		Description: input.Description,
		Subject:     "Custom Campaign",
		Status:      status,
	}

	if err := tx.Create(&campaign).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to create campaign: %v", err)
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
			cc.Logger.Printf("Failed to associate lead list: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to associate lead list with campaign",
			})
		}
	}

	// Create campaign flow
	flow := models.CampaignFlow{
		CampaignID: campaign.ID,
		UserID:     user.ID,
		Nodes:      input.Flow.Nodes,
		Edges:      input.Flow.Edges,
	}

	// Ensure nodes and edges are properly serialized
	nodesJSON, err := json.Marshal(input.Flow.Nodes)
	if err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to marshal nodes: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid nodes format",
		})
	}
	edgesJSON, err := json.Marshal(input.Flow.Edges)
	if err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to marshal edges: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid edges format",
		})
	}

	cc.Logger.Printf("Serialized nodes: %s", string(nodesJSON))
	cc.Logger.Printf("Serialized edges: %s", string(edgesJSON))

	if err := tx.Create(&flow).Error; err != nil {
		tx.Rollback()
		cc.Logger.Printf("Failed to create campaign flow: %v", err)
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
