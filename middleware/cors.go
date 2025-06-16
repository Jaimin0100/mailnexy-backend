package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CORSConfig defines the config for CORS middleware
type CORSConfig struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from
	AllowedOrigins []string
	
	// AllowCredentials indicates whether the request can include user credentials
	AllowCredentials bool
	
	// AllowedMethods is a list of methods the client is allowed to use
	AllowedMethods []string
	
	// AllowedHeaders is a list of non-simple headers the client is allowed to use
	AllowedHeaders []string
	
	// ExposedHeaders indicates which headers are safe to expose to the API of a CORS API specification
	ExposedHeaders []string
	
	// MaxAge indicates how long (in seconds) the results of a preflight request can be cached
	MaxAge int
}

// DefaultCORSConfig returns a default CORS config
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposedHeaders:   []string{"Content-Length"},
		MaxAge:           3600,
	}
}

// CORS creates a new CORS middleware handler
func CORS(config ...CORSConfig) fiber.Handler {
	// Set default config
	cfg := DefaultCORSConfig()

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
	}

	// Convert allowed origins to map for faster lookup
	allowedOrigins := make(map[string]struct{})
	for _, origin := range cfg.AllowedOrigins {
		allowedOrigins[origin] = struct{}{}
	}

	// Convert allowed methods to string
	allowedMethods := strings.Join(cfg.AllowedMethods, ",")

	// Convert allowed headers to string
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ",")

	// Convert exposed headers to string
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ",")

	return func(c *fiber.Ctx) error {
		// Get origin header
		origin := c.Get("Origin")

		// Check if the origin is allowed
		if len(cfg.AllowedOrigins) > 0 {
			if _, ok := allowedOrigins[origin]; ok {
				c.Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			c.Set("Access-Control-Allow-Origin", "*")
		}

		// Set allow credentials header
		if cfg.AllowCredentials {
			c.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle OPTIONS method for preflight requests
		if c.Method() == "OPTIONS" {
			// Set CORS headers
			c.Set("Access-Control-Allow-Methods", allowedMethods)
			c.Set("Access-Control-Allow-Headers", allowedHeaders)
			c.Set("Access-Control-Expose-Headers", exposedHeaders)
			c.Set("Access-Control-Max-Age", string(rune(cfg.MaxAge)))
			
			// Return 204 No Content
			return c.SendStatus(fiber.StatusNoContent)
		}

		// Continue stack
		return c.Next()
	}
}