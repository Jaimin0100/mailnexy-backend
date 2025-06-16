package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"mailnexy/config"
	"mailnexy/models"
)

type Claims struct {
	UserID       uint   `json:"user_id"`
	TokenVersion uint   `json:"token_version"`
	SessionID    string `json:"session_id"` // Unique session identifier
	jwt.RegisteredClaims
}

// GenerateSecureToken creates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func GenerateJWTToken(user *models.User, userAgent string, ipAddress string) (string, string, string, error) {
	// Generate a unique session ID for refresh token tracking
	sessionID, err := GenerateSecureToken(32)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Access token (15 minutes expiry)
	accessClaims := &Claims{
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		SessionID:    sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(config.AppConfig.EncryptionKey))
	if err != nil {
		return "", "", "", err
	}

	// Refresh token (7 days expiry)
	refreshClaims := &Claims{
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		SessionID:    sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(config.AppConfig.EncryptionKey))
	if err != nil {
		return "", "", "", err
	}

	// Store the refresh token in the database with session info
	refreshTokenRecord := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: HashString(refreshTokenString),
		SessionID: sessionID,
		UserAgent: userAgent,
		IPAddress: ipAddress,
		ExpiresAt: refreshClaims.ExpiresAt.Time,
		IsRevoked: false,
	}

	if err := config.DB.Create(&refreshTokenRecord).Error; err != nil {
		return "", "", "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	return accessTokenString, refreshTokenString, sessionID, nil
}

func ParseJWTToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.AppConfig.EncryptionKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Verify token version
		var user models.User
		if err := config.DB.First(&user, claims.UserID).Error; err != nil {
			return nil, errors.New("user not found")
		}
		if claims.TokenVersion != user.TokenVersion {
			return nil, errors.New("invalid token version")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func RefreshTokens(refreshToken string) (string, string, error) {
	// First verify the refresh token structure
	claims, err := ParseJWTToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Check if refresh token is expired
	if time.Until(claims.ExpiresAt.Time) <= 0 {
		return "", "", errors.New("refresh token expired")
	}

	// Verify the refresh token exists in database and isn't revoked
	var tokenRecord models.RefreshToken
	if err := config.DB.Where("session_id = ? AND is_revoked = ?", claims.SessionID, false).First(&tokenRecord).Error; err != nil {
		return "", "", errors.New("invalid refresh token")
	}

	// Verify the token hash matches
	if !VerifyHash(tokenRecord.TokenHash, refreshToken) {
		// Possible token theft - revoke all tokens for this user
		config.DB.Model(&models.RefreshToken{}).Where("user_id = ?", claims.UserID).Update("is_revoked", true)
		return "", "", errors.New("invalid refresh token")
	}

	// Get the user
	var user models.User
	if err := config.DB.First(&user, claims.UserID).Error; err != nil {
		return "", "", errors.New("user not found")
	}

	// Generate new tokens (pass empty strings for userAgent and ipAddress since we don't have them here)
	accessToken, newRefreshToken, _, err := GenerateJWTToken(&user, "", "")
	if err != nil {
		return "", "", err
	}

	// Revoke the old refresh token
	config.DB.Model(&tokenRecord).Update("is_revoked", true)

	return accessToken, newRefreshToken, nil
}

// HashString creates a secure hash of a string (for storing refresh tokens)
func HashString(s string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return string(hash)
}

// VerifyHash checks if a string matches a hash
func VerifyHash(hash, s string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(s)) == nil
}
