package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"gorm.io/gorm"

	"github.com/gofiber/fiber/v2"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"omitempty,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	SessionID    string       `json:"session_id,omitempty"` // Add this line
	User         *models.User `json:"user"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type VerifyResetOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
	OTP   string `json:"otp" validate:"required,len=6"`
}

type ResetPasswordRequest struct {
	Email      string `json:"email" validate:"required,email"`
	ResetToken string `json:"reset_token" validate:"required"`
	Password   string `json:"password" validate:"required,min=8"`
}

var (
	googleOAuthConfig *oauth2.Config
)

func Register(c *fiber.Ctx) error {
	var req RegisterRequest
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

	// Check if user already exists
	var existingUser models.User
	if err := config.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Email already registered",
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	// Create user
	user := models.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         &req.Name,
		IsActive:     true,
		PlanName:     "free",
		EmailCredits: 5000,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	// Generate tokens
	accessToken, refreshToken, _, err := utils.GenerateJWTToken(&user, c.Get("User-Agent"), c.IP())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	otp, err := utils.GenerateOTP()
	if err != nil {
		// Log error but don't fail registration
		// logger.Error("Failed to generate OTP", zap.Error(err))
		return c.Status(fiber.StatusCreated).JSON(AuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			User:         &user,
		})
	}

	// Send verification email
	if err := utils.SendOTPEmail(user.Email, otp); err != nil {
		// Log error but don't fail registration
		// logger.Error("Failed to send verification email", zap.Error(err))
	}

	return c.Status(fiber.StatusCreated).JSON(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
	})
}

func Login(c *fiber.Ctx) error {
	var req LoginRequest
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid email or password",
		})
	}

	// Check if user is active
	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is not active",
		})
	}

	// Generate tokens
	accessToken, refreshToken, _, err := utils.GenerateJWTToken(&user, c.Get("User-Agent"), c.IP())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens",
		})
	}

	return c.JSON(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         &user,
	})
}

func Logout(c *fiber.Ctx) error {
	// Add token to blacklist (implement proper token blacklisting)
	// token := c.Get("Authorization")
	// utils.BlacklistToken(token)
	return c.SendStatus(fiber.StatusOK)
}

func ChangePassword(c *fiber.Ctx) error {
	var req ChangePasswordRequest
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

	// Get authenticated user
	user := c.Locals("user").(*models.User)

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid current password",
		})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	// Update password
	user.PasswordHash = string(hashedPassword)
	if err := config.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update password",
		})
	}

	// Invalidate existing tokens by updating user's token version
	user.TokenVersion = user.TokenVersion + 1
	if err := config.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update token version",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
	})
}

func RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	accessToken, refreshToken, err := utils.RefreshTokens(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
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
		// Don't reveal if user doesn't exist
		return c.JSON(fiber.Map{
			"message": "If an account exists, a reset code will be sent",
		})
	}

	// Check rate limiting
	canResend, remaining, err := utils.CanResendOTP(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check rate limit",
		})
	}
	if !canResend {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "Please wait before requesting another reset",
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
	// if err := utils.SendPasswordResetOTPEmail(user.Email, otp); err != nil {
	// 	fmt.Printf("Failed to send OTP email: %v\n", err)
	// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
	// 		"error": "Failed to send OTP email",
	// 	})
	// }

	if err := utils.SendPasswordResetOTPEmail(user.Email, otp); err != nil {
		// Log the detailed error
		fmt.Printf("Failed to send password reset OTP email to %s: %v\n", user.Email, err)

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to send OTP email",
			"details": err.Error(), // Include the actual error details
		})
	}

	return c.JSON(fiber.Map{
		"message": "If an account exists, a reset code will be sent",
	})
}

func VerifyResetPasswordOTP(c *fiber.Ctx) error {
	var req VerifyResetOTPRequest
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
		// Don't reveal if user doesn't exist (security best practice)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "OTP verification complete",
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
			"error": "Invalid or expired OTP",
		})
	}

	// Generate a password reset token (optional but recommended)
	resetToken, err := utils.GenerateSecureOTPToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate reset token",
		})
	}

	// Save reset token to user (with expiration)
	user.ResetToken = &resetToken
	resetTokenExpiry := time.Now().Add(30 * time.Minute) // Token valid for 30 mins
	user.ResetTokenExpiresAt = &resetTokenExpiry

	// Clear the OTP after successful verification
	user.OTP = ""
	user.OTPExpiresAt = time.Time{}

	if err := config.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user record",
		})
	}

	return c.JSON(fiber.Map{
		"message":     "OTP verified successfully",
		"reset_token": resetToken, // Return this token to client for password reset
	})
}

func ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
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
		// Don't reveal if user doesn't exist (security best practice)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Password reset successfully",
		})
	}

	// Verify reset token
	if user.ResetToken == nil || *user.ResetToken != req.ResetToken {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid reset token",
		})
	}

	// Check token expiration
	if user.ResetTokenExpiresAt == nil || time.Now().After(*user.ResetTokenExpiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Expired reset token",
		})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	// Update password and clear reset token
	user.PasswordHash = string(hashedPassword)
	user.ResetToken = nil
	user.ResetTokenExpiresAt = nil
	user.TokenVersion = user.TokenVersion + 1 // Invalidate existing tokens

	if err := config.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update password",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password reset successfully",
	})
}

func GetCurrentUser(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	return c.JSON(user)
}

func init() {
	googleOAuthConfig = &oauth2.Config{
		ClientID:     config.AppConfig.Google.ClientID,
		ClientSecret: config.AppConfig.Google.ClientSecret,
		RedirectURL:  config.AppConfig.Google.RedirectURI,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleOAuth(c *fiber.Ctx) error {
	// Generate OAuth state token with CSRF protection
	state, err := utils.GenerateSecureOTPToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate state token",
		})
	}

	// Store state in HTTP-only secure cookie with short expiry
	cookie := new(fiber.Cookie)
	cookie.Name = "oauth_state"
	cookie.Value = state
	cookie.Expires = time.Now().Add(10 * time.Minute) // Short-lived
	cookie.HTTPOnly = true
	cookie.Secure = true // Only send over HTTPS
	cookie.SameSite = "Lax"
	c.Cookie(cookie)

	url := googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

func GoogleOAuthCallback(c *fiber.Ctx) error {
	// Verify state token from cookie
	state := c.Query("state")
	cookieState := c.Cookies("oauth_state")

	if state == "" || cookieState == "" || state != cookieState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid state parameter",
		})
	}

	// Clear the state cookie
	c.ClearCookie("oauth_state")

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code not provided",
		})
	}

	// Exchange code for token
	token, err := googleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to exchange token: " + err.Error(),
		})
	}

	// Get user info
	client := googleOAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user info: " + err.Error(),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Google API error: " + string(body),
		})
	}

	var googleUser struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Picture  string `json:"picture"`
		Verified bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to parse user info: " + err.Error(),
		})
	}

	// Validate required fields
	if googleUser.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Google account email is required",
		})
	}

	// Find or create user
	var user models.User
	err = config.DB.Where("email = ?", googleUser.Email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User doesn't exist, create new
			user = models.User{
				Email:          googleUser.Email,
				Name:           &googleUser.Name,
				GoogleID:       &googleUser.ID,
				GoogleImageURL: &googleUser.Picture,
				EmailVerified:  googleUser.Verified,
				IsActive:       true,
				PlanName:       "free",
				EmailCredits:   5000,
				TokenVersion:   1,
			}

			if err := config.DB.Create(&user).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to create user: " + err.Error(),
				})
			}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database error: " + err.Error(),
			})
		}
	} else {
		// Update Google info if needed
		updateNeeded := false
		if user.GoogleID == nil || *user.GoogleID != googleUser.ID {
			user.GoogleID = &googleUser.ID
			updateNeeded = true
		}
		if user.GoogleImageURL == nil || *user.GoogleImageURL != googleUser.Picture {
			user.GoogleImageURL = &googleUser.Picture
			updateNeeded = true
		}
		if !user.EmailVerified && googleUser.Verified {
			user.EmailVerified = true
			updateNeeded = true
		}

		// Invalidate all previous sessions if this is a new Google login
		if updateNeeded {
			user.TokenVersion = user.TokenVersion + 1
			if err := config.DB.Save(&user).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to update user: " + err.Error(),
				})
			}

			// Revoke all existing refresh tokens for this user
			if err := config.DB.Model(&models.RefreshToken{}).
				Where("user_id = ? AND is_revoked = ?", user.ID, false).
				Update("is_revoked", true).Error; err != nil {
				// Log but don't fail
				fmt.Printf("Failed to revoke old refresh tokens: %v\n", err)
			}
		}
	}

	// Generate tokens
	accessToken, refreshToken, sessionID, err := utils.GenerateJWTToken(&user, c.Get("User-Agent"), c.IP())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate tokens: " + err.Error(),
		})
	}

	// Set secure HTTP-only cookies
	accessCookie := new(fiber.Cookie)
	accessCookie.Name = "access_token"
	accessCookie.Value = accessToken
	accessCookie.Expires = time.Now().Add(15 * time.Minute) // Matches access token expiry
	accessCookie.HTTPOnly = true
	accessCookie.Secure = true
	accessCookie.SameSite = "Lax"
	c.Cookie(accessCookie)

	refreshCookie := new(fiber.Cookie)
	refreshCookie.Name = "refresh_token"
	refreshCookie.Value = refreshToken
	refreshCookie.Expires = time.Now().Add(7 * 24 * time.Hour) // Matches refresh token expiry
	refreshCookie.HTTPOnly = true
	refreshCookie.Secure = true
	refreshCookie.SameSite = "Lax"
	c.Cookie(refreshCookie)

	// Return response (you can customize this based on your needs)
	return c.JSON(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
		User:         &user,
	})
}







// package controller

// import (
// 	"context"
// 	"encoding/json"
// 	"io"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"mailnexy/config"
// 	"mailnexy/models"
// 	"mailnexy/utils"
// 	"golang.org/x/crypto/bcrypt"
// 	"golang.org/x/oauth2"
// 	"golang.org/x/oauth2/google"
// )

// type RegisterRequest struct {
// 	Email    string `json:"email" validate:"required,email"`
// 	Password string `json:"password" validate:"required,min=8"`
// 	Name     string `json:"name" validate:"omitempty,max=100"`
// }

// type LoginRequest struct {
// 	Email    string `json:"email" validate:"required,email"`
// 	Password string `json:"password" validate:"required"`
// }

// type AuthResponse struct {
// 	AccessToken  string       `json:"access_token"`
// 	RefreshToken string       `json:"refresh_token"`
// 	User         *models.User `json:"user"`
// }

// type RefreshTokenRequest struct {
// 	RefreshToken string `json:"refresh_token" validate:"required"`
// }

// type ForgotPasswordRequest struct {
// 	Email string `json:"email" validate:"required,email"`
// }

// type ResetPasswordRequest struct {
// 	Token    string `json:"token" validate:"required"`
// 	Password string `json:"password" validate:"required,min=8"`
// }

// var (
// 	googleOAuthConfig *oauth2.Config
// )

// func Register(c *fiber.Ctx) error {
// 	var req RegisterRequest
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

// 	// Check if user already exists
// 	var existingUser models.User
// 	if err := config.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
// 		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
// 			"error": "Email already registered",
// 		})
// 	}

// 	// Hash password
// 	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to hash password",
// 		})
// 	}

// 	// Create user
// 	user := models.User{
// 		Email:        req.Email,
// 		PasswordHash: string(hashedPassword),
// 		Name:         &req.Name,
// 		IsActive:     true,
// 		PlanName:     "free",
// 		EmailCredits: 5000,
// 	}

// 	if err := config.DB.Create(&user).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create user",
// 		})
// 	}

// 	// Generate tokens
// 	accessToken, refreshToken, err := utils.GenerateJWTToken(&user)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate tokens",
// 		})
// 	}

// 	// Send verification email (if needed)
// 	// ...

// 	return c.Status(fiber.StatusCreated).JSON(AuthResponse{
// 		AccessToken:  accessToken,
// 		RefreshToken: refreshToken,
// 		User:         &user,
// 	})
// }

// func Login(c *fiber.Ctx) error {
// 	var req LoginRequest
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
// 		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "Invalid email or password",
// 		})
// 	}

// 	// Check password
// 	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
// 		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": "Invalid email or password",
// 		})
// 	}

// 	// Check if user is active
// 	if !user.IsActive {
// 		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
// 			"error": "Account is not active",
// 		})
// 	}

// 	// Generate tokens
// 	accessToken, refreshToken, err := utils.GenerateJWTToken(&user)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate tokens",
// 		})
// 	}

// 	return c.JSON(AuthResponse{
// 		AccessToken:  accessToken,
// 		RefreshToken: refreshToken,
// 		User:         &user,
// 	})
// }

// func Logout(c *fiber.Ctx) error {
// 	// In a stateless JWT system, logout is handled client-side by deleting the token
// 	// But we can implement token blacklisting if needed
// 	return c.SendStatus(fiber.StatusOK)
// }

// func RefreshToken(c *fiber.Ctx) error {
// 	var req RefreshTokenRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request body",
// 		})
// 	}

// 	accessToken, refreshToken, err := utils.RefreshTokens(req.RefreshToken)
// 	if err != nil {
// 		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
// 			"error": err.Error(),
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"access_token":  accessToken,
// 		"refresh_token": refreshToken,
// 	})
// }

// func ForgotPassword(c *fiber.Ctx) error {
// 	var req ForgotPasswordRequest
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
// 		// Don't reveal if user doesn't exist for security
// 		return c.SendStatus(fiber.StatusOK)
// 	}

// 	// Generate OTP
// 	otp, err := utils.GenerateOTP()
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate OTP",
// 		})
// 	}

// 	// Save OTP to user
// 	if err := utils.SaveOTP(user.ID, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to save OTP",
// 		})
// 	}

// 	// Send OTP via email
// 	if err := utils.SendPasswordResetOTPEmail(user.Email, otp); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to send OTP email",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"message": "OTP sent to your email",
// 		"user_id": user.ID, // Return user ID for verification step
// 	})
// }

// func VerifyResetPasswordOTP(c *fiber.Ctx) error {
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
// 			"error": "Invalid or expired OTP",
// 		})
// 	}

// 	// Generate a password reset token (can be the OTP itself or a new token)
// 	resetToken, err := utils.GenerateSecureToken()
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate reset token",
// 		})
// 	}

// 	// Save the reset token (you might want to store this in a separate table)
// 	user.OTP = resetToken
// 	user.OTPExpiresAt = time.Now().Add(10 * time.Minute) // Short expiry for reset token
// 	if err := config.DB.Save(&user).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to save reset token",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"message":     "OTP verified successfully",
// 		"reset_token": resetToken,
// 	})
// }

// func ResetPassword(c *fiber.Ctx) error {
// 	var req ResetPasswordRequest
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

// 	// Find user by reset token
// 	var user models.User
// 	if err := config.DB.Where("otp = ? AND otp_expires_at > ?", req.Token, time.Now()).First(&user).Error; err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid or expired token",
// 		})
// 	}

// 	// Hash new password
// 	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to hash password",
// 		})
// 	}

// 	// Update password and clear token
// 	user.PasswordHash = string(hashedPassword)
// 	user.OTP = ""
// 	user.OTPExpiresAt = time.Time{}
// 	if err := config.DB.Save(&user).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update password",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"message": "Password reset successfully",
// 	})
// }
// func GetCurrentUser(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	return c.JSON(user)
// }

// func init() {
// 	googleOAuthConfig = &oauth2.Config{
// 		ClientID:     config.AppConfig.Google.ClientID,
// 		ClientSecret: config.AppConfig.Google.ClientSecret,
// 		RedirectURL:  config.AppConfig.Google.RedirectURI,
// 		Scopes: []string{
// 			"https://www.googleapis.com/auth/userinfo.email",
// 			"https://www.googleapis.com/auth/userinfo.profile",
// 		},
// 		Endpoint: google.Endpoint,
// 	}
// }

// func GoogleOAuth(c *fiber.Ctx) error {
// 	// Generate OAuth state token
// 	state, err := utils.GenerateSecureToken()
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate state token",
// 		})
// 	}

// 	// Save state to session or database if needed
// 	// ...

// 	url := googleOAuthConfig.AuthCodeURL(state)
// 	return c.Redirect(url, fiber.StatusTemporaryRedirect)
// }

// func GoogleOAuthCallback(c *fiber.Ctx) error {
// 	// Verify state token if you saved it
// 	// state := c.Query("state")
// 	// ...

// 	code := c.Query("code")
// 	if code == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Authorization code not provided",
// 		})
// 	}

// 	// Exchange code for token
// 	token, err := googleOAuthConfig.Exchange(context.Background(), code)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to exchange token",
// 		})
// 	}

// 	// Get user info
// 	client := googleOAuthConfig.Client(context.Background(), token)
// 	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to get user info",
// 		})
// 	}
// 	defer resp.Body.Close()

// 	data, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to read user info",
// 		})
// 	}

// 	var googleUser struct {
// 		ID      string `json:"id"`
// 		Email   string `json:"email"`
// 		Name    string `json:"name"`
// 		Picture string `json:"picture"`
// 	}

// 	if err := json.Unmarshal(data, &googleUser); err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to parse user info",
// 		})
// 	}

// 	// Find or create user
// 	var user models.User
// 	if err := config.DB.Where("email = ?", googleUser.Email).First(&user).Error; err != nil {
// 		// User doesn't exist, create new
// 		user = models.User{
// 			Email:          googleUser.Email,
// 			Name:           &googleUser.Name,
// 			GoogleID:       &googleUser.ID,
// 			GoogleImageURL: &googleUser.Picture,
// 			EmailVerified:  true,
// 			IsActive:       true,
// 			PlanName:       "free",
// 			EmailCredits:   5000,
// 		}

// 		if err := config.DB.Create(&user).Error; err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create user",
// 			})
// 		}
// 	} else {
// 		// Update Google info if needed
// 		if user.GoogleID == nil || *user.GoogleID != googleUser.ID {
// 			user.GoogleID = &googleUser.ID
// 			user.GoogleImageURL = &googleUser.Picture
// 			if err := config.DB.Save(&user).Error; err != nil {
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": "Failed to update user",
// 				})
// 			}
// 		}
// 	}

// 	// Generate tokens
// 	accessToken, refreshToken, err := utils.GenerateJWTToken(&user)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to generate tokens",
// 		})
// 	}

// 	// Redirect to frontend with tokens
// 	// In production, you might want to use a proper redirect with tokens in secure cookies
// 	return c.JSON(AuthResponse{
// 		AccessToken:  accessToken,
// 		RefreshToken: refreshToken,
// 		User:         &user,
// 	})
// }