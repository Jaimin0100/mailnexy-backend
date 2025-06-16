package middleware

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
)

// SenderRateLimiter provides rate limiting specifically for sender testing endpoints
func SenderRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        config.AppConfig.RateLimitTestSender,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			// Get user from context (set by JWT middleware)
			user := c.Locals("user").(*models.User)
			
			// Rate limit key combines user ID, sender ID, and endpoint
			senderID := c.Params("id")
			return utils.GenerateRateLimitKey(user.ID, senderID, c.Path())
		},
		LimitReached: func(c *fiber.Ctx) error {
			// Log rate limit hit
			user := c.Locals("user").(*models.User)
			utils.LogEvent("rate_limit_hit", map[string]interface{}{
				"user_id":    user.ID,
				"endpoint":   c.Path(),
				"ip":         c.IP(),
				"user_agent": c.Get("User-Agent"),
			})

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":          "Too many test requests. Please wait before testing again.",
				"retry_after":    "1 minute",
				"documentation":  "https://yourdocs.com/rate-limits",
			})
		},
		Storage: createRateLimitStorage(),
	})
}

// createRateLimitStorage creates a persistent storage for rate limiting
func createRateLimitStorage() fiber.Storage {
	if config.AppConfig.Redis.Enabled {
		return NewRedisStorage(config.AppConfig.Redis)
	}
	return nil
}

// RedisStorage implements fiber.Storage for Redis
type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(config config.RedisConfig) *RedisStorage {
	return &RedisStorage{
		client: redis.NewClient(&redis.Options{
			Addr:     config.Address,
			Password: config.Password,
			DB:       config.DB,
		}),
	}
}

func (r *RedisStorage) Get(key string) ([]byte, error) {
	return r.client.Get(context.Background(), key).Bytes()
}

func (r *RedisStorage) Set(key string, val []byte, exp time.Duration) error {
	return r.client.Set(context.Background(), key, val, exp).Err()
}

func (r *RedisStorage) Delete(key string) error {
	return r.client.Del(context.Background(), key).Err()
}

func (r *RedisStorage) Reset() error {
	return r.client.FlushDB(context.Background()).Err()
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}
















// package middleware

// import (
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/gofiber/fiber/v2/middleware/limiter"
// 	"mailnexy/config"
// 	"mailnexy/models"
// 	"mailnexy/utils"
// )

// // SenderRateLimiter provides rate limiting specifically for sender testing endpoints
// func SenderRateLimiter() fiber.Handler {
// 	return limiter.New(limiter.Config{
// 		Max:        config.AppConfig.RateLimitTestSender, // Configurable from .env
// 		Expiration: 1 * time.Minute,
// 		KeyGenerator: func(c *fiber.Ctx) string {
// 			// Get user from context (set by JWT middleware)
// 			user := c.Locals("user").(*models.User)
			
// 			// Rate limit key combines user ID, sender ID, and endpoint
// 			senderID := c.Params("id")
// 			return utils.GenerateRateLimitKey(user.ID, senderID, c.Path())
// 		},
// 		LimitReached: func(c *fiber.Ctx) error {
// 			// Log rate limit hit
// 			user := c.Locals("user").(*models.User)
// 			utils.LogEvent("rate_limit_hit", map[string]interface{}{
// 				"user_id":   user.ID,
// 				"endpoint":  c.Path(),
// 				"ip":        c.IP(),
// 				"user_agent": c.Get("User-Agent"),
// 			})

// 			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
// 				"error": "Too many test requests. Please wait before testing again.",
// 				"retry_after": "1 minute",
// 				"documentation": "https://yourdocs.com/rate-limits",
// 			})
// 		},
// 		Storage: createRateLimitStorage(), // Custom storage for distributed systems
// 	})
// }

// // createRateLimitStorage creates a persistent storage for rate limiting
// func createRateLimitStorage() fiber.Storage {
// 	if config.AppConfig.Redis.Enabled {
// 		return NewRedisStorage(config.AppConfig.Redis)
// 	}
// 	return nil // Uses in-memory storage by default
// }

// // RedisStorage implements fiber.Storage for Redis
// type RedisStorage struct {
// 	client *redis.Client
// }

// func NewRedisStorage(config config.RedisConfig) *RedisStorage {
// 	return &RedisStorage{
// 		client: redis.NewClient(&redis.Options{
// 			Addr:     config.Address,
// 			Password: config.Password,
// 			DB:       config.DB,
// 		}),
// 	}
// }

// func (r *RedisStorage) Get(key string) ([]byte, error) {
// 	return r.client.Get(context.Background(), key).Bytes()
// }

// func (r *RedisStorage) Set(key string, val []byte, exp time.Duration) error {
// 	return r.client.Set(context.Background(), key, val, exp).Err()
// }

// func (r *RedisStorage) Delete(key string) error {
// 	return r.client.Del(context.Background(), key).Err()
// }

// func (r *RedisStorage) Reset() error {
// 	return r.client.FlushDB(context.Background()).Err()
// }

// func (r *RedisStorage) Close() error {
// 	return r.client.Close()
// }