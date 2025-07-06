package controller

import (
	"encoding/csv"
	"log"
	"strconv"
	"strings"
	"time"

	"mailnexy/models"
	"mailnexy/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type LeadController struct {
	DB     *gorm.DB
	Logger *log.Logger
}

func NewLeadController(db *gorm.DB, logger *log.Logger) *LeadController {
	return &LeadController{
		DB:     db,
		Logger: logger,
	}
}

// CreateLead creates a new lead with validation
func (lc *LeadController) CreateLead(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var input struct {
		Email        string            `json:"email" validate:"required,email"`
		FirstName    string            `json:"first_name" validate:"omitempty,max=100"`
		LastName     string            `json:"last_name" validate:"omitempty,max=100"`
		Phone        string            `json:"phone" validate:"omitempty,e164"`
		Company      string            `json:"company" validate:"omitempty,max=200"`
		CustomFields map[string]string `json:"custom_fields"`
		ListIDs      []uint            `json:"list_ids"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	if len(input.ListIDs) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "At least one lead list ID is required", nil)
	}

	// Check if lead already exists
	var existingLead models.Lead
	if err := lc.DB.Where("email = ? AND user_id = ?", input.Email, user.ID).First(&existingLead).Error; err == nil {
		return utils.ErrorResponse(c, fiber.StatusConflict, "Lead with this email already exists", nil)
	}

	lead := models.Lead{
		UserID:       user.ID,
		LeadListID:   input.ListIDs[0], // Set primary list
		Email:        strings.ToLower(input.Email),
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Phone:        input.Phone,
		Company:      input.Company,
		CustomFields: convertCustomFields(input.CustomFields),
	}

	if err := lc.DB.Create(&lead).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create lead", err)
	}

	// Create lead list memberships
	for _, listID := range input.ListIDs {
		var list models.LeadList
		if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}

		membership := models.LeadListMembership{
			LeadListID: listID,
			LeadID:     lead.ID,
		}
		if err := lc.DB.Create(&membership).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to add lead to list", err)
		}
	}

	return c.Status(fiber.StatusCreated).JSON(utils.SuccessResponse(lead))
}

// Helper function to convert custom fields
func convertCustomFields(fields map[string]string) []models.LeadCustomField {
	var result []models.LeadCustomField
	for name, value := range fields {
		result = append(result, models.LeadCustomField{
			Name:  name,
			Value: value,
		})
	}
	return result
}

// GetLeads returns paginated list of leads with filters
func (lc *LeadController) GetLeads(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	// Get list_id from query params
	listID := c.Query("list_id")

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Filters
	email := c.Query("email")
	company := c.Query("company")
	status := c.Query("status")

	query := lc.DB.Where("user_id = ?", user.ID)

	if listID != "" {
		// Convert to uint
		listIDUint, err := strconv.ParseUint(listID, 10, 32)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid list ID", err)
		}

		// Use JOIN to filter by list membership
		query = query.Joins("JOIN lead_list_memberships ON lead_list_memberships.lead_id = leads.id").
			Where("lead_list_memberships.lead_list_id = ?", uint(listIDUint))
	}

	if email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}
	if company != "" {
		query = query.Where("company LIKE ?", "%"+company+"%")
	}
	if status != "" {
		switch status {
		case "active":
			query = query.Where("is_unsubscribed = false AND is_b bounced = false AND is_do_not_contact = false")
		case "unsubscribed":
			query = query.Where("is_unsubscribed = true")
		case "bounced":
			query = query.Where("is_bounced = true")
		case "do_not_contact":
			query = query.Where("is_do_not_contact = true")
		}
	}

	var leads []models.Lead
	if err := query.Offset(offset).Limit(limit).Find(&leads).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
	}

	var total int64
	query.Model(&models.Lead{}).Count(&total)

	return c.JSON(utils.PaginatedResponse{
		Data:  leads,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetLead returns a single lead by ID
func (lc *LeadController) GetLead(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	leadID := c.Params("id")

	var lead models.Lead
	if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead", err)
	}

	return c.JSON(utils.SuccessResponse(lead))
}

// UpdateLead updates lead details
func (lc *LeadController) UpdateLead(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	leadID := c.Params("id")

	var input struct {
		Email        string            `json:"email" validate:"omitempty,email"`
		FirstName    string            `json:"first_name" validate:"omitempty,max=100"`
		LastName     string            `json:"last_name" validate:"omitempty,max=100"`
		Phone        string            `json:"phone" validate:"omitempty,e164"`
		Company      string            `json:"company" validate:"omitempty,max=200"`
		CustomFields map[string]string `json:"custom_fields"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	var lead models.Lead
	if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead", err)
	}

	// Check if email is being updated to an existing one
	if input.Email != "" && input.Email != lead.Email {
		var existingLead models.Lead
		if err := lc.DB.Where("email = ? AND user_id = ?", input.Email, user.ID).First(&existingLead).Error; err == nil {
			return utils.ErrorResponse(c, fiber.StatusConflict, "Lead with this email already exists", nil)
		}
		lead.Email = strings.ToLower(input.Email)
	}

	// Update fields
	if input.FirstName != "" {
		lead.FirstName = input.FirstName
	}
	if input.LastName != "" {
		lead.LastName = input.LastName
	}
	if input.Phone != "" {
		lead.Phone = input.Phone
	}
	if input.Company != "" {
		lead.Company = input.Company
	}
	if input.CustomFields != nil {
		lead.CustomFields = convertCustomFields(input.CustomFields)
	}

	if err := lc.DB.Save(&lead).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update lead", err)
	}

	return c.JSON(utils.SuccessResponse(lead))
}

