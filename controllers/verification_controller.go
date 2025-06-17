// controller/verification_controller.go
package controller

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/likexian/whois"
	"gorm.io/gorm"

	"mailnexy/models"
	"mailnexy/utils"
)

type VerificationController struct {
	DB     *gorm.DB
	Logger *log.Logger
}

func NewVerificationController(db *gorm.DB, logger *log.Logger) *VerificationController {
	return &VerificationController{
		DB:     db,
		Logger: logger,
	}
}

// VerifyEmail handles single email verification with enhanced accuracy
func (vc *VerificationController) VerifyEmail(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	email := c.Query("email")

	if email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email address is required",
		})
	}

	// Check if user has enough verification credits
	if user.VerifyCredits < 1 {
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": "Insufficient verification credits",
		})
	}

	// Perform verification with enhanced checks
	result, err := utils.EnhancedVerifyEmailAddress(email)
	if err != nil {
		vc.Logger.Printf("Verification failed for %s: %v", email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Verification failed",
		})
	}

	// Deduct credit
	user.VerifyCredits -= 1
	if err := vc.DB.Save(user).Error; err != nil {
		vc.Logger.Printf("Failed to update user credits: %v", err)
	}

	// Add WHOIS data to the result
	whoisInfo, err := whois.Whois(utils.ExtractDomain(email))
	if err == nil {
		result.WHOIS = whoisInfo
	}

	return c.JSON(result)
}

