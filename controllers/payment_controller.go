package controller

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/charge"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"mailnexy/config"
	"mailnexy/models"
	"mailnexy/utils"
)

func InitStripe() {
	stripe.Key = config.AppConfig.StripeSecretKey
}

type PaymentRequest struct {
	PlanID uint `json:"plan_id" validate:"required"`
	UserID uint `json:"user_id" validate:"required"`
}

// CreatePaymentIntent creates a Stripe Payment Intent
func CreatePaymentIntent(c *fiber.Ctx) error {
	var req PaymentRequest

	if err := c.BodyParser(&req); err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to parse request body", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if req.PlanID == 0 || req.UserID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Plan ID and User ID are required",
		})
	}

	// Get the plan from database
	var plan models.Plan
	if err := config.DB.First(&plan, req.PlanID).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Plan not found", "plan_id", req.PlanID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Plan not found",
		})
	}

	// Get the user
	var user models.User
	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "User not found", "user_id", req.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Create or get Stripe customer
	customerID, err := getOrCreateStripeCustomer(&user)
	if err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to create Stripe customer", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process payment",
		})
	}

	// Create Payment Intent
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(plan.EmailPrice)),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Customer: stripe.String(customerID),
		Metadata: map[string]string{
			"user_id": strconv.Itoa(int(user.ID)),
			"plan_id": strconv.Itoa(int(plan.ID)),
		},
		Description: stripe.String("Purchase of " + plan.Name + " plan"),
	}

	if plan.BillingInterval != "one_time" {
		params.SetupFutureUsage = stripe.String("off_session")
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to create payment intent", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process payment",
		})
	}

	// Create transaction record
	transaction := models.CreditTransaction{
		UserID:                user.ID,
		PlanID:                &plan.ID,
		EmailCredits:          plan.EmailCredits,
		VerifyCredits:         plan.VerifyCredits,
		Amount:                plan.EmailPrice,
		Currency:              "usd",
		PaymentStatus:         "requires_payment_method",
		StripePaymentIntentID: pi.ID,
		Description:           "Purchase of " + plan.Name + " plan",
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to create transaction", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process transaction",
		})
	}

	return c.JSON(fiber.Map{
		"clientSecret":   pi.ClientSecret,
		"transaction_id": transaction.ID,
		"amount":         plan.EmailPrice,
		"currency":       "usd",
	})
}

// HandlePaymentWebhook handles Stripe webhook events
func HandlePaymentWebhook(c *fiber.Ctx) error {
	event, err := utils.ConstructStripeEvent(c)
	if err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to construct Stripe event", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook payload",
		})
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			config.DB.Logger.Error(c.Context(), "Failed to parse payment intent", "error", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Error parsing payment intent",
			})
		}
		return handlePaymentIntentSucceeded(c, &paymentIntent)

	case "payment_intent.payment_failed":
		var paymentIntent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			config.DB.Logger.Error(c.Context(), "Failed to parse payment intent", "error", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Error parsing payment intent",
			})
		}
		return handlePaymentIntentFailed(c, &paymentIntent)

	case "charge.succeeded":
		var charge stripe.Charge
		if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
			config.DB.Logger.Error(c.Context(), "Failed to parse charge", "error", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Error parsing charge",
			})
		}
		return handleChargeSucceeded(c, &charge)

	default:
		return c.SendStatus(fiber.StatusOK)
	}
}

// handlePaymentIntentSucceeded processes successful payments
func handlePaymentIntentSucceeded(c *fiber.Ctx, pi *stripe.PaymentIntent) error {
	var transaction models.CreditTransaction
	if err := config.DB.Where("stripe_payment_intent_id = ?", pi.ID).First(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Transaction not found", "payment_intent_id", pi.ID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Transaction not found",
		})
	}

	transaction.PaymentStatus = "succeeded"
	transaction.PaymentMethod = string(pi.PaymentMethod.Type)

	if pi.LatestCharge != nil {
		ch, err := charge.Get(pi.LatestCharge.ID, nil)
		if err == nil {
			transaction.StripeChargeID = ch.ID
			transaction.ReceiptURL = ch.ReceiptURL
		}
	}

	if err := config.DB.Save(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to update transaction", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update transaction",
		})
	}

	var user models.User
	if err := config.DB.First(&user, transaction.UserID).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "User not found", "user_id", transaction.UserID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	user.EmailCredits += transaction.EmailCredits
	user.VerifyCredits += transaction.VerifyCredits

	if err := config.DB.Save(&user).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to update user credits", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user credits",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// handleChargeSucceeded handles charge.succeeded events
func handleChargeSucceeded(c *fiber.Ctx, ch *stripe.Charge) error {
	if ch.PaymentIntent == nil {
		return c.SendStatus(fiber.StatusOK)
	}

	var transaction models.CreditTransaction
	if err := config.DB.Where("stripe_payment_intent_id = ?", ch.PaymentIntent.ID).First(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Transaction not found", "payment_intent_id", ch.PaymentIntent.ID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Transaction not found",
		})
	}

	transaction.StripeChargeID = ch.ID
	transaction.ReceiptURL = ch.ReceiptURL
	transaction.PaymentStatus = "succeeded"

	if err := config.DB.Save(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to update transaction", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update transaction",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// handlePaymentIntentFailed processes failed payments
func handlePaymentIntentFailed(c *fiber.Ctx, pi *stripe.PaymentIntent) error {
	var transaction models.CreditTransaction
	if err := config.DB.Where("stripe_payment_intent_id = ?", pi.ID).First(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Transaction not found", "payment_intent_id", pi.ID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Transaction not found",
		})
	}

	transaction.PaymentStatus = "failed"
	if pi.LastPaymentError != nil {
		// Use the correct field from the Stripe SDK v76
		errorMessage := "Payment failed"
		if pi.LastPaymentError.Msg != "" {
			errorMessage = "Payment failed: " + pi.LastPaymentError.Msg
		}
		transaction.Description = errorMessage
	}

	if err := config.DB.Save(&transaction).Error; err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to update transaction", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update transaction",
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// getOrCreateStripeCustomer gets or creates a Stripe customer
func getOrCreateStripeCustomer(user *models.User) (string, error) {
	if user.StripeCustomerID != nil {
		return *user.StripeCustomerID, nil
	}

	var name string
	if user.Name != nil {
		name = *user.Name
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(user.Email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"user_id": strconv.Itoa(int(user.ID)),
		},
	}

	c, err := customer.New(params)
	if err != nil {
		return "", err
	}

	user.StripeCustomerID = &c.ID
	if err := config.DB.Save(user).Error; err != nil {
		return "", err
	}

	return c.ID, nil
}