// DeleteLead deletes a lead
func (lc *LeadController) DeleteLead(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	leadID := c.Params("id")

	tx := lc.DB.Begin()

	// Delete memberships
	if err := tx.Where("lead_id = ?", leadID).Delete(&models.LeadListMembership{}).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete lead memberships", err)
	}

	// Delete lead
	result := tx.Where("id = ? AND user_id = ?", leadID, user.ID).Delete(&models.Lead{})
	if result.Error != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete lead", result.Error)
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
	}

	tx.Commit()

	return c.JSON(utils.SuccessResponse(fiber.Map{
		"message": "Lead deleted successfully",
	}))
}

// ImportLeads imports leads from CSV file
func (lc *LeadController) ImportLeads(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	leadListIDStr := c.Query("list_id")

	if leadListIDStr == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Lead list ID is required for import", nil)
	}

	leadListID, err := strconv.ParseUint(leadListIDStr, 10, 32)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid lead list ID", err)
	}

	// Verify list exists and belongs to user
	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", leadListID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "File upload error", err)
	}

	// Check file size (max 5MB)
	if file.Size > 5<<20 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "File too large (max 5MB)", nil)
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to open file", err)
	}
	defer src.Close()

	// Parse CSV
	reader := csv.NewReader(src)
	records, err := reader.ReadAll()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Failed to parse CSV file", err)
	}

	if len(records) < 2 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "CSV file must have at least a header and one row", nil)
	}

	header := records[0]
	rows := records[1:]

	// Process leads in batches
	batchSize := 100
	var leads []models.Lead
	var leadListMembers []models.LeadListMembership

	for _, row := range rows {
		if len(row) != len(header) {
			continue // Skip malformed rows
		}

		leadData := make(map[string]string)
		for i, col := range header {
			leadData[col] = row[i]
		}

		email, ok := leadData["email"]
		if !ok || email == "" {
			continue // Skip rows without email
		}

		// Check if lead already exists
		var existingLead models.Lead
		err := lc.DB.Where("email = ? AND user_id = ?", email, user.ID).First(&existingLead).Error

		if err == gorm.ErrRecordNotFound {
			// Create new lead
			lead := models.Lead{
				UserID:       user.ID,
				LeadListID:   uint(leadListID), // Set primary list
				Email:        strings.ToLower(email),
				FirstName:    leadData["first_name"],
				LastName:     leadData["last_name"],
				Phone:        leadData["phone"],
				Company:      leadData["company"],
				CustomFields: convertCustomFields(leadData),
			}
			leads = append(leads, lead)
		} else if err == nil {
			// Add existing lead to list
			leadListMembers = append(leadListMembers, models.LeadListMembership{
				LeadListID: uint(leadListID),
				LeadID:     existingLead.ID,
			})
		}

		// Process batch when size is reached
		if len(leads) >= batchSize {
			if err := lc.DB.Create(&leads).Error; err != nil {
				lc.Logger.Printf("Failed to import batch of leads: %v", err)
			}
			// Add new leads to list
			for _, lead := range leads {
				leadListMembers = append(leadListMembers, models.LeadListMembership{
					LeadListID: uint(leadListID),
					LeadID:     lead.ID,
				})
			}
			leads = nil
		}
	}

	// Process remaining leads
	if len(leads) > 0 {
		if err := lc.DB.Create(&leads).Error; err != nil {
			lc.Logger.Printf("Failed to import final batch of leads: %v", err)
		}
		// Add new leads to list
		for _, lead := range leads {
			leadListMembers = append(leadListMembers, models.LeadListMembership{
				LeadListID: uint(leadListID),
				LeadID:     lead.ID,
			})
		}
	}

	// Add leads to list
	if len(leadListMembers) > 0 {
		if err := lc.DB.Create(&leadListMembers).Error; err != nil {
			lc.Logger.Printf("Failed to add leads to list: %v", err)
		}
	}

	return c.JSON(utils.SuccessResponse(fiber.Map{
		"message":       "Leads imported successfully",
		"total_rows":    len(rows),
		"imported":      len(leads) + len(leadListMembers),
		"new_leads":     len(leads),
		"added_to_list": len(leadListMembers),
	}))
}

