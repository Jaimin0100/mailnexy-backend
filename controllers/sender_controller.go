package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"

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

type TestResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
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

// func TestSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	// Decrypt credentials
// 	smtpPassword, err := utils.Decrypt(sender.SMTPPassword)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to decrypt SMTP password",
// 		})
// 	}

// 	var smtpTestResult struct {
// 		Success bool
// 		Error   string
// 	}

// 	// Test SMTP connection
// 	if sender.SMTPHost != "" && sender.SMTPUsername != "" && smtpPassword != "" {
// 		smtpAddr := fmt.Sprintf("%s:%d", sender.SMTPHost, sender.SMTPPort)

// 		var auth smtp.Auth
// 		if sender.SMTPUsername != "" && smtpPassword != "" {
// 			auth = smtp.PlainAuth("", sender.SMTPUsername, smtpPassword, sender.SMTPHost)
// 		}

// 		// Handle different encryption types
// 		switch sender.Encryption {
// 		case "SSL", "TLS":
// 			conn, err := smtp.Dial(smtpAddr)
// 			if err != nil {
// 				smtpTestResult.Error = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
// 			} else {
// 				defer conn.Close()

// 				if err := conn.StartTLS(nil); err != nil {
// 					smtpTestResult.Error = fmt.Sprintf("Failed to start TLS: %v", err)
// 				} else if auth != nil {
// 					if err := conn.Auth(auth); err != nil {
// 						smtpTestResult.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
// 					} else {
// 						smtpTestResult.Success = true
// 					}
// 				} else {
// 					smtpTestResult.Success = true
// 				}
// 			}
// 		case "STARTTLS":
// 			conn, err := smtp.Dial("smtpAddr")
// 			if err != nil {
// 				smtpTestResult.Error = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
// 			} else {
// 				defer conn.Close()

// 				if err := conn.StartTLS(nil); err != nil {
// 					smtpTestResult.Error = fmt.Sprintf("Failed to start TLS: %v", err)
// 				} else if auth != nil {
// 					if err := conn.Auth(auth); err != nil {
// 						smtpTestResult.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
// 					} else {
// 						smtpTestResult.Success = true
// 					}
// 				} else {
// 					smtpTestResult.Success = true
// 				}
// 			}
// 		default:
// 			// No encryption
// 			conn, err := smtp.Dial(smtpAddr)
// 			if err != nil {
// 				smtpTestResult.Error = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
// 			} else {
// 				defer conn.Close()
// 				if auth != nil {
// 					if err := conn.Auth(auth); err != nil {
// 						smtpTestResult.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
// 					} else {
// 						smtpTestResult.Success = true
// 					}
// 				} else {
// 					smtpTestResult.Success = true
// 				}
// 			}
// 		}
// 	}

// 	var imapTestResult struct {
// 		Success bool
// 		Error   string
// 	}

// 	// Test IMAP connection if configured
// 	if sender.IMAPHost != "" && sender.IMAPUsername != "" {
// 		imapPassword, err := utils.Decrypt(sender.IMAPPassword)
// 		if err != nil {
// 			imapTestResult.Error = fmt.Sprintf("Failed to decrypt IMAP password: %v", err)
// 		} else {
// 			imapAddr := fmt.Sprintf("%s:%d", sender.IMAPHost, sender.IMAPPort)

// 			var c *client.Client
// 			switch sender.IMAPEncryption {
// 			case "SSL", "TLS":
// 				c, err = client.DialTLS(imapAddr, nil)
// 			case "STARTTLS":
// 				c, err = client.Dial(imapAddr)
// 				if err == nil {
// 					err = c.StartTLS(nil)
// 				}
// 			default:
// 				c, err = client.Dial(imapAddr)
// 			}

// 			if err != nil {
// 				imapTestResult.Error = fmt.Sprintf("Failed to connect to IMAP server: %v", err)
// 			} else {
// 				defer c.Logout()

// 				// Set timeout for login
// 				c.Timeout = 10 * time.Second

// 				if err := c.Login(sender.IMAPUsername, imapPassword); err != nil {
// 					imapTestResult.Error = fmt.Sprintf("IMAP authentication failed: %v", err)
// 				} else {
// 					imapTestResult.Success = true
// 				}
// 			}
// 		}
// 	}

