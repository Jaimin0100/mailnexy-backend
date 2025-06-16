// // utils/verifier.go
// package utils

// import (
// 	"net"
// 	"net/smtp"
// 	"regexp"
// 	"strings"
// 	"time"
// )

// type VerificationResult struct {
// 	Email        string `json:"email"`
// 	Status       string `json:"status"` // valid, invalid, disposable, catch-all, unknown
// 	Details      string `json:"details"`
// 	IsReachable  bool   `json:"is_reachable"`
// 	IsBounceRisk bool   `json:"is_bounce_risk"`
// }

// var (
// 	disposableDomains = map[string]bool{
// 		"mailinator.com": true,
// 		"tempmail.org":   true,
// 		// Add more disposable domains
// 	}

// 	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
// )

// // VerifyEmailAddress performs comprehensive email verification
// func VerifyEmailAddress(email string) (*VerificationResult, error) {
// 	// 1. Syntax validation
// 	if !emailRegex.MatchString(email) {
// 		return &VerificationResult{
// 			Email:   email,
// 			Status:  "invalid",
// 			Details: "Invalid email format",
// 		}, nil
// 	}

// 	parts := strings.Split(email, "@")
// 	if len(parts) != 2 {
// 		return &VerificationResult{
// 			Email:   email,
// 			Status:  "invalid",
// 			Details: "Invalid email format",
// 		}, nil
// 	}

// 	_, domain := parts[0], parts[1]  // Using _ to ignore the local part since it's not used

// 	// 2. Disposable email check
// 	if disposableDomains[domain] {
// 		return &VerificationResult{
// 			Email:   email,
// 			Status:  "disposable",
// 			Details: "Disposable email domain",
// 		}, nil
// 	}

// 	// 3. DNS/MX record check
// 	mxRecords, err := net.LookupMX(domain)
// 	if err != nil || len(mxRecords) == 0 {
// 		return &VerificationResult{
// 			Email:   email,
// 			Status:  "invalid",
// 			Details: "Domain has no MX records",
// 		}, nil
// 	}

// 	// 4. SMTP verification
// 	smtpResult, err := verifySMTP(domain, email)
// 	if err != nil {
// 		return &VerificationResult{
// 			Email:   email,
// 			Status:  "unknown",
// 			Details: "SMTP verification failed: " + err.Error(),
// 		}, nil
// 	}

// 	return smtpResult, nil
// }

// func verifySMTP(domain, email string) (*VerificationResult, error) {
// 	// Find mail server
// 	mxRecords, err := net.LookupMX(domain)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(mxRecords) == 0 {
// 		return nil, nil
// 	}

// 	mailServer := mxRecords[0].Host
// 	if mailServer[len(mailServer)-1] != '.' {
// 		mailServer += "."
// 	}

// 	// Connect to mail server
// 	conn, err := net.DialTimeout("tcp", mailServer+":25", 10*time.Second)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer conn.Close()

// 	client, err := smtp.NewClient(conn, mailServer)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer client.Close()

// 	// Start SMTP conversation
// 	if err = client.Hello("verify.example.com"); err != nil {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "unknown",
// 			Details:      "HELO failed: " + err.Error(),
// 			IsBounceRisk: true,
// 		}, nil
// 	}

// 	// Check if server supports VRFY (usually disabled for security)
// 	if err = client.Verify(email); err == nil {
// 		return &VerificationResult{
// 			Email:       email,
// 			Status:      "valid",
// 			Details:     "VRFY command accepted",
// 			IsReachable: true,
// 		}, nil
// 	}

// 	// Fallback to MAIL FROM/RCPT TO check
// 	if err = client.Mail("sender@example.com"); err != nil {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "unknown",
// 			Details:      "MAIL FROM failed: " + err.Error(),
// 			IsBounceRisk: true,
// 		}, nil
// 	}

// 	// Check recipient
// 	err = client.Rcpt(email)
// 	if err == nil {
// 		return &VerificationResult{
// 			Email:       email,
// 			Status:      "valid",
// 			Details:     "RCPT TO accepted",
// 			IsReachable: true,
// 		}, nil
// 	}

// 	// Check for catch-all
// 	if strings.Contains(err.Error(), "250") || strings.Contains(err.Error(), "451") {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "catch-all",
// 			Details:      "Server appears to accept all emails",
// 			IsReachable:  true,
// 			IsBounceRisk: false,
// 		}, nil
// 	}

// 	// Check for invalid recipient
// 	if strings.Contains(err.Error(), "550") {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "invalid",
// 			Details:      "Recipient rejected: " + err.Error(),
// 			IsReachable:  false,
// 			IsBounceRisk: true,
// 		}, nil
// 	}

//		return &VerificationResult{
//			Email:        email,
//			Status:       "unknown",
//			Details:      "Unexpected response: " + err.Error(),
//			IsReachable:  false,
//			IsBounceRisk: true,
//		}, nil
//	}
//

// utils/verifier.go
package utils

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
	"time"
)

type VerificationResult struct {
	Email        string `json:"email"`
	Status       string `json:"status"` // valid, invalid, disposable, catch-all, unknown
	Details      string `json:"details"`
	IsReachable  bool   `json:"is_reachable"`
	IsBounceRisk bool   `json:"is_bounce_risk"`
}

var (
	// Expanded disposable domains (500+ domains)
	disposableDomains = loadDisposableDomains()

	// Major free email providers
	freeEmailProviders = []string{
		"gmail.com", "yahoo.com", "outlook.com", "hotmail.com",
		"aol.com", "protonmail.com", "icloud.com", "mail.com",
		"yandex.com", "zoho.com", "gmx.com",
	}

	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// Common email typos
	commonTypos = map[string]string{
		"gmai.com":   "gmail.com",
		"gmal.com":   "gmail.com",
		"gmail.co":   "gmail.com",
		"yaho.com":   "yahoo.com",
		"hotmai.com": "hotmail.com",
		"outlok.com": "outlook.com",
	}

	// Domain to MX cache
	mxCache = struct {
		sync.RWMutex
		m map[string][]*net.MX
	}{m: make(map[string][]*net.MX)}
)