// ExportLeads exports leads to CSV
func (lc *LeadController) ExportLeads(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var leads []models.Lead
	if err := lc.DB.Where("user_id = ?", user.ID).Find(&leads).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=leads_export_"+time.Now().Format("20060102")+".csv")

	writer := csv.NewWriter(c)
	defer writer.Flush()

	// Write header
	header := []string{"email", "first_name", "last_name", "phone", "company"}
	if err := writer.Write(header); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate CSV", err)
	}

	// Write data
	for _, lead := range leads {
		record := []string{
			lead.Email,
			lead.FirstName,
			lead.LastName,
			lead.Phone,
			lead.Company,
		}
		if err := writer.Write(record); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate CSV", err)
		}
	}

	return nil
}

// CreateLeadList creates a new lead list
func (lc *LeadController) CreateLeadList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var input struct {
		Name        string `json:"name" validate:"required,max=100"`
		Description string `json:"description" validate:"max=500"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	// Check if list with same name exists
	var existingList models.LeadList
	if err := lc.DB.Where("name = ? AND user_id = ?", input.Name, user.ID).First(&existingList).Error; err == nil {
		return utils.ErrorResponse(c, fiber.StatusConflict, "List with this name already exists", nil)
	}

	leadList := models.LeadList{
		UserID:      user.ID,
		Name:        input.Name,
		Description: input.Description,
	}

	if err := lc.DB.Create(&leadList).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create lead list", err)
	}

	return c.Status(fiber.StatusCreated).JSON(utils.SuccessResponse(leadList))
}

// GetLeadLists returns all lead lists for the user
func (lc *LeadController) GetLeadLists(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)

	var lists []models.LeadList
	if err := lc.DB.Where("user_id = ?", user.ID).Find(&lists).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead lists", err)
	}

	return c.JSON(utils.SuccessResponse(lists))
}

// GetLeadList returns a single lead list with member count
func (lc *LeadController) GetLeadList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	// Get member count
	var count int64
	lc.DB.Model(&models.LeadListMembership{}).Where("lead_list_id = ?", listID).Count(&count)

	return c.JSON(utils.SuccessResponse(fiber.Map{
		"list":        list,
		"memberCount": count,
	}))
}

// UpdateLeadList updates lead list details
func (lc *LeadController) UpdateLeadList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	var input struct {
		Name        string `json:"name" validate:"required,max=100"`
		Description string `json:"description" validate:"max=500"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	// Check if name is being updated to an existing one
	if input.Name != list.Name {
		var existingList models.LeadList
		if err := lc.DB.Where("name = ? AND user_id = ?", input.Name, user.ID).First(&existingList).Error; err == nil {
			return utils.ErrorResponse(c, fiber.StatusConflict, "List with this name already exists", nil)
		}
		list.Name = input.Name
	}

	list.Description = input.Description

	if err := lc.DB.Save(&list).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update lead list", err)
	}

	return c.JSON(utils.SuccessResponse(list))
}

// DeleteLeadList deletes a lead list
func (lc *LeadController) DeleteLeadList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	// Start transaction
	tx := lc.DB.Begin()

	// First delete memberships
	if err := tx.Where("lead_list_id = ?", listID).Delete(&models.LeadListMembership{}).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete list memberships", err)
	}

	// Then delete the list
	result := tx.Where("id = ? AND user_id = ?", listID, user.ID).Delete(&models.LeadList{})
	if result.Error != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete lead list", result.Error)
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
	}

	tx.Commit()

	return c.JSON(utils.SuccessResponse(fiber.Map{
		"message": "Lead list deleted successfully",
	}))
}