// 	// Update sender verification status in database if SMTP test was successful
// 	if smtpTestResult.Success {
// 		if err := config.DB.Model(&sender).Update("SMTPVerified", true).Error; err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to update sender verification status",
// 			})
// 		}
// 	}

// 	return c.JSON(fiber.Map{
// 		"message": "Sender test completed",
// 		"status": fiber.Map{
// 			"smtp":          smtpTestResult,
// 			"imap":          imapTestResult,
// 			"smtp_verified": sender.SMTPVerified,
// 		},
// 	})
// }

func TestSender(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	senderID := c.Params("id")

	// Start transaction
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sender models.Sender
	if err := tx.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
		tx.Rollback()
		LogError("sender_not_found", err, map[string]interface{}{
			"user_id":   user.ID,
			"sender_id": senderID,
		})
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender not found",
		})
	}

	// Decrypt credentials
	smtpPassword, err := utils.Decrypt(sender.SMTPPassword)
	if err != nil {
		tx.Rollback()
		LogError("decrypt_failed", err, map[string]interface{}{
			"operation": "SMTP password decryption",
			"sender_id": sender.ID,
		})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to decrypt SMTP password",
		})
	}

	// Validate email format and domain
	if _, err := mail.ParseAddress(sender.FromEmail); err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid from email format",
		})
	}

	// DNS validation
	if hasMX, err := utils.ValidateMXRecords(sender.FromEmail); err != nil || !hasMX {
		tx.Rollback()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Domain MX records not found or invalid",
		})
	}

	var testResults struct {
		SMTP         TestResult `json:"smtp"`
		IMAP         TestResult `json:"imap"`
		SMTPVerified bool       `json:"smtp_verified"`
		EmailSent    bool       `json:"email_sent"`
	}

	// Test SMTP connection
	if sender.SMTPHost != "" {
		testResults.SMTP = testSMTPConnection(sender, smtpPassword)

		// Only try to send test email if SMTP test succeeded
		if testResults.SMTP.Success {
			testResults.EmailSent = sendTestEmail(sender, smtpPassword, user.Email)
		}
	}

	// Test IMAP connection if configured
	if sender.IMAPHost != "" {
		testResults.IMAP = testIMAPConnection(sender)
	}

	// Update verification status if successful
	if testResults.SMTP.Success && testResults.EmailSent {
		if err := tx.Model(&sender).Update("SMTPVerified", true).Error; err != nil {
			tx.Rollback()
			LogError("update_verification_failed", err, map[string]interface{}{
				"sender_id": sender.ID,
			})
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update verification status",
			})
		}
		testResults.SMTPVerified = true
	}

	tx.Commit()

	// Log test results
	LogEvent("sender_test_completed", map[string]interface{}{
		"sender_id":    sender.ID,
		"smtp_success": testResults.SMTP.Success,
		"email_sent":   testResults.EmailSent,
		"imap_success": testResults.IMAP.Success,
	})

	return c.JSON(fiber.Map{
		"message": "Sender test completed",
		"results": testResults,
	})
}

// LogError logs errors with structured context to both console and Sentry
func LogError(errorType string, err error, context map[string]interface{}) {
	// Initialize logger with structured fields
	log := logrus.WithFields(logrus.Fields{
		"error_type": errorType,
		"error":      err.Error(),
	})

	// Add additional context
	for k, v := range context {
		log = log.WithField(k, v)
	}

	// Log to console
	log.Error("Error occurred")

	// Send to Sentry
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("error_type", errorType)
		for k, v := range context {
			scope.SetExtra(k, v)
		}
		sentry.CaptureException(err)
	})
}

