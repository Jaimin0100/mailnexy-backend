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
}

// Embedded email templates
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

func SendEmail(data EmailData) error {
	// Set default from email if not provided
	if data.FromEmail == "" {
		data.FromEmail = os.Getenv("SMTP_FROM_EMAIL")
	}
	if data.FromName == "" {
		data.FromName = os.Getenv("SMTP_FROM_NAME")
	}

	// Get template from embedded templates
	tmplContent, ok := emailTemplates[data.Template]
	if !ok {
		return fmt.Errorf("template '%s' not found", data.Template)
	}

	// Parse template
	tmpl, err := template.New("email").Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data.Data); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// Convert SMTP_PORT to int
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP_PORT: %v", err)
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

	// Debug: Log environment variables
	fmt.Println("[DEBUG] SMTP Configuration:")
	fmt.Println("SMTP_HOST:", os.Getenv("SMTP_HOST"))
	fmt.Println("SMTP_PORT:", os.Getenv("SMTP_PORT"))
	fmt.Println("SMTP_USERNAME:", os.Getenv("SMTP_USERNAME"))
	fmt.Println("SMTP_FROM_EMAIL:", os.Getenv("SMTP_FROM_EMAIL"))
	fmt.Println("Using template:", data.Template)

	// Send email
	d := gomail.NewDialer(
		os.Getenv("SMTP_HOST"),
		smtpPort,
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		fmt.Println("[DEBUG] SMTP Error:", err)
		return fmt.Errorf("error sending email: %v", err)
	}

	fmt.Println("[DEBUG] Email sent successfully!")
	return nil
}

func SendOTPEmail(email, otp string) error {
	data := EmailData{
		Subject:  "Your Verification Code",
		To:       []string{email},
		Template: "otp",
		Year:     time.Now().Year(),
		Data: struct {
			OTP string
		}{
			OTP: otp,
		},
	}

	return SendEmail(data)
}

func SendPasswordResetEmail(email, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("APP_URL"), token)
	data := EmailData{
		Subject:  "Password Reset Request",
		To:       []string{email},
		Template: "password_reset",
		Data: struct {
			ResetLink string
		}{
			ResetLink: resetLink,
		},
	}

	return SendEmail(data)
}

func SendPasswordResetOTPEmail(email, otp string) error {
	data := EmailData{
		Subject:  "Password Reset Verification Code",
		To:       []string{email},
		Template: "password_reset_otp",
		Data: struct {
			OTP string
		}{
			OTP: otp,
		},
	}

	return SendEmail(data)
}













// package utils

// import (
// 	"bytes"
// 	"fmt"
// 	"html/template"
// 	"os"
// 	"strconv"
// 	"time"

// 	"gopkg.in/gomail.v2"
// )

// type EmailData struct {
// 	Subject   string
// 	To        []string
// 	CC        []string
// 	BCC       []string
// 	Template  string
// 	Data      interface{}
// 	Year      int
// 	FromName  string
// 	FromEmail string
// }

// // Embedded email templates
// var emailTemplates = map[string]string{
// 	"otp": `<!DOCTYPE html>
// <html>
// <head>
//     <meta charset="UTF-8">
//     <title>{{.Subject}}</title>
//     <style>
//         body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
//         .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
//         .content { margin: 20px 0; }
//         .otp-code { font-size: 24px; font-weight: bold; color: #3498db; margin: 20px 0; text-align: center; }
//         .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
//     </style>
// </head>
// <body>
//     <div class="header">
//         <h2>Your Verification Code</h2>
//     </div>
    
//     <div class="content">
//         <p>Hello,</p>
//         <p>Here is your one-time verification code:</p>
        
//         <div class="otp-code">{{.OTP}}</div>
        
//         <p>This code will expire in 15 minutes. Please don't share this code with anyone.</p>
//     </div>
    
//     <div class="footer">
//         <p>If you didn't request this code, you can safely ignore this email.</p>
//         <p>© {{.Year}} Your App Name. All rights reserved.</p>
//     </div>
// </body>
// </html>`,

// 	"password_reset": `<!DOCTYPE html>
// <html>
// <head>
//     <meta charset="UTF-8">
//     <title>{{.Subject}}</title>
//     <style>
//         body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
//         .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
//         .content { margin: 20px 0; }
//         .button { display: inline-block; padding: 10px 20px; background-color: #3498db; color: white; text-decoration: none; border-radius: 4px; }
//         .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
//     </style>
// </head>
// <body>
//     <div class="header">
//         <h2>Password Reset Request</h2>
//     </div>
    
//     <div class="content">
//         <p>Hello,</p>
//         <p>We received a request to reset your password. Click the button below to proceed:</p>
        
//         <p style="text-align: center;">
//             <a href="{{.ResetLink}}" class="button">Reset Password</a>
//         </p>
        
//         <p>If you didn't request a password reset, please ignore this email. This link will expire in 24 hours.</p>
        
//         <p>Or copy and paste this link into your browser:<br>
//         <small>{{.ResetLink}}</small></p>
//     </div>
    
//     <div class="footer">
//         <p>For security reasons, don't share this link with anyone.</p>
//         <p>© {{.Year}} Your App Name. All rights reserved.</p>
//     </div>
// </body>
// </html>`,

