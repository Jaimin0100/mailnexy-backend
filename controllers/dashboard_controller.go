package controller

import (
	"log"
	"time"

	"mailnexy/models"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DashboardController struct {
	DB     *gorm.DB
	Logger *log.Logger
}

func NewDashboardController(db *gorm.DB, logger *log.Logger) *DashboardController {
	return &DashboardController{
		DB:     db,
		Logger: logger,
	}
}

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type DashboardStats struct {
	TotalEmailSent int64   `json:"total_email_sent"`
	OpenRate       float64 `json:"open_rate"`
	ClickRate      float64 `json:"click_rate"`
	ReplyRate      float64 `json:"reply_rate"`
	BounceRate     float64 `json:"bounce_rate"`
}

type TimeSeriesData struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

type Dataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BorderColor     string    `json:"borderColor"`
	BackgroundColor string    `json:"backgroundColor"`
}

type CampaignSummary struct {
	Name      string  `json:"name"`
	Sent      int     `json:"sent"`
	OpenRate  float64 `json:"open_rate"`
	ClickRate float64 `json:"click_rate"`
}

// GetDashboardStats returns summary statistics for the dashboard cards
// func (dc *DashboardController) GetDashboardStats(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	timeFrame := c.Query("time_frame", "week") // hour, day, week, month

// 	// Calculate time range based on timeframe
// 	now := time.Now()
// 	var startTime time.Time

// 	switch timeFrame {
// 	case "hour":
// 		startTime = now.Add(-1 * time.Hour)
// 	case "day":
// 		startTime = now.Add(-24 * time.Hour)
// 	case "week":
// 		startTime = now.Add(-7 * 24 * time.Hour)
// 	case "month":
// 		startTime = now.Add(-30 * 24 * time.Hour)
// 	default:
// 		startTime = now.Add(-7 * 24 * time.Hour)
// 	}

// 	// Get stats from database
// 	var stats DashboardStats

// 	// Total emails sent
// 	if err := dc.DB.Model(&models.CampaignActivity{}).
// 		Where("user_id = ? AND sent_at BETWEEN ? AND ?", user.ID, startTime, now).
// 		Count(&stats.TotalEmailSent).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get email stats", err)
// 	}

// 	// Open rate
// 	var openCount int64
// 	dc.DB.Model(&models.CampaignActivity{}).
// 		Where("user_id = ? AND opened_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, startTime, now).
// 		Count(&openCount)

// 	if stats.TotalEmailSent > 0 {
// 		stats.OpenRate = float64(openCount) / float64(stats.TotalEmailSent) * 100
// 	}

// 	// Click rate
// 	var clickCount int64
// 	dc.DB.Model(&models.CampaignActivity{}).
// 		Where("user_id = ? AND clicked_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, startTime, now).
// 		Count(&clickCount)

// 	if stats.TotalEmailSent > 0 {
// 		stats.ClickRate = float64(clickCount) / float64(stats.TotalEmailSent) * 100
// 	}

// 	// Reply rate
// 	var replyCount int64
// 	dc.DB.Model(&models.CampaignActivity{}).
// 		Where("user_id = ? AND replied_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, startTime, now).
// 		Count(&replyCount)

// 	if stats.TotalEmailSent > 0 {
// 		stats.ReplyRate = float64(replyCount) / float64(stats.TotalEmailSent) * 100
// 	}

// 	// Bounce rate
// 	var bounceCount int64
// 	dc.DB.Model(&models.CampaignActivity{}).
// 		Where("user_id = ? AND bounced_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, startTime, now).
// 		Count(&bounceCount)

// 	if stats.TotalEmailSent > 0 {
// 		stats.BounceRate = float64(bounceCount) / float64(stats.TotalEmailSent) * 100
// 	}

// 	return c.JSON(utils.SuccessResponse(stats))
// }

