package utils

import (
	"context"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/webhook"
	"mailnexy/config"
)

// ConstructStripeEvent securely constructs and verifies a Stripe webhook event
func ConstructStripeEvent(c *fiber.Ctx) (stripe.Event, error) {
	// Read the raw request body
	payload, err := io.ReadAll(c.Request().BodyStream())
	if err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to read webhook payload", "error", err)
		return stripe.Event{}, fiber.NewError(fiber.StatusBadRequest, "Failed to read request body")
	}

	// Get and validate the Stripe-Signature header
	signature := c.Get("Stripe-Signature")
	if signature == "" {
		config.DB.Logger.Error(c.Context(), "Missing Stripe-Signature header")
		return stripe.Event{}, fiber.NewError(fiber.StatusBadRequest, "Missing Stripe-Signature header")
	}

	// Verify the webhook signature with tolerance for clock drift
	event, err := webhook.ConstructEventWithTolerance(
		payload,
		signature,
		config.AppConfig.StripeWebhookSecret,
		5*time.Minute, // Recommended tolerance for clock drift
	)
	if err != nil {
		config.DB.Logger.Error(c.Context(), "Failed to verify webhook signature",
			"error", err,
			"signature_prefix", signature[:10]+"...", // Log partial signature for debugging
		)
		return stripe.Event{}, fiber.NewError(fiber.StatusBadRequest, "Invalid webhook signature")
	}

	// Log successful event verification
	config.DB.Logger.Info(c.Context(), "Stripe webhook event verified",
		"event_id", event.ID,
		"event_type", event.Type,
	)

	return event, nil
}

// GetStripePrice retrieves a price from Stripe with proper error handling
func GetStripePrice(priceID string) (*stripe.Price, error) {
	// Validate price ID
	if priceID == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "Price ID is required")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve the price from Stripe
	p, err := price.Get(priceID, &stripe.PriceParams{
		Params: stripe.Params{
			Context: ctx,
		},
	})
	if err != nil {
		config.DB.Logger.Error(ctx, "Failed to get Stripe price",
			"price_id", priceID,
			"error", err,
		)
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve price information")
	}

	// Additional validation - check if price is active
	if !p.Active {
		config.DB.Logger.Warn(ctx, "Inactive Stripe price retrieved",
			"price_id", priceID,
		)
	}

	return p, nil
}

// GetPriceAmount retrieves the amount in cents for a given price ID
func GetPriceAmount(priceID string) (int64, error) {
	price, err := GetStripePrice(priceID)
	if err != nil {
		return 0, err
	}
	return price.UnitAmount, nil
}

// IsPriceActive checks if a price is currently active
func IsPriceActive(priceID string) (bool, error) {
	price, err := GetStripePrice(priceID)
	if err != nil {
		return false, err
	}
	return price.Active, nil
}