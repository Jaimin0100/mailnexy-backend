package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
)

func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to get token from Authorization header first
		var token string
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			// Check if it's a Bearer token
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid authorization format",
				})
			}
			token = tokenParts[1]
		} else {
			// Fall back to cookie if header not present
			token = c.Cookies("access_token")
			if token == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Authorization required",
				})
			}
		}

		// Parse and validate JWT
		claims, err := utils.ParseJWTToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Find user
		var user models.User
		if err := config.DB.First(&user, claims.UserID).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		// Check if user is active
		if !user.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Account is not active",
			})
		}

		// Verify token version
		if claims.TokenVersion != user.TokenVersion {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token version",
			})
		}

		// Add user and session to context
		c.Locals("user", &user)
		c.Locals("userID", user.ID)
		c.Locals("sessionID", claims.SessionID)

		return c.Next()
	}
}