func (dc *DashboardController) GetDashboardStats(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	timeFrame := c.Query("time_frame", "week") // hour, day, week, month

	// Calculate time range based on timeframe
	now := time.Now()
	var startTime time.Time

	switch timeFrame {
	case "hour":
		startTime = now.Add(-1 * time.Hour)
	case "day":
		startTime = now.Add(-24 * time.Hour)
	case "week":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "month":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-7 * 24 * time.Hour)
	}

	// Get stats from database
	var stats DashboardStats

	// Total emails sent
	if err := dc.DB.Model(&models.CampaignActivity{}).
		Joins("JOIN campaigns ON campaigns.id = campaign_activities.campaign_id").
		Where("campaigns.user_id = ? AND campaign_activities.sent_at BETWEEN ? AND ?", user.ID, startTime, now).
		Count(&stats.TotalEmailSent).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get email stats", err)
	}

	// Open rate
	var openCount int64
	dc.DB.Model(&models.CampaignActivity{}).
		Joins("JOIN campaigns ON campaigns.id = campaign_activities.campaign_id").
		Where("campaigns.user_id = ? AND campaign_activities.opened_at IS NOT NULL AND campaign_activities.sent_at BETWEEN ? AND ?", user.ID, startTime, now).
		Count(&openCount)
	if stats.TotalEmailSent > 0 {
		stats.OpenRate = float64(openCount) / float64(stats.TotalEmailSent) * 100
	}

	// Click rate
	var clickCount int64
	dc.DB.Model(&models.CampaignActivity{}).
		Joins("JOIN campaigns ON campaigns.id = campaign_activities.campaign_id").
		Where("campaigns.user_id = ? AND campaign_activities.clicked_at IS NOT NULL AND campaign_activities.sent_at BETWEEN ? AND ?", user.ID, startTime, now).
		Count(&clickCount)
	if stats.TotalEmailSent > 0 {
		stats.ClickRate = float64(clickCount) / float64(stats.TotalEmailSent) * 100
	}

	// Reply rate
	var replyCount int64
	dc.DB.Model(&models.CampaignActivity{}).
		Joins("JOIN campaigns ON campaigns.id = campaign_activities.campaign_id").
		Where("campaigns.user_id = ? AND campaign_activities.replied_at IS NOT NULL AND campaign_activities.sent_at BETWEEN ? AND ?", user.ID, startTime, now).
		Count(&replyCount)
	if stats.TotalEmailSent > 0 {
		stats.ReplyRate = float64(replyCount) / float64(stats.TotalEmailSent) * 100
	}

	// Bounce rate
	var bounceCount int64
	dc.DB.Model(&models.CampaignActivity{}).
		Joins("JOIN campaigns ON campaigns.id = campaign_activities.campaign_id").
		Where("campaigns.user_id = ? AND campaign_activities.bounced_at IS NOT NULL AND campaign_activities.sent_at BETWEEN ? AND ?", user.ID, startTime, now).
		Count(&bounceCount)
	if stats.TotalEmailSent > 0 {
		stats.BounceRate = float64(bounceCount) / float64(stats.TotalEmailSent) * 100
	}

	return c.JSON(utils.SuccessResponse(stats))
}

