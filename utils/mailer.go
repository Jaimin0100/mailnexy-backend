package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"time"

	"gopkg.in/gomail.v2"
)

type EmailData struct {
	Subject   string
	To        []string
	CC        []string
	BCC       []string
	Template  string
	Data      interface{}
	Year      int
	FromName  string
	FromEmail string
	OTP       string
}

const (
	maxRetries     = 3
	retryDelayBase = 1 * time.Second
)

// Embedded email templates (same as before)
var emailTemplates = map[string]string{
	"otp": `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
        .content { margin: 20px 0; }
        .otp-code { font-size: 24px; font-weight: bold; color: #3498db; margin: 20px 0; text-align: center; }
        .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
    </style>
</head>
<body>
    <div class="header">
        <h2>Your Verification Code</h2>
    </div>
    
    <div class="content">
        <p>Hello,</p>
        <p>Here is your one-time verification code:</p>
        
        <div class="otp-code">{{.OTP}}</div>
        
        <p>This code will expire in 15 minutes. Please don't share this code with anyone.</p>
    </div>
    
    <div class="footer">
        <p>If you didn't request this code, you can safely ignore this email.</p>
        <p>© {{.Year}} Your App Name. All rights reserved.</p>
    </div>
</body>
</html>`,

	"password_reset": `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
        .content { margin: 20px 0; }
        .button { display: inline-block; padding: 10px 20px; background-color: #3498db; color: white; text-decoration: none; border-radius: 4px; }
        .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
    </style>
</head>
<body>
    <div class="header">
        <h2>Password Reset Request</h2>
    </div>
    
    <div class="content">
        <p>Hello,</p>
        <p>We received a request to reset your password. Click the button below to proceed:</p>
        
        <p style="text-align: center;">
            <a href="{{.ResetLink}}" class="button">Reset Password</a>
        </p>
        
        <p>If you didn't request a password reset, please ignore this email. This link will expire in 24 hours.</p>
        
        <p>Or copy and paste this link into your browser:<br>
        <small>{{.ResetLink}}</small></p>
    </div>
    
    <div class="footer">
        <p>For security reasons, don't share this link with anyone.</p>
        <p>© {{.Year}} Your App Name. All rights reserved.</p>
    </div>
</body>
</html>`,

	"password_reset_otp": `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
        .content { margin: 20px 0; }
        .otp-code { font-size: 24px; font-weight: bold; color: #e74c3c; margin: 20px 0; text-align: center; }
        .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
    </style>
</head>
<body>
    <div class="header">
        <h2>Password Reset Request</h2>
    </div>
    
    <div class="content">
        <p>Hello,</p>
        <p>We received a request to reset your password. Here is your verification code:</p>
        
        <div class="otp-code">{{.OTP}}</div>
        
        <p>This code will expire in 15 minutes. If you didn't request a password reset, please ignore this email.</p>
    </div>
    
    <div class="footer">
        <p>For security reasons, don't share this code with anyone.</p>
        <p>© {{.Year}} Your App Name. All rights reserved.</p>
    </div>
</body>
</html>`,
}

// SendEmail sends an email with retry logic and improved error handling
func SendEmail(data EmailData) error {
	// Validate required fields
	if len(data.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}
	if data.Template == "" {
		return fmt.Errorf("email template not specified")
	}

	// Set default values
	if data.FromEmail == "" {
		data.FromEmail = os.Getenv("SMTP_FROM_EMAIL")
		if data.FromEmail == "" {
			return fmt.Errorf("SMTP_FROM_EMAIL not configured")
		}
	}
	if data.FromName == "" {
		data.FromName = os.Getenv("SMTP_FROM_NAME")
	}
	if data.Year == 0 {
		data.Year = time.Now().Year()
	}

	// Validate SMTP configuration
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		return fmt.Errorf("SMTP_HOST not configured")
	}

	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr == "" {
		return fmt.Errorf("SMTP_PORT not configured")
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %v", err)
	}

	smtpUser := os.Getenv("SMTP_USERNAME")
	smtpPass := os.Getenv("SMTP_PASSWORD")

	// Get template
	tmplContent, ok := emailTemplates[data.Template]
	if !ok {
		return fmt.Errorf("template '%s' not found", data.Template)
	}

	// Parse template
	tmpl, err := template.New("email").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	// Execute template
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", data.FromName, data.FromEmail))
	m.SetHeader("To", data.To...)
	if len(data.CC) > 0 {
		m.SetHeader("Cc", data.CC...)
	}
	if len(data.BCC) > 0 {
		m.SetHeader("Bcc", data.BCC...)
	}
	m.SetHeader("Subject", data.Subject)
	m.SetBody("text/html", body.String())

	// Configure dialer
	d := gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)

	// For timeout, you can set it on the underlying SMTP client
	d.SSL = false     // Set to true if using SSL
	d.TLSConfig = nil // Add your TLS config if needed

	// Retry logic
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a new SMTP client for each attempt
		var sendCloser gomail.SendCloser
		var err error

		if attempt == 1 {
			// First attempt - try to create new connection
			sendCloser, err = d.Dial()
		} else {
			// Subsequent attempts - try to reuse existing connection
			if sendCloser != nil {
				sendCloser, err = d.Dial()
			}
		}

		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				retryDelay := time.Duration(attempt) * retryDelayBase
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("after %d attempts: %v", maxRetries, err)
		}

		// Try to send the email
		if err := gomail.Send(sendCloser, m); err != nil {
			lastErr = err
			if attempt < maxRetries {
				retryDelay := time.Duration(attempt) * retryDelayBase
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("after %d attempts: %v", maxRetries, err)
		}

		// If we get here, the email was sent successfully
		return nil
	}

	return lastErr
}
