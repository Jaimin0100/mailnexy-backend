package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GenerateRateLimitKey creates a unique key for rate limiting
func GenerateRateLimitKey(userID uint, senderID, path string) string {
	return fmt.Sprintf("rl:%d:%s:%s", userID, senderID, path)
}

// LogEvent logs an event with structured data
func LogEvent(eventType string, data map[string]interface{}) {
	// Implement your logging logic here
	// Could use logrus, zap, or other logging libraries
	fmt.Printf("[%s] %+v\n", eventType, data)
}

// ValidateMXRecords checks if a domain has valid MX records
func ValidateMXRecords(email string) (bool, error) {
	parts := strings.Split(email, "@")
	if len(parts) < 2 {
		return false, fmt.Errorf("invalid email format")
	}
	
	domain := parts[1]
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return false, err
	}
	
	return len(mxRecords) > 0, nil
}

// Pointer returns a pointer to the given value
func Pointer[T any](v T) *T {
	return &v
}

// ParseDuration parses a duration string (e.g., "1h", "30m")
func ParseDuration(durationStr string) (time.Duration, error) {
	return time.ParseDuration(durationStr)
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d.Hours() >= 24 {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	} else if d.Hours() >= 1 {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else if d.Minutes() >= 1 {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.1f seconds", d.Seconds())
}

// ErrorResponse creates a standardized error response
func ErrorResponse(c *fiber.Ctx, status int, message string, err error) error {
	response := fiber.Map{
		"success": false,
		"error":   message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	return c.Status(status).JSON(response)
}

// SuccessResponse creates a standardized success response
func SuccessResponse(data interface{}) fiber.Map {
	return fiber.Map{
		"success": true,
		"data":    data,
	}
}

// ParseUint safely parses a string to uint
func ParseUint(s string) uint {
	i, _ := strconv.ParseUint(s, 10, 32)
	return uint(i)
}

// PaginatedResponse structure for paginated results
type PaginatedResponse struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}



// package utils

// import (
// 	"fmt"
// 	"strconv"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// )

// // Pointer returns a pointer to the given value
// func Pointer[T any](v T) *T {
// 	return &v
// }

// // ParseDuration parses a duration string (e.g., "1h", "30m")
// func ParseDuration(durationStr string) (time.Duration, error) {
// 	return time.ParseDuration(durationStr)
// }

// // FormatDuration formats a duration in a human-readable way
// func FormatDuration(d time.Duration) string {
// 	if d.Hours() >= 24 {
// 		days := int(d.Hours() / 24)
// 		return fmt.Sprintf("%d days", days)
// 	} else if d.Hours() >= 1 {
// 		return fmt.Sprintf("%.1f hours", d.Hours())
// 	} else if d.Minutes() >= 1 {
// 		return fmt.Sprintf("%.1f minutes", d.Minutes())
// 	}
// 	return fmt.Sprintf("%.1f seconds", d.Seconds())
// }

// // ErrorResponse creates a standardized error response
// func ErrorResponse(c *fiber.Ctx, status int, message string, err error) error {
// 	response := fiber.Map{
// 		"success": false,
// 		"error":   message,
// 	}
// 	if err != nil {
// 		response["details"] = err.Error()
// 	}
// 	return c.Status(status).JSON(response)
// }

// // SuccessResponse creates a standardized success response
// func SuccessResponse(data interface{}) fiber.Map {
// 	return fiber.Map{
// 		"success": true,
// 		"data":    data,
// 	}
// }

// // ParseUint safely parses a string to uint
// func ParseUint(s string) uint {
// 	i, _ := strconv.ParseUint(s, 10, 32)
// 	return uint(i)
// }

// // PaginatedResponse structure for paginated results
// type PaginatedResponse struct {
// 	Data  interface{} `json:"data"`
// 	Total int64       `json:"total"`
// 	Page  int         `json:"page"`
// 	Limit int         `json:"limit"`
// }