// LogEvent logs events with structured context
func LogEvent(eventType string, data map[string]interface{}) {
	log := logrus.WithFields(logrus.Fields{
		"event_type": eventType,
	})

	for k, v := range data {
		log = log.WithField(k, v)
	}

	log.Info("Event occurred")

	// Send to Sentry as breadcrumb
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:      "info",
		Category:  eventType,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// testSMTPConnection tests the SMTP server connection
func testSMTPConnection(sender models.Sender, password string) TestResult {
	result := TestResult{Success: false}

	// Prepare context for logging
	logContext := map[string]interface{}{
		"smtp_host": sender.SMTPHost,
		"smtp_port": sender.SMTPPort,
		"username":  sender.SMTPUsername,
	}

	// Construct SMTP address
	smtpAddr := fmt.Sprintf("%s:%d", sender.SMTPHost, sender.SMTPPort)

	// Prepare authentication
	var auth smtp.Auth
	if sender.SMTPUsername != "" && password != "" {
		auth = smtp.PlainAuth("", sender.SMTPUsername, password, sender.SMTPHost)
	}

	// Handle different encryption types
	switch strings.ToUpper(sender.Encryption) {
	case "SSL", "TLS":
		// Create TLS configuration
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         sender.SMTPHost,
		}

		conn, err := tls.Dial("tcp", smtpAddr, tlsConfig)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to establish TLS connection: %v", err)
			LogError("smtp_tls_connection", err, logContext)
			return result
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, sender.SMTPHost)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to create SMTP client: %v", err)
			LogError("smtp_client_creation", err, logContext)
			return result
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				result.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
				LogError("smtp_authentication", err, logContext)
				return result
			}
		}
		result.Success = true

	case "STARTTLS":
		client, err := smtp.Dial(smtpAddr)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
			LogError("smtp_connection", err, logContext)
			return result
		}
		defer client.Close()

		if err := client.StartTLS(&tls.Config{
			InsecureSkipVerify: false,
			ServerName:         sender.SMTPHost,
		}); err != nil {
			result.Error = fmt.Sprintf("Failed to start TLS: %v", err)
			LogError("smtp_starttls", err, logContext)
			return result
		}

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				result.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
				LogError("smtp_authentication", err, logContext)
				return result
			}
		}
		result.Success = true

	default:
		// No encryption
		client, err := smtp.Dial(smtpAddr)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to connect to SMTP server: %v", err)
			LogError("smtp_connection", err, logContext)
			return result
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				result.Error = fmt.Sprintf("SMTP authentication failed: %v", err)
				LogError("smtp_authentication", err, logContext)
				return result
			}
		}
		result.Success = true
	}

	LogEvent("smtp_test_success", logContext)
	return result
}

// // sendTestEmail sends a test email using the provided sender configuration
// func sendTestEmail(sender models.Sender, password string, toEmail string) bool {
// 	logContext := map[string]interface{}{
// 		"smtp_host": sender.SMTPHost,
// 		"smtp_port": sender.SMTPPort,
// 		"username":  sender.SMTPUsername,
// 		"to_email":  toEmail,
// 	}

// 	// Create new email message
// 	m := gomail.NewMessage()
// 	m.SetHeader("From", sender.FromEmail)
// 	m.SetHeader("To", toEmail)
// 	m.SetHeader("Subject", "Test Email from Your App")
// 	m.SetBody("text/plain", "This is a test email to verify your SMTP configuration.")

// 	// Configure dialer
// 	d := gomail.NewDialer(
// 		sender.SMTPHost,
// 		sender.SMTPPort,
// 		sender.SMTPUsername,
// 		password,
// 	)

// 	// Set encryption
// 	switch strings.ToUpper(sender.Encryption) {
// 	case "SSL", "TLS":
// 		d.SSL = true
// 	case "STARTTLS":
// 		d.TLSConfig = &tls.Config{InsecureSkipVerify: false, ServerName: sender.SMTPHost}
// 	default:
// 		d.SSL = false
// 	}

// 	// Set timeout
// 	d.Timeout = 10 * time.Second

// 	// Send email
// 	if err := d.DialAndSend(m); err != nil {
// 		LogError("send_test_email", err, logContext)
// 		return false
// 	}