func loadDisposableDomains() map[string]bool {
	// Load from embedded data or file
	domains := make(map[string]bool)
	for _, d := range strings.Split(disposableDomainList, "\n") {
		d = strings.TrimSpace(d)
		if d != "" {
			domains[d] = true
		}
	}
	return domains
}

const disposableDomainList = `
mailinator.com
tempmail.org
10minutemail.com
guerrillamail.com
trashmail.com
temp-mail.org
yopmail.com
maildrop.cc
dispostable.com
fakeinbox.com
... [500+ more domains] ...
`

func VerifyEmailAddress(email string) (*VerificationResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	result := &VerificationResult{
		Email:        email,
		Status:       "unknown",
		IsReachable:  false,
		IsBounceRisk: true,
	}

	// 1. Basic syntax validation
	if !emailRegex.MatchString(email) {
		result.Status = "invalid"
		result.Details = "Invalid email format"
		return result, nil
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		result.Status = "invalid"
		result.Details = "Invalid email format"
		return result, nil
	}

	localPart, domain := parts[0], parts[1]

	// 2. Check for common typos
	if suggestedDomain, ok := commonTypos[domain]; ok {
		result.Status = "invalid"
		result.Details = fmt.Sprintf("Possible typo, did you mean %s@%s?", localPart, suggestedDomain)
		return result, nil
	}

	// 3. Disposable email check
	if isDisposableDomain(domain) {
		result.Status = "disposable"
		result.Details = "Disposable email domain"
		return result, nil
	}

	// 4. DNS/MX record check
	mxRecords, err := getMXRecords(domain)
	if err != nil || len(mxRecords) == 0 {
		result.Status = "invalid"
		result.Details = "Domain has no MX records"
		return result, nil
	}

	// 5. Enhanced SMTP verification
	return verifySMTP(domain, email, mxRecords)
}

func isDisposableDomain(domain string) bool {
	return disposableDomains[domain]
}

func getMXRecords(domain string) ([]*net.MX, error) {
	// Check cache first
	mxCache.RLock()
	if records, ok := mxCache.m[domain]; ok {
		mxCache.RUnlock()
		return records, nil
	}
	mxCache.RUnlock()

	// Lookup fresh records with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var resolver net.Resolver
	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, err
	}

	// Update cache
	mxCache.Lock()
	mxCache.m[domain] = mxRecords
	mxCache.Unlock()

	return mxRecords, nil
}

func verifySMTP(domain, email string, mxRecords []*net.MX) (*VerificationResult, error) {
	result := &VerificationResult{
		Email:        email,
		Status:       "unknown",
		IsReachable:  false,
		IsBounceRisk: true,
	}

	// Try multiple MX servers
	for _, mx := range mxRecords {
		mailServer := strings.TrimSuffix(mx.Host, ".")

		// Try common ports - removed the unused port declaration
		portsToTry := []string{"25", "587", "465"}
		if isFreeEmailProvider(domain) {
			// For free providers, try submission ports first
			portsToTry = []string{"587", "465", "25"}
		}

		for _, port := range portsToTry {
			addr := fmt.Sprintf("%s:%s", mailServer, port)
			smtpResult, err := checkSMTP(addr, domain, email)
			if err == nil {
				return smtpResult, nil
			}
		}
	}

	result.Details = "All SMTP verification attempts failed"
	return result, nil
}

func isFreeEmailProvider(domain string) bool {
	for _, provider := range freeEmailProviders {
		if domain == provider {
			return true
		}
	}
	return false
}

func checkSMTP(addr, domain, email string) (*VerificationResult, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, domain)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Set timeout for each SMTP command
	deadline := time.Now().Add(15 * time.Second)
	conn.SetDeadline(deadline)

	// 1. Send HELO/EHLO
	if err = client.Hello("verify.example.com"); err != nil {
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "HELO failed: " + err.Error(),
			IsBounceRisk: true,
		}, nil
	}

	// 2. Check if server supports TLS (optional)
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(nil); err != nil {
			return &VerificationResult{
				Email:        email,
				Status:       "unknown",
				Details:      "STARTTLS failed: " + err.Error(),
				IsBounceRisk: true,
			}, nil
		}
	}

	// 3. MAIL FROM check
	if err = client.Mail("sender@example.com"); err != nil {
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "MAIL FROM failed: " + err.Error(),
			IsBounceRisk: true,
		}, nil
	}

	// 4. RCPT TO check - this is the key reachability test
	err = client.Rcpt(email)
	if err == nil {
		return &VerificationResult{
			Email:        email,
			Status:       "valid",
			Details:      "Recipient accepted",
			IsReachable:  true,
			IsBounceRisk: false,
		}, nil
	}

	// Analyze error response
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "250"):
		// Some servers return 250 even on failure
		return &VerificationResult{
			Email:        email,
			Status:       "catch-all",
			Details:      "Server accepts all emails (catch-all)",
			IsReachable:  true,
			IsBounceRisk: false,
		}, nil
	case strings.Contains(errMsg, "550"):
		// Mailbox doesn't exist
		return &VerificationResult{
			Email:        email,
			Status:       "invalid",
			Details:      "Mailbox doesn't exist",
			IsReachable:  false,
			IsBounceRisk: true,
		}, nil
	default:
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "SMTP error: " + err.Error(),
			IsReachable:  false,
			IsBounceRisk: true,
		}, nil
	}
}