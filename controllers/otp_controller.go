package controller

import (
	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
)

type SendOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
	OTP   string `json:"otp" validate:"required,len=6"`
}

type ResendOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func SendOTP(c *fiber.Ctx) error {
	var req SendOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Find user
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Check if OTP was recently sent
	canResend, remaining, err := utils.CanResendOTP(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check OTP status",
		})
	}
	if !canResend {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "OTP was recently sent",
			"retry_after": remaining.Seconds(),
		})
	}

	// Generate OTP
	otp, err := utils.GenerateOTP()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate OTP",
		})
	}

	// Save OTP
	if err := utils.SaveOTP(user.ID, otp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save OTP",
		})
	}

	// Send OTP via email
	if err := utils.SendOTPEmail(user.Email, otp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send OTP",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func VerifyOTP(c *fiber.Ctx) error {
	var req VerifyOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Find user
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Verify OTP
	valid, err := utils.VerifyOTP(user.ID, req.OTP)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify OTP",
		})
	}
	if !valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid OTP",
		})
	}

	// Mark email as verified
	user.EmailVerified = true
	if err := config.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func ResendOTP(c *fiber.Ctx) error {
	var req ResendOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Find user
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Check if OTP was recently sent
	canResend, remaining, err := utils.CanResendOTP(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check OTP status",
		})
	}
	if !canResend {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "OTP was recently sent",
			"retry_after": remaining.Seconds(),
		})
	}

	// Generate new OTP
	otp, err := utils.GenerateOTP()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate OTP",
		})
	}

	// Save OTP
	if err := utils.SaveOTP(user.ID, otp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save OTP",
		})
	}

	// Send OTP via email
	if err := utils.SendOTPEmail(user.Email, otp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send OTP",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}













// package controller

// import (
// 	"github.com/gofiber/fiber/v2"
// 	"mailnexy/config"
// 	"mailnexy/models"
// 	"mailnexy/utils"
// )

// type SendOTPRequest struct {
// 	Email string `json:"email" validate:"required,email"`
// }

// type VerifyOTPRequest struct {
// 	Email string `json:"email" validate:"required,email"`
// 	OTP   string `json:"otp" validate:"required,len=6"`
// }

// type ResendOTPRequest struct {
// 	Email string `json:"email" validate:"required,email"`
// }

// func SendOTP(c *fiber.Ctx) error {
// 	var req SendOTPRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	// Validate request
// 	if err := utils.ValidateStruct(req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	// Find user
// 	var user models.User
// 	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "User not found",
// 		})
// 	}

// 	// Check if OTP was recently sent
// 	canResend, remaining, err := utils.CanResendOTP(user.ID)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to check OTP status",
// 		})
// 	}
// 	if !canResend {
// 		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
// 			"error":       "OTP was recently sent",
// 			"retry_after": remaining.Seconds(),
// 		})
// 	}

// 	// Generate OTP
// 	otp, err := utils.GenerateOTP()
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate OTP",
// 		})
// 	}

// 	// Save OTP
// 	if err := utils.SaveOTP(user.ID, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to save OTP",
// 		})
// 	}

// 	// Send OTP via email
// 	if err := utils.SendOTPEmail(user.Email, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to send OTP",
// 		})
// 	}

// 	return c.SendStatus(fiber.StatusOK)
// }

// func VerifyOTP(c *fiber.Ctx) error {
// 	var req VerifyOTPRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	// Validate request
// 	if err := utils.ValidateStruct(req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	// Find user
// 	var user models.User
// 	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "User not found",
// 		})
// 	}

// 	// Verify OTP
// 	valid, err := utils.VerifyOTP(user.ID, req.OTP)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to verify OTP",
// 		})
// 	}
// 	if !valid {
// 		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "Invalid OTP",
// 		})
// 	}

// 	// Mark email as verified
// 	user.EmailVerified = true
// 	if err := config.DB.Save(&user).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update user",
// 		})
// 	}

// 	return c.SendStatus(fiber.StatusOK)
// }

// func ResendOTP(c *fiber.Ctx) error {
// 	var req ResendOTPRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	// Validate request
// 	if err := utils.ValidateStruct(req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	// Find user
// 	var user models.User
// 	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "User not found",
// 		})
// 	}

// 	// Check if OTP was recently sent
// 	canResend, remaining, err := utils.CanResendOTP(user.ID)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to check OTP status",
// 		})
// 	}
// 	if !canResend {
// 		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
// 			"error":       "OTP was recently sent",
// 			"retry_after": remaining.Seconds(),
// 		})
// 	}

// 	// Generate new OTP
// 	otp, err := utils.GenerateOTP()
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate OTP",
// 		})
// 	}

// 	// Save OTP
// 	if err := utils.SaveOTP(user.ID, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to save OTP",
// 		})
// 	}

// 	// Send OTP via email
// 	if err := utils.SendOTPEmail(user.Email, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to send OTP",
// 		})
// 	}

// 	return c.SendStatus(fiber.StatusOK)
// }