//		LogEvent("test_email_sent", logContext)
//		return true
//	}
//
// sendTestEmail sends a test email using the provided sender configuration
func sendTestEmail(sender models.Sender, password string, toEmail string) bool {
	logContext := map[string]interface{}{
		"smtp_host": sender.SMTPHost,
		"smtp_port": sender.SMTPPort,
		"username":  sender.SMTPUsername,
		"to_email":  toEmail,
	}

	// Create new email message
	m := gomail.NewMessage()
	m.SetHeader("From", sender.FromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Test Email from Your App")
	m.SetBody("text/plain", "This is a test email to verify your SMTP configuration.")

	// Configure dialer
	d := gomail.NewDialer(
		sender.SMTPHost,
		sender.SMTPPort,
		sender.SMTPUsername,
		password,
	)

	// Set encryption
	switch strings.ToUpper(sender.Encryption) {
	case "SSL", "TLS":
		d.SSL = true
	case "STARTTLS":
		d.TLSConfig = &tls.Config{InsecureSkipVerify: false, ServerName: sender.SMTPHost}
	default:
		d.SSL = false
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send email with context
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.DialAndSend(m)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			LogError("send_test_email", err, logContext)
			return false
		}
	case <-ctx.Done():
		LogError("send_test_email_timeout", ctx.Err(), logContext)
		return false
	}

	LogEvent("test_email_sent", logContext)
	return true
}

// testIMAPConnection tests the IMAP server connection
func testIMAPConnection(sender models.Sender) TestResult {
	result := TestResult{Success: false}

	logContext := map[string]interface{}{
		"imap_host": sender.IMAPHost,
		"imap_port": sender.IMAPPort,
		"username":  sender.IMAPUsername,
	}

	imapPassword, err := utils.Decrypt(sender.IMAPPassword)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to decrypt IMAP password: %v", err)
		LogError("imap_password_decrypt", err, logContext)
		return result
	}

	imapAddr := fmt.Sprintf("%s:%d", sender.IMAPHost, sender.IMAPPort)
	var c *client.Client

	switch strings.ToUpper(sender.IMAPEncryption) {
	case "SSL", "TLS":
		c, err = client.DialTLS(imapAddr, &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         sender.IMAPHost,
		})
	case "STARTTLS":
		c, err = client.Dial(imapAddr)
		if err == nil {
			err = c.StartTLS(&tls.Config{
				InsecureSkipVerify: false,
				ServerName:         sender.IMAPHost,
			})
		}
	default:
		c, err = client.Dial(imapAddr)
	}

	if err != nil {
		result.Error = fmt.Sprintf("Failed to connect to IMAP server: %v", err)
		LogError("imap_connection", err, logContext)
		return result
	}
	defer c.Logout()

	// Set timeout for login
	c.Timeout = 10 * time.Second

	if err := c.Login(sender.IMAPUsername, imapPassword); err != nil {
		result.Error = fmt.Sprintf("IMAP authentication failed: %v", err)
		LogError("imap_authentication", err, logContext)
		return result
	}

	// Test mailbox selection if specified
	if sender.IMAPMailbox != "" {
		_, err = c.Select(sender.IMAPMailbox, false)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to select mailbox: %v", err)
			LogError("imap_mailbox_select", err, logContext)
			return result
		}
	}

	result.Success = true
	LogEvent("imap_test_success", logContext)
	return result
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

	// Update sender is already tested in TestSenderTestSender, so we just need to check the verification status
	if !sender.SMTPVerified {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Sender SMTP settings not verified. Please run test first",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Sender verified successfully",
		"data": fiber.Map{
			"smtp_verified": true,
		},
	})
}















// package controller

// import (
// 	"fmt"
// 	"strconv"

// 	"github.com/gofiber/fiber/v2"
// 	"mailnexy/config"
// 	"mailnexy/models"
// 	"mailnexy/utils"
// )

