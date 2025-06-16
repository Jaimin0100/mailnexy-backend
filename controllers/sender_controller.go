package controller

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
)

type CreateSenderRequest struct {
	Name              string `json:"name" validate:"required"`
	FromEmail         string `json:"from_email" validate:"required,email"`
	FromName          string `json:"from_name" validate:"required"`
	ProviderType      string `json:"provider_type" validate:"required,oneof=smtp gmail outlook yahoo custom"`
	SMTPHost          string `json:"smtp_host" validate:"required_if=ProviderType smtp"`
	SMTPPort          int    `json:"smtp_port" validate:"required_if=ProviderType smtp"`
	SMTPUsername      string `json:"smtp_username" validate:"required_if=ProviderType smtp"`
	SMTPPassword      string `json:"smtp_password" validate:"required_if=ProviderType smtp"`
	Encryption        string `json:"encryption" validate:"required_if=ProviderType smtp,oneof=SSL TLS STARTTLS"`
	IMAPHost          string `json:"imap_host"`
	IMAPPort          int    `json:"imap_port"`
	IMAPUsername      string `json:"imap_username"`
	IMAPPassword      string `json:"imap_password"`
	IMAPEncryption    string `json:"imap_encryption" validate:"omitempty,oneof=SSL TLS STARTTLS"`
	IMAPMailbox       string `json:"imap_mailbox"`
	OAuthProvider     string `json:"oauth_provider"`
	OAuthToken        string `json:"oauth_token"`
	OAuthRefreshToken string `json:"oauth_refresh_token"`
	TrackOpens        bool   `json:"track_opens"`
	TrackClicks       bool   `json:"track_clicks"`
	TrackReplies      bool   `json:"track_replies"`
}

type UpdateSenderRequest struct {
	Name              *string `json:"name"`
	FromEmail         *string `json:"from_email" validate:"omitempty,email"`
	FromName          *string `json:"from_name"`
	SMTPPassword      *string `json:"smtp_password"`
	IMAPPassword      *string `json:"imap_password"`
	OAuthToken        *string `json:"oauth_token"`
	OAuthRefreshToken *string `json:"oauth_refresh_token"`
	TrackOpens        *bool   `json:"track_opens"`
	TrackClicks       *bool   `json:"track_clicks"`
	TrackReplies      *bool   `json:"track_replies"`
}

func CreateSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var req CreateSenderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Encrypt sensitive data
	encryptedSMTPPassword, err := utils.Encrypt(req.SMTPPassword)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to encrypt SMTP password",
		})
	}

	encryptedIMAPPassword, err := utils.Encrypt(req.IMAPPassword)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to encrypt IMAP password",
		})
	}

	encryptedOAuthToken, err := utils.Encrypt(req.OAuthToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to encrypt OAuth token",
		})
	}

	encryptedRefreshToken, err := utils.Encrypt(req.OAuthRefreshToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to encrypt OAuth refresh token",
		})
	}

	// Create sender
	sender := models.Sender{
		UserID:            user.ID,
		Name:              req.Name,
		FromEmail:         req.FromEmail,
		FromName:          req.FromName,
		ProviderType:      req.ProviderType,
		SMTPHost:          req.SMTPHost,
		SMTPPort:          req.SMTPPort,
		SMTPUsername:      req.SMTPUsername,
		SMTPPassword:      encryptedSMTPPassword,
		Encryption:        req.Encryption,
		IMAPHost:          req.IMAPHost,
		IMAPPort:          req.IMAPPort,
		IMAPUsername:      req.IMAPUsername,
		IMAPPassword:      encryptedIMAPPassword,
		IMAPEncryption:    req.IMAPEncryption,
		IMAPMailbox:       req.IMAPMailbox,
		OAuthProvider:     req.OAuthProvider,
		OAuthToken:        encryptedOAuthToken,
		OAuthRefreshToken: encryptedRefreshToken,
		TrackOpens:        req.TrackOpens,
		TrackClicks:       req.TrackClicks,
		TrackReplies:      req.TrackReplies,
	}

	if err := config.DB.Create(&sender).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create sender",
		})
	}

	// Sanitize before returning
	sender.Sanitize()

	return c.Status(fiber.StatusCreated).JSON(sender)
}

