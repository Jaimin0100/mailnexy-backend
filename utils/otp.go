// // package utils

// // import (
// //     "math/rand"
// //     "time"
// // "fmt"
// // )

// // func GenerateOTP() string {
// //     rand.Seed(time.Now().UnixNano())
// //     return fmt.Sprintf("%06d", rand.Intn(1000000))
// // }

// package utils

// import (
// 	"fmt"
// 	"math/rand"
// 	"time"
// )

// func GenerateOTP() string {
// 	rand.Seed(time.Now().UnixNano())
// 	return fmt.Sprintf("%06d", rand.Intn(1000000))
// }

// // Add this new function to calculate expiration time
//
//	func GetOTPExpiryTime() time.Time {
//		return time.Now().Add(20 * time.Minute) // 20 minutes from now
//	}

package utils

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"time"

	"mailnexy/config"
	"mailnexy/models"
)

const (
	OTPLength         = 6
	OTPExpiry         = 15 * time.Minute
	MaxOTPAttempts    = 3
	OTPResendCooldown = 1 * time.Minute
)

func GenerateOTP() (string, error) {
	const digits = "0123456789"
	otp := make([]byte, OTPLength)

	for i := range otp {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		otp[i] = digits[num.Int64()]
	}

	return string(otp), nil
}

func GenerateSecureToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func SaveOTP(userID uint, otp string) error {
	expiresAt := time.Now().Add(OTPExpiry)

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return err
	}

	user.OTP = otp
	user.OTPExpiresAt = expiresAt
	user.OTPVerified = false

	return config.DB.Save(&user).Error
}

func VerifyOTP(userID uint, otp string) (bool, error) {
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return false, err
	}

	// Check if OTP matches and is not expired
	if user.OTP == otp && time.Now().Before(user.OTPExpiresAt) {
		user.OTP = ""
		user.OTPVerified = true
		if err := config.DB.Save(&user).Error; err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func CanResendOTP(userID uint) (bool, time.Duration, error) {
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return false, 0, err
	}

	if user.OTPExpiresAt.IsZero() {
		return true, 0, nil
	}

	remaining := time.Until(user.OTPExpiresAt.Add(-OTPResendCooldown))
	if remaining <= 0 {
		return true, 0, nil
	}

	return false, remaining, nil
}