// type CreateSenderRequest struct {
// 	Name              string `json:"name" validate:"required"`
// 	FromEmail         string `json:"from_email" validate:"required,email"`
// 	FromName          string `json:"from_name" validate:"required"`
// 	ProviderType      string `json:"provider_type" validate:"required,oneof=smtp gmail outlook yahoo custom"`
// 	SMTPHost          string `json:"smtp_host" validate:"required_if=ProviderType smtp"`
// 	SMTPPort          int    `json:"smtp_port" validate:"required_if=ProviderType smtp"`
// 	SMTPUsername      string `json:"smtp_username" validate:"required_if=ProviderType smtp"`
// 	SMTPPassword      string `json:"smtp_password" validate:"required_if=ProviderType smtp"`
// 	Encryption        string `json:"encryption" validate:"required_if=ProviderType smtp,oneof=SSL TLS STARTTLS"`
// 	IMAPHost          string `json:"imap_host"`
// 	IMAPPort          int    `json:"imap_port"`
// 	IMAPUsername      string `json:"imap_username"`
// 	IMAPPassword      string `json:"imap_password"`
// 	IMAPEncryption    string `json:"imap_encryption" validate:"omitempty,oneof=SSL TLS STARTTLS"`
// 	IMAPMailbox       string `json:"imap_mailbox"`
// 	OAuthProvider     string `json:"oauth_provider"`
// 	OAuthToken        string `json:"oauth_token"`
// 	OAuthRefreshToken string `json:"oauth_refresh_token"`
// 	TrackOpens        bool   `json:"track_opens"`
// 	TrackClicks       bool   `json:"track_clicks"`
// 	TrackReplies      bool   `json:"track_replies"`
// }

// type UpdateSenderRequest struct {
// 	Name              *string `json:"name"`
// 	FromEmail         *string `json:"from_email" validate:"omitempty,email"`
// 	FromName          *string `json:"from_name"`
// 	SMTPPassword      *string `json:"smtp_password"`
// 	IMAPPassword      *string `json:"imap_password"`
// 	OAuthToken        *string `json:"oauth_token"`
// 	OAuthRefreshToken *string `json:"oauth_refresh_token"`
// 	TrackOpens        *bool   `json:"track_opens"`
// 	TrackClicks       *bool   `json:"track_clicks"`
// 	TrackReplies      *bool   `json:"track_replies"`
// }

// func CreateSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var req CreateSenderRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	// Validate request
// 	if err := utils.ValidateStruct(req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	// Encrypt sensitive data
// 	encryptedSMTPPassword, err := utils.Encrypt(req.SMTPPassword)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to encrypt SMTP password",
// 		})
// 	}

// 	encryptedIMAPPassword, err := utils.Encrypt(req.IMAPPassword)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to encrypt IMAP password",
// 		})
// 	}

// 	encryptedOAuthToken, err := utils.Encrypt(req.OAuthToken)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to encrypt OAuth token",
// 		})
// 	}

// 	encryptedRefreshToken, err := utils.Encrypt(req.OAuthRefreshToken)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to encrypt OAuth refresh token",
// 		})
// 	}

// 	// Create sender
// 	sender := models.Sender{
// 		UserID:            user.ID,
// 		Name:              req.Name,
// 		FromEmail:         req.FromEmail,
// 		FromName:          req.FromName,
// 		ProviderType:      req.ProviderType,
// 		SMTPHost:          req.SMTPHost,
// 		SMTPPort:          req.SMTPPort,
// 		SMTPUsername:      req.SMTPUsername,
// 		SMTPPassword:      encryptedSMTPPassword,
// 		Encryption:        req.Encryption,
// 		IMAPHost:          req.IMAPHost,
// 		IMAPPort:          req.IMAPPort,
// 		IMAPUsername:      req.IMAPUsername,
// 		IMAPPassword:      encryptedIMAPPassword,
// 		IMAPEncryption:    req.IMAPEncryption,
// 		IMAPMailbox:       req.IMAPMailbox,
// 		OAuthProvider:     req.OAuthProvider,
// 		OAuthToken:        encryptedOAuthToken,
// 		OAuthRefreshToken: encryptedRefreshToken,
// 		TrackOpens:        req.TrackOpens,
// 		TrackClicks:       req.TrackClicks,
// 		TrackReplies:      req.TrackReplies,
// 	}

// 	if err := config.DB.Create(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create sender",
// 		})
// 	}

// 	// Sanitize before returning
// 	sender.Sanitize()

// 	return c.Status(fiber.StatusCreated).JSON(sender)
// }

// func GetSenders(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var senders []models.Sender
// 	if err := config.DB.Where("user_id = ?", user.ID).Find(&senders).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to fetch senders",
// 		})
// 	}

// 	// Sanitize all senders
// 	for i := range senders {
// 		senders[i].Sanitize()
// 	}

