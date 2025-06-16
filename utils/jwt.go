package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"mailnexy/config"
	"mailnexy/models"
)

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateJWTToken(user *models.User) (string, string, error) {
	// Access token (15 minutes expiry)
	accessClaims := &Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(config.AppConfig.EncryptionKey))
	if err != nil {
		return "", "", err
	}

	// Refresh token (7 days expiry)
	refreshClaims := &Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(config.AppConfig.EncryptionKey))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
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
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func RefreshTokens(refreshToken string) (string, string, error) {
	claims, err := ParseJWTToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Check if refresh token is expired
	if time.Until(claims.ExpiresAt.Time) <= 0 {
		return "", "", errors.New("refresh token expired")
	}

	// Generate new tokens
	var user models.User
	if err := config.DB.First(&user, claims.UserID).Error; err != nil {
		return "", "", errors.New("user not found")
	}

	return GenerateJWTToken(&user)
}