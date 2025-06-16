package utils

import (
	"fmt"
	"net/smtp"
)

// EmailConfig holds SMTP configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
}

var emailConfig EmailConfig

// InitEmailConfig initializes email configuration
func InitEmailConfig(host, port, username, password, from string) {
	emailConfig = EmailConfig{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUsername: username,
		SMTPPassword: password,
		FromEmail:    from,
	}
}

// SendOTPEmail sends an OTP verification email
func SendOTPEmail(to, otp string) error {
	subject := "Your Verification Code"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Your Verification Code</h2>
			<p>Please use the following code to verify your account:</p>
			<h3>%s</h3>
			<p>This code will expire in 15 minutes.</p>
		</body>
		</html>
	`, otp)

	return sendEmail(to, subject, body)
}

// SendPasswordResetOTPEmail sends a password reset OTP email
func SendPasswordResetOTPEmail(to, otp string) error {
	subject := "Password Reset Code"
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>Please use the following code to reset your password:</p>
			<h3>%s</h3>
			<p>This code will expire in 15 minutes.</p>
			<p>If you didn't request this, please ignore this email.</p>
		</body>
		</html>
	`, otp)

	return sendEmail(to, subject, body)
}

// sendEmail is a helper function to send emails
func sendEmail(to, subject, body string) error {
	// Check if email config is initialized
	if emailConfig.SMTPHost == "" {
		return fmt.Errorf("email configuration not initialized")
	}

	// Set up authentication
	auth := smtp.PlainAuth("", emailConfig.SMTPUsername, emailConfig.SMTPPassword, emailConfig.SMTPHost)

	// Prepare message
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n"+
		"\r\n"+
		"%s\r\n", to, emailConfig.FromEmail, subject, body))

	// Send email
	err := smtp.SendMail(
		fmt.Sprintf("%s:%s", emailConfig.SMTPHost, emailConfig.SMTPPort),
		auth,
		emailConfig.FromEmail,
		[]string{to},
		msg,
	)

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