// AddLeadsToList adds multiple leads to a list
func (lc *LeadController) AddLeadsToList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	var input struct {
		LeadIDs []uint `json:"lead_ids" validate:"required,min=1"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	// Verify list exists and belongs to user
	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	// Process leads in batches
	batchSize := 100
	var memberships []models.LeadListMembership
	var leadsNotFound []uint

	for _, leadID := range input.LeadIDs {
		// Verify lead exists and belongs to user
		var lead models.Lead
		if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				leadsNotFound = append(leadsNotFound, leadID)
				continue
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to verify lead", err)
		}

		// Check if membership already exists
		var existingMembership models.LeadListMembership
		err := lc.DB.Where("lead_list_id = ? AND lead_id = ?", listID, leadID).First(&existingMembership).Error

		if err == gorm.ErrRecordNotFound {
			memberships = append(memberships, models.LeadListMembership{
				LeadListID: list.ID,
				LeadID:     lead.ID,
			})
		}

		// Process batch when size is reached
		if len(memberships) >= batchSize {
			if err := lc.DB.Create(&memberships).Error; err != nil {
				lc.Logger.Printf("Failed to add batch of leads to list: %v", err)
			}
			memberships = nil
		}
	}

	// Process remaining memberships
	if len(memberships) > 0 {
		if err := lc.DB.Create(&memberships).Error; err != nil {
			lc.Logger.Printf("Failed to add final batch of leads to list: %v", err)
		}
	}

	response := fiber.Map{
		"message":         "Leads added to list successfully",
		"added":           len(memberships),
		"already_in_list": len(input.LeadIDs) - len(memberships) - len(leadsNotFound),
	}

	if len(leadsNotFound) > 0 {
		response["leads_not_found"] = leadsNotFound
	}

	return c.JSON(utils.SuccessResponse(response))
}

// RemoveLeadsFromList removes multiple leads from a list
func (lc *LeadController) RemoveLeadsFromList(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	var input struct {
		LeadIDs []uint `json:"lead_ids" validate:"required,min=1"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
	}

	// Validate input
	if err := utils.ValidateStruct(input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
	}

	// Verify list exists and belongs to user
	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	// Delete memberships
	result := lc.DB.Where("lead_list_id = ? AND lead_id IN ?", listID, input.LeadIDs).Delete(&models.LeadListMembership{})
	if result.Error != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to remove leads from list", result.Error)
	}

	return c.JSON(utils.SuccessResponse(fiber.Map{
		"message": "Leads removed from list successfully",
		"removed": result.RowsAffected,
	}))
}