// GetEmailMetricsOverTime returns time series data for the line chart
func (dc *DashboardController) GetEmailMetricsOverTime(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	timeRange := c.Query("range", "year") // month, year

	now := time.Now()
	var startTime time.Time
	var labels []string
	var interval string

	if timeRange == "month" {
		startTime = now.Add(-30 * 24 * time.Hour)
		labels = []string{"Week 1", "Week 2", "Week 3", "Week 4"}
		interval = "week"
	} else {
		startTime = now.Add(-365 * 24 * time.Hour)
		labels = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		interval = "month"
	}

	data := TimeSeriesData{
		Labels: labels,
		Datasets: []Dataset{
			{
				Label:           "Total Email Sent",
				BorderColor:     "#10B981",
				BackgroundColor: "rgba(16, 185, 129, 0.1)",
			},
			{
				Label:           "Open Rate",
				BorderColor:     "#3B82F6",
				BackgroundColor: "rgba(59, 130, 246, 0.1)",
			},
			{
				Label:           "Click Rate",
				BorderColor:     "#EF4444",
				BackgroundColor: "rgba(239, 68, 68, 0.1)",
			},
			{
				Label:           "Reply Rate",
				BorderColor:     "#8B5CF6",
				BackgroundColor: "rgba(139, 92, 246, 0.1)",
			},
		},
	}

	// Populate data for each interval
	for i := range labels {
		var start, end time.Time
		if interval == "week" {
			start = startTime.Add(time.Duration(i) * 7 * 24 * time.Hour)
			end = start.Add(7 * 24 * time.Hour)
		} else {
			start = time.Date(now.Year(), time.Month(i+1), 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 1, 0)
		}

		// Get counts for each metric in this interval
		var sentCount int64
		var openCount int64
		var clickCount int64
		var replyCount int64

		dc.DB.Model(&models.CampaignActivity{}).
			Where("user_id = ? AND sent_at BETWEEN ? AND ?", user.ID, start, end).
			Count(&sentCount)

		dc.DB.Model(&models.CampaignActivity{}).
			Where("user_id = ? AND opened_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, start, end).
			Count(&openCount)

		dc.DB.Model(&models.CampaignActivity{}).
			Where("user_id = ? AND clicked_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, start, end).
			Count(&clickCount)

		dc.DB.Model(&models.CampaignActivity{}).
			Where("user_id = ? AND replied_at IS NOT NULL AND sent_at BETWEEN ? AND ?", user.ID, start, end).
			Count(&replyCount)

		// Add to datasets
		data.Datasets[0].Data = append(data.Datasets[0].Data, float64(sentCount))
		data.Datasets[1].Data = append(data.Datasets[1].Data, float64(openCount))
		data.Datasets[2].Data = append(data.Datasets[2].Data, float64(clickCount))
		data.Datasets[3].Data = append(data.Datasets[3].Data, float64(replyCount))
	}

	return c.JSON(utils.SuccessResponse(data))
}

// GetEmailStatusBreakdown returns data for the donut chart
func (dc *DashboardController) GetEmailStatusBreakdown(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	timeRange := c.Query("range", "week") // week, month

	now := time.Now()
	var startTime time.Time

	if timeRange == "month" {
		startTime = now.Add(-30 * 24 * time.Hour)
	} else {
		startTime = now.Add(-7 * 24 * time.Hour)
	}

	data := struct {
		Labels   []string `json:"labels"`
		Datasets []struct {
			Data            []int64  `json:"data"`
			BackgroundColor []string `json:"backgroundColor"`
		} `json:"datasets"`
	}{
		Labels: []string{"Delivered", "Bounced", "Unsubscribed"},
		Datasets: []struct {
			Data            []int64  `json:"data"`
			BackgroundColor []string `json:"backgroundColor"`
		}{
			{
				Data:            make([]int64, 3),
				BackgroundColor: []string{"#3B82F6", "#EF4444", "#D1D5DB"},
			},
		},
	}

	// Get delivered count (sent but not bounced or unsubscribed)
	if err := dc.DB.Model(&models.CampaignActivity{}).
		Where("user_id = ? AND sent_at BETWEEN ? AND ? AND bounced_at IS NULL AND unsubscribed_at IS NULL",
			user.ID, startTime, now).
		Count(&data.Datasets[0].Data[0]).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get delivered count", err)
	}

	// Get bounced count
	if err := dc.DB.Model(&models.CampaignActivity{}).
		Where("user_id = ? AND bounced_at IS NOT NULL AND sent_at BETWEEN ? AND ?",
			user.ID, startTime, now).
		Count(&data.Datasets[0].Data[1]).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get bounced count", err)
	}

	// Get unsubscribed count
	if err := dc.DB.Model(&models.CampaignActivity{}).
		Where("user_id = ? AND unsubscribed_at IS NOT NULL AND sent_at BETWEEN ? AND ?",
			user.ID, startTime, now).
		Count(&data.Datasets[0].Data[2]).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get unsubscribed count", err)
	}

	return c.JSON(utils.SuccessResponse(data))
}

// GetRecentCampaigns returns data for the recent campaigns table
func (dc *DashboardController) GetRecentCampaigns(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	limit := c.QueryInt("limit", 3)

	var campaigns []models.Campaign
	if err := dc.DB.Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(limit).
		Find(&campaigns).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to get campaigns", err)
	}

	var summaries []CampaignSummary
	for _, campaign := range campaigns {
		var sentCount, openCount, clickCount int64

		dc.DB.Model(&models.CampaignActivity{}).
			Where("campaign_id = ?", campaign.ID).
			Count(&sentCount)

		dc.DB.Model(&models.CampaignActivity{}).
			Where("campaign_id = ? AND opened_at IS NOT NULL", campaign.ID).
			Count(&openCount)

		dc.DB.Model(&models.CampaignActivity{}).
			Where("campaign_id = ? AND clicked_at IS NOT NULL", campaign.ID).
			Count(&clickCount)

		var openRate, clickRate float64
		if sentCount > 0 {
			openRate = float64(openCount) / float64(sentCount) * 100
			clickRate = float64(clickCount) / float64(sentCount) * 100
		}

		summaries = append(summaries, CampaignSummary{
			Name:      campaign.Name,
			Sent:      int(sentCount),
			OpenRate:  openRate,
			ClickRate: clickRate,
		})
	}

	return c.JSON(utils.SuccessResponse(summaries))
}