// 	"password_reset_otp": `<!DOCTYPE html>
// <html>
// <head>
//     <meta charset="UTF-8">
//     <title>{{.Subject}}</title>
//     <style>
//         body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
//         .header { color: #2c3e50; border-bottom: 1px solid #eee; padding-bottom: 10px; }
//         .content { margin: 20px 0; }
//         .otp-code { font-size: 24px; font-weight: bold; color: #e74c3c; margin: 20px 0; text-align: center; }
//         .footer { margin-top: 30px; font-size: 12px; color: #7f8c8d; text-align: center; }
//     </style>
// </head>
// <body>
//     <div class="header">
//         <h2>Password Reset Request</h2>
//     </div>
    
//     <div class="content">
//         <p>Hello,</p>
//         <p>We received a request to reset your password. Here is your verification code:</p>
        
//         <div class="otp-code">{{.OTP}}</div>
        
//         <p>This code will expire in 15 minutes. If you didn't request a password reset, please ignore this email.</p>
//     </div>
    
//     <div class="footer">
//         <p>For security reasons, don't share this code with anyone.</p>
//         <p>© {{.Year}} Your App Name. All rights reserved.</p>
//     </div>
// </body>
// </html>`,
// }

// func SendEmail(data EmailData) error {
// 	// Set default from email if not provided
// 	if data.FromEmail == "" {
// 		data.FromEmail = os.Getenv("SMTP_FROM_EMAIL")
// 	}
// 	if data.FromName == "" {
// 		data.FromName = os.Getenv("SMTP_FROM_NAME")
// 	}

// 	// Get template from embedded templates
// 	tmplContent, ok := emailTemplates[data.Template]
// 	if !ok {
// 		return fmt.Errorf("template '%s' not found", data.Template)
// 	}

// 	// Parse template
// 	tmpl, err := template.New("email").Parse(tmplContent)
// 	if err != nil {
// 		return fmt.Errorf("error parsing template: %v", err)
// 	}

// 	var body bytes.Buffer
// 	if err := tmpl.Execute(&body, data.Data); err != nil {
// 		return fmt.Errorf("error executing template: %v", err)
// 	}

// 	// Convert SMTP_PORT to int
// 	smtpPortStr := os.Getenv("SMTP_PORT")
// 	smtpPort, err := strconv.Atoi(smtpPortStr)
// 	if err != nil {
// 		return fmt.Errorf("invalid SMTP_PORT: %v", err)
// 	}

// 	// Create email message
// 	m := gomail.NewMessage()
// 	m.SetHeader("From", fmt.Sprintf("%s <%s>", data.FromName, data.FromEmail))
// 	m.SetHeader("To", data.To...)
// 	if len(data.CC) > 0 {
// 		m.SetHeader("Cc", data.CC...)
// 	}
// 	if len(data.BCC) > 0 {
// 		m.SetHeader("Bcc", data.BCC...)
// 	}
// 	m.SetHeader("Subject", data.Subject)
// 	m.SetBody("text/html", body.String())

// 	// Debug: Log environment variables
// 	fmt.Println("[DEBUG] SMTP Configuration:")
// 	fmt.Println("SMTP_HOST:", os.Getenv("SMTP_HOST"))
// 	fmt.Println("SMTP_PORT:", os.Getenv("SMTP_PORT"))
// 	fmt.Println("SMTP_USERNAME:", os.Getenv("SMTP_USERNAME"))
// 	fmt.Println("SMTP_FROM_EMAIL:", os.Getenv("SMTP_FROM_EMAIL"))
// 	fmt.Println("Using template:", data.Template)

// 	// Send email
// 	d := gomail.NewDialer(
// 		os.Getenv("SMTP_HOST"),
// 		smtpPort,
// 		os.Getenv("SMTP_USERNAME"),
// 		os.Getenv("SMTP_PASSWORD"),
// 	)

// 	if err := d.DialAndSend(m); err != nil {
// 		fmt.Println("[DEBUG] SMTP Error:", err)
// 		return fmt.Errorf("error sending email: %v", err)
// 	}

// 	fmt.Println("[DEBUG] Email sent successfully!")
// 	return nil
// }

// func SendOTPEmail(email, otp string) error {
// 	data := EmailData{
// 		Subject:  "Your Verification Code",
// 		To:       []string{email},
// 		Template: "otp",
// 		Year:     time.Now().Year(),
// 		Data: struct {
// 			OTP string
// 		}{
// 			OTP: otp,
// 		},
// 	}

// 	return SendEmail(data)
// }

// func SendPasswordResetEmail(email, token string) error {
// 	resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("APP_URL"), token)
// 	data := EmailData{
// 		Subject:  "Password Reset Request",
// 		To:       []string{email},
// 		Template: "password_reset",
// 		Data: struct {
// 			ResetLink string
// 		}{
// 			ResetLink: resetLink,
// 		},
// 	}

// 	return SendEmail(data)
// }

// func SendPasswordResetOTPEmail(email, otp string) error {
// 	data := EmailData{
// 		Subject:  "Password Reset Verification Code",
// 		To:       []string{email},
// 		Template: "password_reset_otp",
// 		Data: struct {
// 			OTP string
// 		}{
// 			OTP: otp,
// 		},
// 	}

// 	return SendEmail(data)
// }