// 	return c.JSON(senders)
// }

// func validateSenderID(id string) error {
// 	if id == "" || id == "undefined" {
// 		return fiber.NewError(fiber.StatusBadRequest, "Invalid sender ID")
// 	}
// 	// Add check for numeric ID if your DB uses numeric IDs
// 	if _, err := strconv.Atoi(id); err != nil {
// 		return fiber.NewError(fiber.StatusBadRequest, "Sender ID must be numeric")
// 	}
// 	return nil
// }

// func GetSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	if err := validateSenderID(senderID); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	sender.Sanitize()
// 	return c.JSON(sender)
// }

// func UpdateSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	if err := validateSenderID(senderID); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	var req UpdateSenderRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	if err := utils.ValidateStruct(req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	// Update fields
// 	if req.Name != nil {
// 		sender.Name = *req.Name
// 	}
// 	if req.FromEmail != nil {
// 		sender.FromEmail = *req.FromEmail
// 	}
// 	if req.FromName != nil {
// 		sender.FromName = *req.FromName
// 	}
// 	if req.SMTPPassword != nil {
// 		encrypted, err := utils.Encrypt(*req.SMTPPassword)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to encrypt SMTP password",
// 			})
// 		}
// 		sender.SMTPPassword = encrypted
// 	}
// 	if req.IMAPPassword != nil {
// 		encrypted, err := utils.Encrypt(*req.IMAPPassword)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to encrypt IMAP password",
// 			})
// 		}
// 		sender.IMAPPassword = encrypted
// 	}
// 	if req.OAuthToken != nil {
// 		encrypted, err := utils.Encrypt(*req.OAuthToken)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to encrypt OAuth token",
// 			})
// 		}
// 		sender.OAuthToken = encrypted
// 	}
// 	if req.OAuthRefreshToken != nil {
// 		encrypted, err := utils.Encrypt(*req.OAuthRefreshToken)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to encrypt OAuth refresh token",
// 			})
// 		}
// 		sender.OAuthRefreshToken = encrypted
// 	}
// 	if req.TrackOpens != nil {
// 		sender.TrackOpens = *req.TrackOpens
// 	}
// 	if req.TrackClicks != nil {
// 		sender.TrackClicks = *req.TrackClicks
// 	}
// 	if req.TrackReplies != nil {
// 		sender.TrackReplies = *req.TrackReplies
// 	}

// 	if err := config.DB.Save(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update sender",
// 		})
// 	}

// 	sender.Sanitize()
// 	return c.JSON(sender)
// }

// func DeleteSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	// Add logging to see what ID is received
// 	fmt.Printf("Attempting to delete sender with ID: %v (type: %T)\n", senderID, senderID)

// 	if err := validateSenderID(senderID); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		fmt.Printf("Error finding sender: %v\n", err)
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	if err := config.DB.Delete(&sender).Error; err != nil {
// 		fmt.Printf("Error deleting sender: %v\n", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to delete sender",
// 		})
// 	}

// 	fmt.Printf("Successfully deleted sender with ID: %v\n", senderID)
// 	return c.SendStatus(fiber.StatusNoContent)
// }
// func TestSender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	// TODO: Implement actual SMTP/IMAP testing logic
// 	// This would involve connecting to the SMTP/IMAP server with the provided credentials
// 	// and verifying they work correctly

// 	// For now, just return success
// 	return c.JSON(fiber.Map{
// 		"message": "Sender test initiated",
// 		"status":  "pending",
// 	})
// }

// func VerifySender(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	senderID := c.Params("id")

// 	if err := validateSenderID(senderID); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	var sender models.Sender
// 	if err := config.DB.Where("id = ? AND user_id = ?", senderID, user.ID).First(&sender).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Sender not found",
// 		})
// 	}

// 	// TODO: Implement actual SMTP verification logic
// 	// This would involve connecting to the SMTP server and verifying the credentials work

// 	// For now, we'll just mark it as verified
// 	if err := config.DB.Model(&sender).Update("smtp_verified", true).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to verify sender",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"message": "Sender verified successfully",
// 		"data": fiber.Map{
// 			"smtp_verified": true,
// 		},
// 	})
// }