// GetLeadListMembers returns all leads in a list with pagination
func (lc *LeadController) GetLeadListMembers(c *fiber.Ctx) error {
	user := c.Locals("user").(*models.User)
	listID := c.Params("id")

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Verify list exists and belongs to user
	var list models.LeadList
	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
	}

	var leads []models.Lead
	query := lc.DB.
		Joins("JOIN lead_list_memberships ON lead_list_memberships.lead_id = leads.id").
		Where("lead_list_memberships.lead_list_id = ?", listID).
		Where("leads.user_id = ?", user.ID)

	// Get total count
	var total int64
	if err := query.Model(&models.Lead{}).Count(&total).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to count leads", err)
	}

	// Get paginated results
	if err := query.Offset(offset).Limit(limit).Find(&leads).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
	}

	return c.JSON(utils.PaginatedResponse{
		Data:  leads,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// package controller

// import (
// 	"encoding/csv"
// 	"log"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"mailnexy/models"
// 	"mailnexy/utils"

// 	"github.com/gofiber/fiber/v2"
// 	"gorm.io/gorm"
// )

// type LeadController struct {
// 	DB     *gorm.DB
// 	Logger *log.Logger
// }

// func NewLeadController(db *gorm.DB, logger *log.Logger) *LeadController {
// 	return &LeadController{
// 		DB:     db,
// 		Logger: logger,
// 	}
// }

// // CreateLead creates a new lead with validation
// func (lc *LeadController) CreateLead(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var input struct {
// 		Email        string            `json:"email" validate:"required,email"`
// 		FirstName    string            `json:"first_name" validate:"omitempty,max=100"`
// 		LastName     string            `json:"last_name" validate:"omitempty,max=100"`
// 		Phone        string            `json:"phone" validate:"omitempty,e164"`
// 		Company      string            `json:"company" validate:"omitempty,max=200"`
// 		CustomFields map[string]string `json:"custom_fields"`
// 		ListIDs      []uint            `json:"list_ids"` // NEW: Add list IDs
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	// Check if lead already exists
// 	var existingLead models.Lead
// 	if err := lc.DB.Where("email = ? AND user_id = ?", input.Email, user.ID).First(&existingLead).Error; err == nil {
// 		return utils.ErrorResponse(c, fiber.StatusConflict, "Lead with this email already exists", nil)
// 	}

// 	lead := models.Lead{
// 		UserID:       user.ID,
// 		Email:        strings.ToLower(input.Email),
// 		FirstName:    input.FirstName,
// 		LastName:     input.LastName,
// 		Phone:        input.Phone,
// 		Company:      input.Company,
// 		CustomFields: convertCustomFields(input.CustomFields),
// 	}

// 	if err := lc.DB.Create(&lead).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create lead", err)
// 	}

// 	// NEW: Create lead list memberships
// 	if len(input.ListIDs) > 0 {
// 		for _, listID := range input.ListIDs {
// 			// Verify list exists and belongs to user
// 			var list models.LeadList
// 			if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 				return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 			}

// 			// Create membership
// 			membership := models.LeadListMembership{
// 				LeadListID: listID,
// 				LeadID:     lead.ID,
// 			}
// 			if err := lc.DB.Create(&membership).Error; err != nil {
// 				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to add lead to list", err)
// 			}
// 		}
// 	}

// 	return c.Status(fiber.StatusCreated).JSON(utils.SuccessResponse(lead))
// }

// // Add this helper function
// func convertCustomFields(fields map[string]string) []models.LeadCustomField {
// 	var result []models.LeadCustomField
// 	for name, value := range fields {
// 		result = append(result, models.LeadCustomField{
// 			Name:  name,
// 			Value: value,
// 		})
// 	}
// 	return result
// }

// // GetLeads returns paginated list of leads with filters
// func (lc *LeadController) GetLeads(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	// Pagination
// 	page, _ := strconv.Atoi(c.Query("page", "1"))
// 	limit, _ := strconv.Atoi(c.Query("limit", "20"))
// 	if limit > 100 {
// 		limit = 100
// 	}
// 	offset := (page - 1) * limit

// 	// Filters
// 	email := c.Query("email")
// 	company := c.Query("company")
// 	status := c.Query("status")

// 	query := lc.DB.Where("user_id = ?", user.ID)

// 	if email != "" {
// 		query = query.Where("email LIKE ?", "%"+email+"%")
// 	}
// 	if company != "" {
// 		query = query.Where("company LIKE ?", "%"+company+"%")
// 	}
// 	if status != "" {
// 		switch status {
// 		case "active":
// 			query = query.Where("is_unsubscribed = false AND is_bounced = false AND is_do_not_contact = false")
// 		case "unsubscribed":
// 			query = query.Where("is_unsubscribed = true")
// 		case "bounced":
// 			query = query.Where("is_bounced = true")
// 		case "do_not_contact":
// 			query = query.Where("is_do_not_contact = true")
// 		}
// 	}

// 	var leads []models.Lead
// 	if err := query.Offset(offset).Limit(limit).Find(&leads).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
// 	}

// 	var total int64
// 	query.Model(&models.Lead{}).Count(&total)

// 	return c.JSON(utils.PaginatedResponse{
// 		Data:  leads,
// 		Total: total,
// 		Page:  page,
// 		Limit: limit,
// 	})
// }

// // GetLead returns a single lead by ID
// func (lc *LeadController) GetLead(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	leadID := c.Params("id")

// 	var lead models.Lead
// 	if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead", err)
// 	}

// 	return c.JSON(utils.SuccessResponse(lead))
// }

// // UpdateLead updates lead details
// func (lc *LeadController) UpdateLead(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	leadID := c.Params("id")

// 	var input struct {
// 		Email        string            `json:"email" validate:"omitempty,email"`
// 		FirstName    string            `json:"first_name" validate:"omitempty,max=100"`
// 		LastName     string            `json:"last_name" validate:"omitempty,max=100"`
// 		Phone        string            `json:"phone" validate:"omitempty,e164"`
// 		Company      string            `json:"company" validate:"omitempty,max=200"`
// 		CustomFields map[string]string `json:"custom_fields"`
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	var lead models.Lead
// 	if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead", err)
// 	}

// 	// Check if email is being updated to an existing one
// 	if input.Email != "" && input.Email != lead.Email {
// 		var existingLead models.Lead
// 		if err := lc.DB.Where("email = ? AND user_id = ?", input.Email, user.ID).First(&existingLead).Error; err == nil {
// 			return utils.ErrorResponse(c, fiber.StatusConflict, "Lead with this email already exists", nil)
// 		}
// 		lead.Email = strings.ToLower(input.Email)
// 	}

// 	// Update fields
// 	if input.FirstName != "" {
// 		lead.FirstName = input.FirstName
// 	}
// 	if input.LastName != "" {
// 		lead.LastName = input.LastName
// 	}
// 	if input.Phone != "" {
// 		lead.Phone = input.Phone
// 	}
// 	if input.Company != "" {
// 		lead.Company = input.Company
// 	}
// 	if input.CustomFields != nil {
// 		lead.CustomFields = convertCustomFields(input.CustomFields)
// 	}

// 	if err := lc.DB.Save(&lead).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update lead", err)
// 	}

// 	return c.JSON(utils.SuccessResponse(lead))
// }

// // DeleteLead deletes a lead
// func (lc *LeadController) DeleteLead(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	leadID := c.Params("id")

// 	result := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).Delete(&models.Lead{})
// 	if result.Error != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete lead", result.Error)
// 	}

// 	if result.RowsAffected == 0 {
// 		return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead not found", nil)
// 	}

// 	return c.JSON(utils.SuccessResponse(fiber.Map{
// 		"message": "Lead deleted successfully",
// 	}))
// }

// // ImportLeads imports leads from CSV file
// func (lc *LeadController) ImportLeads(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	leadListID := c.Query("list_id")

// 	file, err := c.FormFile("file")
// 	if err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "File upload error", err)
// 	}

// 	// Check file size (max 5MB)
// 	if file.Size > 5<<20 {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "File too large (max 5MB)", nil)
// 	}

// 	// Open the file
// 	src, err := file.Open()
// 	if err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to open file", err)
// 	}
// 	defer src.Close()

// 	// Parse CSV
// 	reader := csv.NewReader(src)
// 	records, err := reader.ReadAll()
// 	if err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Failed to parse CSV file", err)
// 	}

// 	if len(records) < 2 {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "CSV file must have at least a header and one row", nil)
// 	}

// 	header := records[0]
// 	rows := records[1:]

// 	// Process leads in batches
// 	batchSize := 100
// 	var leads []models.Lead
// 	var leadListMembers []models.LeadListMembership

// 	for _, row := range rows {
// 		if len(row) != len(header) {
// 			continue // Skip malformed rows
// 		}

// 		leadData := make(map[string]string)
// 		for i, col := range header {
// 			leadData[col] = row[i]
// 		}

// 		email, ok := leadData["email"]
// 		if !ok || email == "" {
// 			continue // Skip rows without email
// 		}

// 		// Check if lead already exists
// 		var existingLead models.Lead
// 		err := lc.DB.Where("email = ? AND user_id = ?", email, user.ID).First(&existingLead).Error

// 		if err == gorm.ErrRecordNotFound {
// 			// Create new lead
// 			lead := models.Lead{
// 				UserID:       user.ID,
// 				Email:        strings.ToLower(email),
// 				FirstName:    leadData["first_name"],
// 				LastName:     leadData["last_name"],
// 				Phone:        leadData["phone"],
// 				Company:      leadData["company"],
// 				CustomFields: convertCustomFields(leadData),
// 			}

// 			leads = append(leads, lead)
// 		} else if err == nil && leadListID != "" {
// 			// Add existing lead to list
// 			leadListMembers = append(leadListMembers, models.LeadListMembership{
// 				LeadListID: utils.ParseUint(leadListID),
// 				LeadID:     existingLead.ID,
// 			})
// 		}

// 		// Process batch when size is reached
// 		if len(leads) >= batchSize {
// 			if err := lc.DB.Create(&leads).Error; err != nil {
// 				lc.Logger.Printf("Failed to import batch of leads: %v", err)
// 			}
// 			leads = nil
// 		}
// 	}

// 	// Process remaining leads
// 	if len(leads) > 0 {
// 		if err := lc.DB.Create(&leads).Error; err != nil {
// 			lc.Logger.Printf("Failed to import final batch of leads: %v", err)
// 		}
// 	}

// 	// Add leads to list if specified
// 	if leadListID != "" && len(leadListMembers) > 0 {
// 		if err := lc.DB.Create(&leadListMembers).Error; err != nil {
// 			lc.Logger.Printf("Failed to add leads to list: %v", err)
// 		}
// 	}

// 	return c.JSON(utils.SuccessResponse(fiber.Map{
// 		"message":       "Leads imported successfully",
// 		"total_rows":    len(rows),
// 		"imported":      len(leads) + len(leadListMembers),
// 		"new_leads":     len(leads),
// 		"added_to_list": len(leadListMembers),
// 	}))
// }

// // ExportLeads exports leads to CSV
// func (lc *LeadController) ExportLeads(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var leads []models.Lead
// 	if err := lc.DB.Where("user_id = ?", user.ID).Find(&leads).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
// 	}

// 	c.Set("Content-Type", "text/csv")
// 	c.Set("Content-Disposition", "attachment; filename=leads_export_"+time.Now().Format("20060102")+".csv")

// 	writer := csv.NewWriter(c)
// 	defer writer.Flush()

// 	// Write header
// 	header := []string{"email", "first_name", "last_name", "phone", "company"}
// 	if err := writer.Write(header); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate CSV", err)
// 	}

// 	// Write data
// 	for _, lead := range leads {
// 		record := []string{
// 			lead.Email,
// 			lead.FirstName,
// 			lead.LastName,
// 			lead.Phone,
// 			lead.Company,
// 		}
// 		if err := writer.Write(record); err != nil {
// 			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate CSV", err)
// 		}
// 	}

// 	return nil
// }

// // CreateLeadList creates a new lead list
// func (lc *LeadController) CreateLeadList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var input struct {
// 		Name        string `json:"name" validate:"required,max=100"`
// 		Description string `json:"description" validate:"max=500"`
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	// Check if list with same name exists
// 	var existingList models.LeadList
// 	if err := lc.DB.Where("name = ? AND user_id = ?", input.Name, user.ID).First(&existingList).Error; err == nil {
// 		return utils.ErrorResponse(c, fiber.StatusConflict, "List with this name already exists", nil)
// 	}

// 	leadList := models.LeadList{
// 		UserID:      user.ID,
// 		Name:        input.Name,
// 		Description: input.Description,
// 	}

// 	if err := lc.DB.Create(&leadList).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create lead list", err)
// 	}

// 	return c.Status(fiber.StatusCreated).JSON(utils.SuccessResponse(leadList))
// }

// // GetLeadLists returns all lead lists for the user
// func (lc *LeadController) GetLeadLists(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)

// 	var lists []models.LeadList
// 	if err := lc.DB.Where("user_id = ?", user.ID).Find(&lists).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead lists", err)
// 	}

// 	return c.JSON(utils.SuccessResponse(lists))
// }

// // GetLeadList returns a single lead list with member count
// func (lc *LeadController) GetLeadList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	var list models.LeadList
// 	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
// 	}

// 	// Get member count
// 	var count int64
// 	lc.DB.Model(&models.LeadListMembership{}).Where("lead_list_id = ?", listID).Count(&count)

// 	return c.JSON(utils.SuccessResponse(fiber.Map{
// 		"list":        list,
// 		"memberCount": count,
// 	}))
// }

// // UpdateLeadList updates lead list details
// func (lc *LeadController) UpdateLeadList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	var input struct {
// 		Name        string `json:"name" validate:"required,max=100"`
// 		Description string `json:"description" validate:"max=500"`
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	var list models.LeadList
// 	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
// 	}

// 	// Check if name is being updated to an existing one
// 	if input.Name != list.Name {
// 		var existingList models.LeadList
// 		if err := lc.DB.Where("name = ? AND user_id = ?", input.Name, user.ID).First(&existingList).Error; err == nil {
// 			return utils.ErrorResponse(c, fiber.StatusConflict, "List with this name already exists", nil)
// 		}
// 		list.Name = input.Name
// 	}

// 	list.Description = input.Description

// 	if err := lc.DB.Save(&list).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update lead list", err)
// 	}

// 	return c.JSON(utils.SuccessResponse(list))
// }

// // DeleteLeadList deletes a lead list
// func (lc *LeadController) DeleteLeadList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	// Start transaction
// 	tx := lc.DB.Begin()

// 	// First delete memberships
// 	if err := tx.Where("lead_list_id = ?", listID).Delete(&models.LeadListMembership{}).Error; err != nil {
// 		tx.Rollback()
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete list memberships", err)
// 	}

// 	// Then delete the list
// 	result := tx.Where("id = ? AND user_id = ?", listID, user.ID).Delete(&models.LeadList{})
// 	if result.Error != nil {
// 		tx.Rollback()
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete lead list", result.Error)
// 	}

// 	if result.RowsAffected == 0 {
// 		tx.Rollback()
// 		return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 	}

// 	tx.Commit()

// 	return c.JSON(utils.SuccessResponse(fiber.Map{
// 		"message": "Lead list deleted successfully",
// 	}))
// }

// // AddLeadsToList adds multiple leads to a list
// func (lc *LeadController) AddLeadsToList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	var input struct {
// 		LeadIDs []uint `json:"lead_ids" validate:"required,min=1"`
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	// Verify list exists and belongs to user
// 	var list models.LeadList
// 	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
// 	}

// 	// Process leads in batches
// 	batchSize := 100
// 	var memberships []models.LeadListMembership
// 	var leadsNotFound []uint

// 	for _, leadID := range input.LeadIDs {
// 		// Verify lead exists and belongs to user
// 		var lead models.Lead
// 		if err := lc.DB.Where("id = ? AND user_id = ?", leadID, user.ID).First(&lead).Error; err != nil {
// 			if err == gorm.ErrRecordNotFound {
// 				leadsNotFound = append(leadsNotFound, leadID)
// 				continue
// 			}
// 			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to verify lead", err)
// 		}

// 		// Check if membership already exists
// 		var existingMembership models.LeadListMembership
// 		err := lc.DB.Where("lead_list_id = ? AND lead_id = ?", listID, leadID).First(&existingMembership).Error

// 		if err == gorm.ErrRecordNotFound {
// 			memberships = append(memberships, models.LeadListMembership{
// 				LeadListID: list.ID,
// 				LeadID:     lead.ID,
// 			})
// 		}

// 		// Process batch when size is reached
// 		if len(memberships) >= batchSize {
// 			if err := lc.DB.Create(&memberships).Error; err != nil {
// 				lc.Logger.Printf("Failed to add batch of leads to list: %v", err)
// 			}
// 			memberships = nil
// 		}
// 	}

// 	// Process remaining memberships
// 	if len(memberships) > 0 {
// 		if err := lc.DB.Create(&memberships).Error; err != nil {
// 			lc.Logger.Printf("Failed to add final batch of leads to list: %v", err)
// 		}
// 	}

// 	response := fiber.Map{
// 		"message":         "Leads added to list successfully",
// 		"added":           len(memberships),
// 		"already_in_list": len(input.LeadIDs) - len(memberships) - len(leadsNotFound),
// 	}

// 	if len(leadsNotFound) > 0 {
// 		response["leads_not_found"] = leadsNotFound
// 	}

// 	return c.JSON(utils.SuccessResponse(response))
// }

// // RemoveLeadsFromList removes multiple leads from a list
// func (lc *LeadController) RemoveLeadsFromList(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	var input struct {
// 		LeadIDs []uint `json:"lead_ids" validate:"required,min=1"`
// 	}

// 	if err := c.BodyParser(&input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body", err)
// 	}

// 	// Validate input
// 	if err := utils.ValidateStruct(input); err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Validation failed", err)
// 	}

// 	// Verify list exists and belongs to user
// 	var list models.LeadList
// 	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
// 	}

// 	// Delete memberships
// 	result := lc.DB.Where("lead_list_id = ? AND lead_id IN ?", listID, input.LeadIDs).Delete(&models.LeadListMembership{})
// 	if result.Error != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to remove leads from list", result.Error)
// 	}

// 	return c.JSON(utils.SuccessResponse(fiber.Map{
// 		"message": "Leads removed from list successfully",
// 		"removed": result.RowsAffected,
// 	}))
// }

// // GetLeadListMembers returns all leads in a list with pagination
// func (lc *LeadController) GetLeadListMembers(c *fiber.Ctx) error {
// 	user := c.Locals("user").(*models.User)
// 	listID := c.Params("id")

// 	// Pagination
// 	page, _ := strconv.Atoi(c.Query("page", "1"))
// 	limit, _ := strconv.Atoi(c.Query("limit", "20"))
// 	if limit > 100 {
// 		limit = 100
// 	}
// 	offset := (page - 1) * limit

// 	// Verify list exists and belongs to user
// 	var list models.LeadList
// 	if err := lc.DB.Where("id = ? AND user_id = ?", listID, user.ID).First(&list).Error; err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return utils.ErrorResponse(c, fiber.StatusNotFound, "Lead list not found", nil)
// 		}
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch lead list", err)
// 	}

// 	var leads []models.Lead
// 	query := lc.DB.
// 		Joins("JOIN lead_list_memberships ON lead_list_memberships.lead_id = leads.id").
// 		Where("lead_list_memberships.lead_list_id = ?", listID).
// 		Where("leads.user_id = ?", user.ID)

// 	// Get total count
// 	var total int64
// 	if err := query.Model(&models.Lead{}).Count(&total).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to count leads", err)
// 	}

// 	// Get paginated results
// 	if err := query.Offset(offset).Limit(limit).Find(&leads).Error; err != nil {
// 		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch leads", err)
// 	}

// 	return c.JSON(utils.PaginatedResponse{
// 		Data:  leads,
// 		Total: total,
// 		Page:  page,
// 		Limit: limit,
// 	})
// }