// BulkVerify handles batch email verification with improved concurrency
func (vc *VerificationController) BulkVerify(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var request struct {
		Emails []string `json:"emails"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Check credit balance
	if user.VerifyCredits < len(request.Emails) {
		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": "Insufficient verification credits",
		})
	}

	// Create verification job
	verification := models.EmailVerification{
		UserID: user.ID,
		Name:   "Bulk verification " + time.Now().Format("2006-01-02"),
		Status: "processing",
	}

	if err := vc.DB.Create(&verification).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create verification job",
		})
	}

	// Process in background with improved concurrency
	go vc.enhancedProcessBulkVerification(verification.ID, request.Emails, user.ID)

	return c.JSON(fiber.Map{
		"message":         "Verification started",
		"verification_id": verification.ID,
	})
}

func (vc *VerificationController) enhancedProcessBulkVerification(verificationID uint, emails []string, userID uint) {
	var (
		valid, invalid, disposable, catchAll, unknown int
		results                                       []models.VerificationResult
		mu                                            sync.Mutex
		wg                                            sync.WaitGroup
	)

	// Create a worker pool
	workerCount := 10 // Adjust based on your needs
	emailChan := make(chan string, len(emails))
	resultChan := make(chan *utils.VerificationResult, len(emails))

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for email := range emailChan {
				result, err := utils.EnhancedVerifyEmailAddress(email)
				if err != nil {
					vc.Logger.Printf("Verification failed for %s: %v", email, err)
					continue
				}
				resultChan <- result
			}
		}()
	}

	// Feed emails to workers
	go func() {
		for _, email := range emails {
			emailChan <- email
		}
		close(emailChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for result := range resultChan {
		mu.Lock()
		switch result.Status {
		case "valid":
			valid++
		case "invalid":
			invalid++
		case "disposable":
			disposable++
		case "catch-all":
			catchAll++
		default:
			unknown++
		}

		results = append(results, models.VerificationResult{
			VerificationID: verificationID,
			Email:          result.Email,
			Status:         result.Status,
			Details:        result.Details,
			IsReachable:    result.IsReachable,
			IsBounceRisk:   result.IsBounceRisk,
		})
		mu.Unlock()
	}

	completedAt := time.Now()
	// Update verification job
	verification := models.EmailVerification{
		Model:           gorm.Model{ID: verificationID},
		Status:          "completed",
		ValidCount:      valid,
		InvalidCount:    invalid,
		DisposableCount: disposable,
		CatchAllCount:   catchAll,
		UnknownCount:    unknown,
		CompletedAt:     &completedAt,
	}

	// Use transaction for atomic updates
	err := vc.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.EmailVerification{}).Where("id = ?", verificationID).Updates(&verification).Error; err != nil {
			return err
		}

		if err := tx.CreateInBatches(results, 100).Error; err != nil {
			return err
		}

		// Deduct credits
		return tx.Model(&models.User{}).Where("id = ?", userID).
			Update("verify_credits", gorm.Expr("verify_credits - ?", len(emails))).
			Error
	})

	if err != nil {
		vc.Logger.Printf("Failed to complete verification job: %v", err)
	}
}

// GetVerificationResults retrieves verification job results
func (vc *VerificationController) GetVerificationResults(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	verificationID := c.Params("id")

	var verification models.EmailVerification
	if err := vc.DB.Preload("VerificationResults").Where("id = ? AND user_id = ?", verificationID, user.ID).First(&verification).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification not found",
		})
	}

	return c.JSON(verification)
}

// // controller/verification_controller.go
// package controller

// import (
// 	"log"
// 	"time"

// 	"github.com/gofiber/fiber/v2"
// 	"gorm.io/gorm"

// 	"mailnexy/models"
// 	"mailnexy/utils"
// )

// type VerificationController struct {
// 	DB     *gorm.DB
// 	Logger *log.Logger
// }

// func NewVerificationController(db *gorm.DB, logger *log.Logger) *VerificationController {
// 	return &VerificationController{
// 		DB:     db,
// 		Logger: logger,
// 	}
// }

// // VerifyEmail handles single email verification
// func (vc *VerificationController) VerifyEmail(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	email := c.Query("email")

// 	if email == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Email address is required",
// 		})
// 	}

// 	// Check if user has enough verification credits
// 	if user.VerifyCredits < 1 {
// 		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
// 			"error": "Insufficient verification credits",
// 		})
// 	}

// 	// Perform verification
// 	result, err := utils.VerifyEmailAddress(email)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Verification failed",
// 		})
// 	}

// 	// Deduct credit
// 	user.VerifyCredits -= 1
// 	if err := vc.DB.Save(user).Error; err != nil {
// 		vc.Logger.Printf("Failed to update user credits: %v", err)
// 	}

// 	return c.JSON(result)
// }

// // BulkVerify handles batch email verification
// func (vc *VerificationController) BulkVerify(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var request struct {
// 		Emails []string `json:"emails"`
// 	}

// 	if err := c.BodyParser(&request); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request format",
// 		})
// 	}

// 	// Check credit balance
// 	if user.VerifyCredits < len(request.Emails) {
// 		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
// 			"error": "Insufficient verification credits",
// 		})
// 	}

// 	// Create verification job
// 	verification := models.EmailVerification{
// 		UserID: user.ID,
// 		Name:   "Bulk verification " + time.Now().Format("2006-01-02"),
// 		Status: "processing",
// 	}

// 	if err := vc.DB.Create(&verification).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create verification job",
// 		})
// 	}

// 	// Process in background
// 	go vc.processBulkVerification(verification.ID, request.Emails, user.ID)

// 	return c.JSON(fiber.Map{
// 		"message":         "Verification started",
// 		"verification_id": verification.ID,
// 	})
// }

// func (vc *VerificationController) processBulkVerification(verificationID uint, emails []string, userID uint) {
// 	var valid, invalid, disposable, catchAll, unknown int
// 	var results []models.VerificationResult

// 	for _, email := range emails {
// 		result, err := utils.VerifyEmailAddress(email)
// 		if err != nil {
// 			vc.Logger.Printf("Verification failed for %s: %v", email, err)
// 			continue
// 		}

// 		// Update counters
// 		switch result.Status {
// 		case "valid":
// 			valid++
// 		case "invalid":
// 			invalid++
// 		case "disposable":
// 			disposable++
// 		case "catch-all":
// 			catchAll++
// 		default:
// 			unknown++
// 		}

// 		results = append(results, models.VerificationResult{
// 			VerificationID: verificationID,
// 			Email:          email,
// 			Status:         result.Status,
// 			Details:        result.Details,
// 			IsReachable:    result.IsReachable,
// 			IsBounceRisk:   result.IsBounceRisk,
// 		})
// 	}
// 	completedAt := time.Now()
// 	// Update verification job
// 	verification := models.EmailVerification{
// 		Model:           gorm.Model{ID: verificationID},
// 		Status:          "completed",
// 		ValidCount:      valid,
// 		InvalidCount:    invalid,
// 		DisposableCount: disposable,
// 		CatchAllCount:   catchAll,
// 		UnknownCount:    unknown,
// 		CompletedAt:     &completedAt,
// 	}

// 	// Use transaction for atomic updates
// 	err := vc.DB.Transaction(func(tx *gorm.DB) error {
// 		if err := tx.Model(&models.EmailVerification{}).Where("id = ?", verificationID).Updates(&verification).Error; err != nil {
// 			return err
// 		}

// 		if err := tx.CreateInBatches(results, 100).Error; err != nil {
// 			return err
// 		}

// 		// Deduct credits
// 		return tx.Model(&models.User{}).Where("id = ?", userID).
// 			Update("verify_credits", gorm.Expr("verify_credits - ?", len(emails))).
// 			Error
// 	})

// 	if err != nil {
// 		vc.Logger.Printf("Failed to complete verification job: %v", err)
// 	}
// }

// // GetVerificationResults retrieves verification job results
// func (vc *VerificationController) GetVerificationResults(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	verificationID := c.Params("id")

// 	var verification models.EmailVerification
// 	if err := vc.DB.Preload("VerificationResults").Where("id = ? AND user_id = ?", verificationID, user.ID).First(&verification).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Verification not found",
// 		})
// 	}

// 	return c.JSON(verification)
// }
