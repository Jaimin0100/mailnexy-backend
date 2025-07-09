package controller

import (
	"log"
	"time"
	
	"github.com/gofiber/websocket/v2"
)

// campaign_controller.go
func HandleCampaignProgressWS(c *websocket.Conn) {
	defer c.Close()

	var input struct {
		CampaignName string `json:"campaignName"`
		Action       string `json:"action"`
	}

	// Read JSON message
	if err := c.ReadJSON(&input); err != nil {
		log.Printf("Error reading JSON: %v", err)
		return
	}

	// Simulate campaign progress
	if input.Action == "simulate" {
		stages := []string{
			"Sending initial emails...",
			"Waiting for responses...",
			"Sending follow-ups...",
			"Tracking opens and clicks...",
			"Processing replies...",
			"Campaign completed!",
		}

		for i, stage := range stages {
			time.Sleep(2 * time.Second)
			progress := struct {
				Message string `json:"message"`
				Percent int    `json:"percent"`
				Status  string `json:"status"`
			}{
				Message: stage,
				Percent: (i + 1) * 100 / len(stages),
				Status:  "running",
			}

			if i == len(stages)-1 {
				progress.Status = "completed"
			}

			// Write JSON message
			if err := c.WriteJSON(progress); err != nil {
				log.Printf("Error writing JSON: %v", err)
				break
			}
		}
	}
}
