package controller

import (
	"log"

	"mailnexy/utils"

	"gorm.io/gorm"
)

type CampaignController struct {
	DB          *gorm.DB
	Logger      *log.Logger
	MailService utils.MailServiceInterface
}

func NewCampaignController(db *gorm.DB, logger *log.Logger) *CampaignController {
	return &CampaignController{
		DB:     db,
		Logger: logger,
	}
}
