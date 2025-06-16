

package utils

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"mailnexy/models"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type WarmupMailer struct {
	db                 *gorm.DB
	validDomains       []string
	blacklistedDomains []string
	warmupEmail        string // Specific email for warmup
}

func NewWarmupMailer(db *gorm.DB, warmupEmail string) *WarmupMailer {
	return &WarmupMailer{
		db: db,

		validDomains: []string{
			"gmail.com", "yahoo.com", "outlook.com",
			"protonmail.com", "icloud.com", "aol.com",
			"hotmail.com", "mail.com", "zoho.com",
		},
		blacklistedDomains: []string{
			"mailinator", "temp", "fake", "example",
			"test", "demo", "trash", "throwaway",
		},
		warmupEmail: warmupEmail, // Stores the specific email address
	}
}

func (wm *WarmupMailer) SendWarmupEmail(senderID uint, fromEmail, fromName string) error {
	// Get the sender's SMTP configuration from the database
	var sender models.Sender
	if err := wm.db.First(&sender, senderID).Error; err != nil {
		return fmt.Errorf("failed to fetch sender SMTP config: %v", err)
	}

	// Decrypt the SMTP password
	decryptedPassword, err := Decrypt(sender.SMTPPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt SMTP password: %v", err)
	}

	// Create a dialer with the sender's SMTP configuration
	dialer := gomail.NewDialer(
		sender.SMTPHost,
		sender.SMTPPort,
		sender.SMTPUsername,
		decryptedPassword,
	)

	// For older versions of gomail, you can set the timeout this way:
	dialer.LocalName = "localhost" // Set this to your domain if needed
	dialer.TLSConfig = &tls.Config{ServerName: sender.SMTPHost}

	// The timeout is handled internally by gomail in this version
	// If you need more control, consider upgrading to a newer version of gomail

	maxRetries := 3
	var lastError error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt*attempt) * time.Second
			time.Sleep(backoff)
		}

		err := wm.trySend(dialer, fromEmail, fromName)
		if err == nil {
			return nil
		}

		lastError = err
		if !wm.isTemporaryError(err) {
			break // Permanent error, don't retry
		}
	}

	return fmt.Errorf("failed after %d attempts: %v", maxRetries, lastError)
}

func (wm *WarmupMailer) trySend(dialer *gomail.Dialer, fromEmail, fromName string) error {
	subject, body := wm.generateWarmupContent(fromName)

	// Use the configured warmup email instead of random generation
	toEmail := wm.warmupEmail // << USES THE STORED SPECIFIC EMAIL

	// Validate email format before sending
	if !wm.isValidEmailFormat(toEmail) {
		return fmt.Errorf("invalid warmup email format: %s", toEmail)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", fromName, fromEmail))
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	// Add professional headers
	m.SetHeader("X-Mailer", "MailWarmupSystem/1.0")
	m.SetHeader("X-Priority", "3") // Normal priority
	m.SetHeader("Auto-Submitted", "auto-generated")
	m.SetHeader("X-Warmup", "true") // Special header to identify warmup emails

	// Add random delay between 3-10 seconds
	delay := time.Duration(3+rand.Intn(7)) * time.Second
	time.Sleep(delay)

	// Test connection first
	if _, err := dialer.Dial(); err != nil {
		return fmt.Errorf("SMTP connection failed: %v", err)
	}

	if err := dialer.DialAndSend(m); err != nil {
		return fmt.Errorf("send failed: %v", err)
	}

	return nil
}

func (wm *WarmupMailer) generateWarmupContent(fromName string) (string, string) {
	subjects := []string{
		"Quick question about your recent post",
		"Following up on our last conversation",
		"Checking in to see how you're doing",
		"Thought you might find this interesting",
		"Let's reconnect soon",
		"An idea I wanted to share with you",
		"Regarding your recent project",
	}

	bodies := []string{
		"Hi there,\n\nI wanted to follow up on our previous conversation. Let me know if you have any questions!\n\nBest regards,\n%s",
		"Hello,\n\nI came across this and thought you might find it valuable. What do you think?\n\nRegards,\n%s",
		"Hi,\n\nJust checking in to see if you had any thoughts on this topic?\n\nThanks,\n%s",
		"Greetings,\n\nI wanted to share this with you. Let me know your thoughts when you get a chance.\n\nBest,\n%s",
		"Hello,\n\nHope this message finds you well. I wanted to touch base about...\n\nWarm regards,\n%s",
	}

	subject := subjects[rand.Intn(len(subjects))]
	body := fmt.Sprintf(bodies[rand.Intn(len(bodies))], fromName)

	return subject, body
}

func (wm *WarmupMailer) generateRandomEmail() string {
	prefixes := []string{
		"user", "contact", "hello", "info", "support",
		"team", "sales", "help", "service", "admin",
		"news", "notifications", "inbox",
	}

	prefix := prefixes[rand.Intn(len(prefixes))]
	number := rand.Intn(1000) // Smaller number range looks more natural
	domain := wm.validDomains[rand.Intn(len(wm.validDomains))]

	return fmt.Sprintf("%s%d@%s", prefix, number, domain)
}

func (wm *WarmupMailer) isValidEmailFormat(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	// Check domain is not blacklisted
	for _, blacklisted := range wm.blacklistedDomains {
		if strings.Contains(strings.ToLower(parts[1]), blacklisted) {
			return false
		}
	}

	// Simple domain format validation
	if _, err := net.LookupMX(parts[1]); err != nil {
		return false
	}

	return true
}

func (wm *WarmupMailer) isTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network/timeout errors that might be temporary
	if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
		return true
	}

	// Check for SMTP error codes that indicate temporary failures
	errStr := strings.ToLower(err.Error())
	tempErrors := []string{
		"try again",
		"temporary",
		"4.",
		"421",
		"450",
		"451",
		"452",
	}

	for _, tempErr := range tempErrors {
		if strings.Contains(errStr, tempErr) {
			return true
		}
	}

	return false
}