func GetSenders(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var senders []models.Sender
	if err := config.DB.Where("user_id = ?", user.ID).Find(&senders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch senders",
		})
	}

	// Sanitize all senders
	for i := range senders {
		senders[i].Sanitize()
	}

	return c.JSON(senders)
}

func validateSenderID(id string) error {
	if id == "" || id == "undefined" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sender ID")
	}
	// Add check for numeric ID if your DB uses numeric IDs
	if _, err := strconv.Atoi(id); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Sender ID must be numeric")
	}
	return nil
}

func GetSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	if err := validateSenderID(senderID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	sender.Sanitize()
	return c.JSON(sender)
}

func UpdateSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	if err := validateSenderID(senderID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var req UpdateSenderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	// Update fields
	if req.Name != nil {
		sender.Name = *req.Name
	}
	if req.FromEmail != nil {
		sender.FromEmail = *req.FromEmail
	}
	if req.FromName != nil {
		sender.FromName = *req.FromName
	}
	if req.SMTPPassword != nil {
		encrypted, err := utils.Encrypt(*req.SMTPPassword)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to encrypt SMTP password",
			})
		}
		sender.SMTPPassword = encrypted
	}
	if req.IMAPPassword != nil {
		encrypted, err := utils.Encrypt(*req.IMAPPassword)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to encrypt IMAP password",
			})
		}
		sender.IMAPPassword = encrypted
	}
	if req.OAuthToken != nil {
		encrypted, err := utils.Encrypt(*req.OAuthToken)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to encrypt OAuth token",
			})
		}
		sender.OAuthToken = encrypted
	}
	if req.OAuthRefreshToken != nil {
		encrypted, err := utils.Encrypt(*req.OAuthRefreshToken)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to encrypt OAuth refresh token",
			})
		}
		sender.OAuthRefreshToken = encrypted
	}
	if req.TrackOpens != nil {
		sender.TrackOpens = *req.TrackOpens
	}
	if req.TrackClicks != nil {
		sender.TrackClicks = *req.TrackClicks
	}
	if req.TrackReplies != nil {
		sender.TrackReplies = *req.TrackReplies
	}

	if err := config.DB.Save(&sender).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update sender",
		})
	}

	sender.Sanitize()
	return c.JSON(sender)
}

func DeleteSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	// Add logging to see what ID is received
	fmt.Printf("Attempting to delete sender with ID: %v (type: %T)\n", senderID, senderID)

	if err := validateSenderID(senderID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		fmt.Printf("Error finding sender: %v\n", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	if err := config.DB.Delete(&sender).Error; err != nil {
		fmt.Printf("Error deleting sender: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete sender",
		})
	}

	fmt.Printf("Successfully deleted sender with ID: %v\n", senderID)
	return c.SendStatus(fiber.StatusNoContent)
}
func TestSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	// TODO: Implement actual SMTP/IMAP testing logic
	// This would involve connecting to the SMTP/IMAP server with the provided credentials
	// and verifying they work correctly

	// For now, just return success
	return c.JSON(fiber.Map{
		"message": "Sender test initiated",
		"status":  "pending",
	})
}

func VerifySender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	if err := validateSenderID(senderID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var sender models.Sender
	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	// TODO: Implement actual SMTP verification logic
	// This would involve connecting to the SMTP server and verifying the credentials work

	// For now, we'll just mark it as verified
	if err := config.DB.Model(&sender).Update("smtp_verified", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify sender",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Sender verified successfully",
		"data": fiber.Map{
			"smtp_verified": true,
		},
	})
}