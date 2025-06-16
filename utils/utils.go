// package utils

// import (
// 	"fmt"
// 	"strconv"
// 	"time"

// 	"github.com/go-playground/validator/v10"
// 	"github.com/gofiber/fiber/v2"
// )

// var validate = validator.New()

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

// // ValidateStruct validates a struct using go-playground/validator
// func ValidateStruct(s interface{}) error {
// 	return validate.Struct(s)
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

package utils

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

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